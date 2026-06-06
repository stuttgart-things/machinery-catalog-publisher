package entity

import (
	"strings"
	"testing"

	rs "github.com/stuttgart-things/machinery/resourceservice"
)

func sample() *rs.ResourceStatus {
	return &rs.ResourceStatus{
		Name:              "demo-vm",
		Kind:              "HarvesterVM",
		Ready:             true,
		StatusMessage:     "provisioned",
		ConnectionDetails: "ssh demo@10.0.0.5",
		Namespace:         "infra",
		InfoFields:        map[string]string{"DataCenter": "stuttgart", "IP": "10.0.0.5"},
	}
}

func TestRender_DeterministicAndShaped(t *testing.T) {
	opt := Options{Owner: "platform-team", EntityNamespace: "default", LastUpdated: "2026-06-06T07:00:00Z"}

	out, err := Render(sample(), opt)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	got := string(out)

	for _, want := range []string{
		"kind: Resource",
		"name: demo-vm-status",          // -status suffix avoids Component collision
		"namespace: default",            // entity namespace, not the cluster ns
		"sthings.lab/phase: Ready",      // ready → Ready
		"sthings.lab/last-updated: \"2026-06-06T07:00:00Z\"",
		"sthings.lab/info.DataCenter: stuttgart",
		"sthings.lab/namespace: infra",  // cluster ns preserved as annotation
		"type: HarvesterVM-status",
		"- component:default/demo-vm",   // dependencyOf relation
	} {
		if !strings.Contains(got, want) {
			t.Errorf("render output missing %q\n---\n%s", want, got)
		}
	}

	// Determinism: identical inputs → byte-identical output.
	out2, _ := Render(sample(), opt)
	if string(out2) != got {
		t.Error("render is not deterministic for identical input")
	}
}

func TestRender_NotReadyPhase(t *testing.T) {
	r := sample()
	r.Ready = false
	out, _ := Render(r, Options{Owner: "x", EntityNamespace: "default", LastUpdated: "t"})
	if !strings.Contains(string(out), "sthings.lab/phase: NotReady") {
		t.Errorf("expected NotReady phase, got:\n%s", out)
	}
}

func TestKey(t *testing.T) {
	if got := Key(sample(), "status/"); got != "status/infra/demo-vm.yaml" {
		t.Errorf("Key = %q", got)
	}
}
