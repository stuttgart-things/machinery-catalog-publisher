// Package config loads the publisher's runtime configuration: a non-secret
// YAML file (mounted from a ConfigMap) plus the S3/MinIO connection material
// (sourced from a Secret via environment variables).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

// File is the YAML document mounted at CONFIG_FILE. It is the deployment-mode
// equivalent of a CatalogStatusSync CR spec, minus the secret material.
type File struct {
	Interval string `json:"interval"` // Go duration, e.g. "5m"
	Owner    string `json:"owner"`    // → spec.owner on rendered entities
	Source   Source `json:"source"`
	Sink     Sink   `json:"sink"`
}

// Source is where live status comes from (machinery's gRPC API).
type Source struct {
	MachineryAddr string   `json:"machineryAddr"`
	Kinds         []string `json:"kinds"` // ["*"] or explicit kinds
}

// Sink describes how rendered entities are laid out in the bucket.
type Sink struct {
	Bucket          string `json:"bucket"`
	KeyPrefix       string `json:"keyPrefix"`       // e.g. "status/"
	Layout          string `json:"layout"`          // PerResource | Aggregate
	EntityNamespace string `json:"entityNamespace"` // Backstage namespace, e.g. "default"
}

// S3Conn is the connection secret, read from the environment (envFrom Secret).
// Mirrors the "mc alias set" mental model: alias/endpoint/creds + TLS trust.
type S3Conn struct {
	Alias              string // informational; surfaced in logs
	Endpoint           string
	Region             string
	AccessKey          string
	SecretKey          string
	CABundle           []byte // PEM; optional (custom RootCAs)
	InsecureSkipVerify bool
}

// Config is the fully-resolved, validated runtime configuration.
type Config struct {
	Interval    time.Duration
	Owner       string
	Source      Source
	Sink        Sink
	S3          S3Conn
	MetricsAddr string
}

const (
	defaultConfigPath  = "/etc/publisher/config.yaml"
	defaultMetricsAddr = ":8080"
	defaultRegion      = "us-east-1" // MinIO ignores it but the SDK requires one
	layoutPerResource  = "PerResource"
)

// Load reads the YAML config file and the S3 connection env vars, applies
// defaults, and validates the result.
func Load() (Config, error) {
	path := envDefault("CONFIG_FILE", defaultConfigPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	interval, err := time.ParseDuration(orDefault(f.Interval, "5m"))
	if err != nil {
		return Config{}, fmt.Errorf("interval %q invalid: %w", f.Interval, err)
	}

	cfg := Config{
		Interval:    interval,
		Owner:       orDefault(f.Owner, "platform-team"),
		Source:      f.Source,
		Sink:        f.Sink,
		MetricsAddr: envDefault("METRICS_ADDR", defaultMetricsAddr),
	}
	if len(cfg.Source.Kinds) == 0 {
		cfg.Source.Kinds = []string{"*"}
	}
	cfg.Sink.KeyPrefix = orDefault(cfg.Sink.KeyPrefix, "status/")
	cfg.Sink.Layout = orDefault(cfg.Sink.Layout, layoutPerResource)
	cfg.Sink.EntityNamespace = orDefault(cfg.Sink.EntityNamespace, "default")

	conn, err := loadS3Conn()
	if err != nil {
		return Config{}, err
	}
	cfg.S3 = conn

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadS3Conn() (S3Conn, error) {
	c := S3Conn{
		Alias:              os.Getenv("S3_ALIAS"),
		Endpoint:           os.Getenv("S3_ENDPOINT"),
		Region:             envDefault("S3_REGION", defaultRegion),
		AccessKey:          os.Getenv("S3_ACCESS_KEY"),
		SecretKey:          os.Getenv("S3_SECRET_KEY"),
		InsecureSkipVerify: boolEnv("S3_INSECURE_SKIP_VERIFY"),
	}
	// CA bundle: inline PEM wins; otherwise a mounted file path.
	if pem := os.Getenv("S3_CA_BUNDLE"); pem != "" {
		c.CABundle = []byte(pem)
	} else if p := os.Getenv("S3_CA_BUNDLE_FILE"); p != "" {
		b, err := os.ReadFile(p)
		if err != nil {
			return S3Conn{}, fmt.Errorf("read S3_CA_BUNDLE_FILE %s: %w", p, err)
		}
		c.CABundle = b
	}
	return c, nil
}

func (c Config) validate() error {
	switch {
	case c.Sink.Bucket == "":
		return fmt.Errorf("sink.bucket is required")
	case c.Source.MachineryAddr == "":
		return fmt.Errorf("source.machineryAddr is required")
	case c.S3.Endpoint == "":
		return fmt.Errorf("S3_ENDPOINT is required (connection secret)")
	case c.S3.AccessKey == "" || c.S3.SecretKey == "":
		return fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY are required (connection secret)")
	}
	if c.S3.InsecureSkipVerify && len(c.S3.CABundle) > 0 {
		return fmt.Errorf("set either S3_CA_BUNDLE or S3_INSECURE_SKIP_VERIFY, not both")
	}
	return nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func boolEnv(key string) bool {
	b, _ := strconv.ParseBool(os.Getenv(key))
	return b
}
