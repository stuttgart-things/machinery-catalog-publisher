// Package entity renders a machinery ResourceStatus into a Backstage Resource
// entity. The output is exactly what the Git-side Location points at on S3.
package entity

import (
	"fmt"
	"sort"
	"strings"

	rs "github.com/stuttgart-things/machinery/resourceservice"
	"sigs.k8s.io/yaml"
)

const annPrefix = "sthings.lab/"

// Options carry the non-status inputs to a render. LastUpdated is injected by
// the caller (never time.Now here) so output is deterministic and testable.
type Options struct {
	Owner           string // spec.owner, e.g. "platform-team"
	EntityNamespace string // Backstage namespace, e.g. "default"
	LastUpdated     string // RFC3339
}

// Render produces one Backstage Resource YAML document for a status. The
// entity is named "<name>-status" so it never collides with the Git-owned
// Component (an entity may belong to only one location).
func Render(r *rs.ResourceStatus, opt Options) ([]byte, error) {
	phase := "NotReady"
	if r.GetReady() {
		phase = "Ready"
	}

	ann := map[string]string{
		annPrefix + "phase":        phase,
		annPrefix + "last-updated": opt.LastUpdated,
		annPrefix + "kind":         r.GetKind(),
		annPrefix + "namespace":    r.GetNamespace(),
	}
	if m := r.GetStatusMessage(); m != "" {
		ann[annPrefix+"status-message"] = m
	}
	if c := r.GetConnectionDetails(); c != "" {
		ann[annPrefix+"connection"] = c
	}
	for k, v := range r.GetInfoFields() { // DataCenter, IP, etc. → annotations
		ann[annPrefix+"info."+sanitize(k)] = v
	}

	ent := map[string]any{
		"apiVersion": "backstage.io/v1alpha1",
		"kind":       "Resource",
		"metadata": map[string]any{
			"name":        r.GetName() + "-status",
			"namespace":   opt.EntityNamespace,
			"annotations": ann,
		},
		"spec": map[string]any{
			"type":  r.GetKind() + "-status",
			"owner": opt.Owner,
			// shows up in the Component's Relations tab:
			"dependencyOf": []string{
				fmt.Sprintf("component:%s/%s", opt.EntityNamespace, r.GetName()),
			},
		},
	}
	return yaml.Marshal(ent)
}

// RenderAll concatenates many entities into one multi-document YAML, for the
// Aggregate layout. Input order is preserved; callers sort for determinism.
func RenderAll(resources []*rs.ResourceStatus, opt Options) ([]byte, error) {
	var b strings.Builder
	for i, r := range resources {
		doc, err := Render(r, opt)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			b.WriteString("---\n")
		}
		b.Write(doc)
	}
	return []byte(b.String()), nil
}

// Key is the stable S3 object key for a resource in PerResource layout.
func Key(r *rs.ResourceStatus, prefix string) string {
	return fmt.Sprintf("%s%s/%s.yaml", prefix, r.GetNamespace(), r.GetName())
}

// AggregateKey is the single object key in Aggregate layout.
func AggregateKey(prefix string) string { return prefix + "all.yaml" }

// SortKey orders resources deterministically (namespace, then name).
func SortKey(r *rs.ResourceStatus) string { return r.GetNamespace() + "/" + r.GetName() }

// Sort sorts a slice of resources in place by SortKey.
func Sort(resources []*rs.ResourceStatus) {
	sort.Slice(resources, func(i, j int) bool {
		return SortKey(resources[i]) < SortKey(resources[j])
	})
}

// sanitize keeps annotation keys within the Backstage/Kubernetes charset.
func sanitize(k string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			return r
		default:
			return '_'
		}
	}, k)
}
