package visualizer

import (
	"strings"
	"testing"
	"time"

	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func testTreeGraph() *graph.ResourceGraph {
	root := &k8s.K8sResource{
		CoreFields: k8s.CoreFields{
			Name:      "deployment",
			Namespace: "default",
			Status:    "Running",
			Age:       5 * time.Minute,
			Raw: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"uid": "root-uid",
				},
			}},
		},
		Kind: "Deployment",
	}
	child := &k8s.K8sResource{
		CoreFields: k8s.CoreFields{Name: "pod", Namespace: "default", Status: "Running"},
		Kind:       "Pod",
	}

	resourceGraph := graph.NewResourceGraph(root)
	childNode := resourceGraph.AddNode(child, graph.RelationshipOwner)
	resourceGraph.AddEdge(resourceGraph.Root, childNode, graph.EdgeTypeOwns)
	return resourceGraph
}

func TestTreeDetailsHiddenByDefault(t *testing.T) {
	view := NewTreeView(testTreeGraph(), 100, 30, "V")

	if view.showDetails {
		t.Fatal("tree details should be hidden by default")
	}
	if strings.Contains(view.View(), "Resource Details") {
		t.Fatal("hidden tree details were rendered")
	}
}

func TestTreeDetailsShowResourceAndRelationshipData(t *testing.T) {
	view := NewTreeView(testTreeGraph(), 100, 30, "V")
	view.HandleCommand(keys.TreeCmdToggleDetails)

	if !view.showDetails {
		t.Fatal("tree details did not open")
	}

	rendered := view.View()
	for _, want := range []string{
		"Resource Details",
		"Kind: Deployment",
		"Name: deployment",
		"Status: Running",
		"Age: 5m",
		"API version: apps/v1",
		"UID: root-uid",
		"Role: root resource",
		"Child: default/Pod/pod (owns)",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("details view does not contain %q", want)
		}
	}
}

func TestGraphViewHandlesNilGraph(t *testing.T) {
	view := NewGraphView(nil, 100, 30, "V")
	if got := view.View(); got != "No resource graph available" {
		t.Fatalf("nil graph view = %q", got)
	}
}
