package modes

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func testManifestResource() k8s.TrackedObject {
	raw := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "demo",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"replicas": int64(1),
		},
	}}
	return &k8s.K8sResource{
		CoreFields: k8s.CoreFields{
			Name:      "demo",
			Namespace: "default",
			Raw:       raw,
		},
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	}
}

func TestManifestScreenLoadsAndRendersResourceSnapshot(t *testing.T) {
	screen := NewManifestScreen()
	screen.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	screen.Update(ManifestRequestedMsg{Resource: testManifestResource()})

	view := ansi.Strip(screen.View())
	if !strings.Contains(view, "Manifest: Deployment/demo") {
		t.Fatalf("manifest view = %q, missing title", view)
	}
	if !strings.Contains(view, "replicas") {
		t.Fatalf("manifest view = %q, missing YAML content", view)
	}

	updated := testManifestResource()
	updated.GetRaw().Object["spec"] = map[string]interface{}{"replicas": int64(2)}
	screen.Update(ManifestEditFinishedMsg{Resource: updated})
	if screen.resource != updated {
		t.Fatal("manifest screen retained the stale pre-edit resource")
	}
	if view := ansi.Strip(screen.View()); !strings.Contains(view, "replicas") || !strings.Contains(view, "2") {
		t.Fatalf("updated manifest view = %q, missing authoritative edited content", view)
	}
}

func TestManifestScreenRaisesWorkflowRequestsAndNavigates(t *testing.T) {
	screen := NewManifestScreen()
	screen.Update(ManifestRequestedMsg{Resource: testManifestResource()})

	editCmd := screen.Update(tea.KeyPressMsg{Text: "e"})
	if editCmd == nil {
		t.Fatal("edit returned no request command")
	}
	if msg, ok := editCmd().(ManifestEditRequestedMsg); !ok || msg.Resource == nil {
		t.Fatalf("edit message = %#v, want resource request", editCmd())
	}

	updated := testManifestResource()
	updated.GetRaw().SetResourceVersion("2")
	screen.Update(ManifestEditFinishedMsg{Resource: updated})

	copyCmd := screen.Update(tea.KeyPressMsg{Text: "ctrl+y"})
	if copyCmd == nil {
		t.Fatal("copy returned no request command")
	}
	if msg, ok := copyCmd().(ManifestCopyRequestedMsg); !ok || msg.Resource != updated {
		t.Fatalf("copy message = %#v, want authoritative updated resource", copyCmd())
	}

	backCmd := screen.Update(tea.KeyPressMsg{Text: "esc"})
	if backCmd == nil {
		t.Fatal("back returned no navigation command")
	}
	if _, ok := backCmd().(BackMsg); !ok {
		t.Fatalf("back message = %#v, want BackMsg", backCmd())
	}
}
