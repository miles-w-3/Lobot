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

	screen.Update(ManifestEditFinishedMsg{Content: "updated: true\n"})
	if view := ansi.Strip(screen.View()); !strings.Contains(view, "updated") {
		t.Fatalf("updated manifest view = %q, missing edited content", view)
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

	copyCmd := screen.Update(tea.KeyPressMsg{Text: "ctrl+y"})
	if copyCmd == nil {
		t.Fatal("copy returned no request command")
	}
	if msg, ok := copyCmd().(ManifestCopyRequestedMsg); !ok || msg.Resource == nil {
		t.Fatalf("copy message = %#v, want resource request", copyCmd())
	}

	backCmd := screen.Update(tea.KeyPressMsg{Text: "esc"})
	if backCmd == nil {
		t.Fatal("back returned no navigation command")
	}
	if msg, ok := backCmd().(NavigateMsg); !ok || msg.Target != ScreenHome {
		t.Fatalf("back message = %#v, want Home navigation", backCmd())
	}
}
