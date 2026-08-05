package modes

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
)

func testResourceGraph() *graph.ResourceGraph {
	root := &k8s.K8sResource{
		CoreFields: k8s.CoreFields{Name: "root", Namespace: "default", Status: "Running"},
		Kind:       "Deployment",
	}
	child := &k8s.K8sResource{
		CoreFields: k8s.CoreFields{Name: "child", Namespace: "default", Status: "Running"},
		Kind:       "Pod",
	}
	resourceGraph := graph.NewResourceGraph(root)
	childNode := resourceGraph.AddNode(child, graph.RelationshipOwner)
	resourceGraph.AddEdge(resourceGraph.Root, childNode, graph.EdgeTypeOwns)
	return resourceGraph
}

func TestVisualizerScreenAcceptsReadyMessageAfterFactoryCreation(t *testing.T) {
	screen := NewVisualizerScreen(nil, nil)
	if !screen.loading {
		t.Fatal("new visualizer screen is not loading")
	}

	screen.Update(VisualizerReadyMsg{Graph: testResourceGraph()})
	if screen.loading {
		t.Fatal("visualizer screen is still loading after graph readiness")
	}
	if screen.graph == nil || screen.treeView == nil {
		t.Fatal("visualizer graph was not installed by the ready message")
	}
}

func TestVisualizerScreenSwitchesBetweenTreeAndGraph(t *testing.T) {
	screen := NewVisualizerScreen(testResourceGraph(), nil)
	screen.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if screen.mode != visualizationTree {
		t.Fatalf("initial mode = %v, want tree", screen.mode)
	}

	screen.Update(tea.KeyPressMsg{Text: "V"})
	if screen.mode != visualizationGraph {
		t.Fatalf("mode after V = %v, want graph", screen.mode)
	}
	if screen.graphView == nil {
		t.Fatal("graph view was not initialized")
	}

	screen.Update(tea.KeyPressMsg{Text: "V"})
	if screen.mode != visualizationTree {
		t.Fatalf("mode after second V = %v, want tree", screen.mode)
	}
}

func TestVisualizerScreenBackNavigatesHome(t *testing.T) {
	screen := NewVisualizerScreen(testResourceGraph(), nil)
	cmd := screen.Update(tea.KeyPressMsg{Text: "esc"})
	if cmd == nil {
		t.Fatal("back returned no navigation command")
	}

	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Fatalf("back message = %T, want BackMsg", msg)
	}
}
