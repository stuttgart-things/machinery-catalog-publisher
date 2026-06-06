// Package sink writes rendered entities to S3/MinIO. Cache-Control: no-cache is
// mandatory — Backstage relies on a fresh ETag per overwrite to detect change.
package sink

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/stuttgart-things/machinery-catalog-publisher/internal/config"
)

// S3 is a minimal PUT/DELETE client against one bucket.
type S3 struct {
	cli    *s3.Client
	bucket string
}

// NewS3 builds a path-style client for a custom endpoint (MinIO / self-hosted),
// wiring the connection secret's static creds and TLS trust (CA bundle or the
// ignore-insecure toggle).
func NewS3(ctx context.Context, conn config.S3Conn, bucket string) (*S3, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case conn.InsecureSkipVerify:
		tlsCfg.InsecureSkipVerify = true // pubca ignored on purpose
	case len(conn.CABundle) > 0:
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(conn.CABundle) {
			return nil, fmt.Errorf("S3 CA bundle contains no usable certificates")
		}
		tlsCfg.RootCAs = pool
	}
	httpClient := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(conn.Region),
		awsconfig.WithHTTPClient(httpClient),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(conn.AccessKey, conn.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	cli := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(conn.Endpoint)
		o.UsePathStyle = true // MinIO and most non-AWS S3 want path-style
	})
	return &S3{cli: cli, bucket: bucket}, nil
}

// Put writes/overwrites an object with no-cache headers so Backstage's
// conditional GET picks up every change.
func (s *S3) Put(ctx context.Context, key string, body []byte) error {
	_, err := s.cli.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(body),
		ContentType:  aws.String("application/yaml"),
		CacheControl: aws.String("no-cache, max-age=0"),
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

// Delete removes an object (e.g. on a DELETED resource event).
func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.cli.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}
