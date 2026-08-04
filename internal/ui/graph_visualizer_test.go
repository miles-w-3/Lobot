//go:build legacyui

package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/keys"
)

func graphTestResource(kind, name, status string) *k8s.K8sResource {
	return &k8s.K8sResource{
		CoreFields: k8s.CoreFields{Name: name, Namespace: "test", Status: status},
		Kind:       kind,
	}
}

func TestGraphVisualizerUsesFullWidthWithoutDetails(t *testing.T) {
	resourceGraph := graph.NewResourceGraph(graphTestResource("HelmRelease", "sample", "deployed"))
	service := resourceGraph.AddNode(graphTestResource("Service", "sample", "Active"), graph.RelationshipHelm)
	deployment := resourceGraph.AddNode(graphTestResource("Deployment", "sample", "Available"), graph.RelationshipHelm)
	resourceGraph.AddEdge(resourceGraph.Root, service, graph.EdgeTypeHelmPart)
	resourceGraph.AddEdge(resourceGraph.Root, deployment, graph.EdgeTypeHelmPart)

	model := NewGraphVisualizerModel(resourceGraph, 120, 40, keys.NewGraphRegistry())
	if got, want := model.viewport2d.Width, 114; got != want {
		t.Fatalf("graph viewport width = %d, want %d", got, want)
	}
	if model.viewport2d.CanScrollRight() {
		t.Fatalf("fitting graph unexpectedly has horizontal overflow: canvas=%d viewport-inner=%d", model.canvasWidth, model.viewport2d.innerWidth())
	}

	left := model.layout.nodePositions[service].X
	right := model.layout.nodePositions[deployment].X
	wantSpread := model.viewport2d.innerWidth() - 2*marginLeft - nodeWidth
	spread := right - left
	if spread < 0 {
		spread = -spread
	}
	if spread != wantSpread {
		t.Fatalf("child node spread = %d, want %d to use available width", spread, wantSpread)
	}
}

func TestMissingNodeRendersMissingStateOnce(t *testing.T) {
	node := &graph.Node{
		Resource: graphTestResource("Service", "sample", "Missing"),
		Metadata: map[string]string{"missing": "true"},
	}
	model := &GraphVisualizerModel{}

	rendered := ansi.Strip(model.renderNodeBox(node, false))
	if count := strings.Count(rendered, "Missing"); count != 1 {
		t.Fatalf("rendered missing count = %d, want 1: %q", count, rendered)
	}
	if !strings.Contains(rendered, "Service") {
		t.Fatalf("rendered node lost its resource kind: %q", rendered)
	}
}
