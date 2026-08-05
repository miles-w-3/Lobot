package modes

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/miles-w-3/lobot/internal/k8s"
	"k8s.io/apimachinery/pkg/api/resource"
)

func utilizationTestMetrics() ([]k8s.NodeMetrics, []k8s.PodMetrics) {
	nodes := []k8s.NodeMetrics{
		{
			Name:           "node-a",
			CPUUsage:       resource.MustParse("500m"),
			CPUAllocatable: resource.MustParse("2"),
			MemoryUsage:    resource.MustParse("1Gi"),
			MemAllocatable: resource.MustParse("4Gi"),
			OS:             "linux",
			Architecture:   "arm64",
		},
		{
			Name:           "node-b",
			CPUUsage:       resource.MustParse("250m"),
			CPUAllocatable: resource.MustParse("1"),
			MemoryUsage:    resource.MustParse("512Mi"),
			MemAllocatable: resource.MustParse("2Gi"),
		},
	}
	pods := []k8s.PodMetrics{
		{
			Name:        "pod-a",
			Namespace:   "default",
			NodeName:    "node-a",
			CPUUsage:    resource.MustParse("100m"),
			MemoryUsage: resource.MustParse("128Mi"),
		},
		{
			Name:        "pod-b",
			Namespace:   "default",
			NodeName:    "node-b",
			CPUUsage:    resource.MustParse("50m"),
			MemoryUsage: resource.MustParse("64Mi"),
		},
	}
	return nodes, pods
}

func TestUtilizationScreenPreservesRegistryBindings(t *testing.T) {
	nodes, pods := utilizationTestMetrics()
	screen := NewUtilizationScreen(nodes, pods, 100, 30)

	screen.Update(tea.KeyPressMsg{Text: "tab"})
	if screen.resourceCategory != ResourceCategoryMemory {
		t.Fatalf("tab category = %v, want memory", screen.resourceCategory)
	}
	screen.Update(tea.KeyPressMsg{Text: "tab"})
	if screen.resourceCategory != ResourceCategoryCPU {
		t.Fatalf("second tab category = %v, want CPU", screen.resourceCategory)
	}

	screen.Update(tea.KeyPressMsg{Text: "down"})
	screen.Update(tea.KeyPressMsg{Text: "right"})
	screen.Update(tea.KeyPressMsg{Text: "down"})
	if screen.focusedPanel != FocusPanelPods || screen.selectedPod != 0 {
		t.Fatalf("pod focus/selection = %v/%d, want pods/0", screen.focusedPanel, screen.selectedPod)
	}

	screen.Update(tea.KeyPressMsg{Text: "d"})
	if !screen.showNodeDetails || len(screen.modalPods) != 1 {
		t.Fatalf("details state = %v with %d modal pods, want open with 1 pod", screen.showNodeDetails, len(screen.modalPods))
	}
	screen.Update(tea.KeyPressMsg{Text: "esc"})
	if screen.showNodeDetails {
		t.Fatal("first esc did not close details")
	}

	cmd := screen.Update(tea.KeyPressMsg{Text: "esc"})
	if cmd == nil {
		t.Fatal("second esc returned no navigation command")
	}
	msg := cmd()
	if _, ok := msg.(BackMsg); !ok {
		t.Fatalf("second esc message = %#v, want BackMsg", msg)
	}
}

func TestUtilizationScreenFiltersPodsByNode(t *testing.T) {
	nodes, pods := utilizationTestMetrics()
	screen := NewUtilizationScreen(nodes, pods, 100, 30)

	screen.Update(tea.KeyPressMsg{Text: "down"})
	if screen.selectedNode != 0 || len(screen.filteredPods) != 1 || screen.filteredPods[0].Name != "pod-a" {
		t.Fatalf("node selection/filter = %d/%v, want node 0 and pod-a", screen.selectedNode, screen.filteredPods)
	}

	screen.Update(tea.KeyPressMsg{Text: "up"})
	if screen.selectedNode != -1 || len(screen.filteredPods) != 2 {
		t.Fatalf("all-node selection/filter = %d/%d, want -1/2", screen.selectedNode, len(screen.filteredPods))
	}
}

func TestUtilizationScreenLoadingAndReadyStates(t *testing.T) {
	screen := newLoadingUtilizationScreen(100, 30)
	if !strings.Contains(screen.View(), "Loading metrics") {
		t.Fatal("loading screen did not render loading state")
	}

	nodes, pods := utilizationTestMetrics()
	screen.Update(UtilizationReadyMsg{NodeMetrics: nodes, PodMetrics: pods})
	if screen.loading || screen.loadError != nil {
		t.Fatalf("ready state = loading=%v error=%v", screen.loading, screen.loadError)
	}
	if !strings.Contains(screen.View(), "CLUSTER UTILIZATION") {
		t.Fatal("ready screen did not render dashboard")
	}
}

func TestUtilizationPodRowsDoNotWrapUsageOntoNextLine(t *testing.T) {
	screen := NewUtilizationScreen(nil, []k8s.PodMetrics{{
		Name:        "local-path-provisioner-5d9d9885-long-name",
		CPUUsage:    resource.MustParse("100m"),
		MemoryUsage: resource.MustParse("128Mi"),
	}}, 100, 30)

	rendered := screen.renderPodsPanel(48, 20)
	if got := lipgloss.Width(rendered); got != 48 {
		t.Fatalf("pod panel width = %d, want allocated width 48", got)
	}
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(ansi.Strip(line)) == "100m" {
			t.Fatalf("usage wrapped onto its own line:\n%s", rendered)
		}
	}
}

func TestUtilizationStackedBarShowsZeroUsagePods(t *testing.T) {
	screen := &UtilizationScreen{
		modalPods: []k8s.PodMetrics{
			{Name: "zero-a"},
			{Name: "zero-b"},
			{Name: "zero-c"},
		},
		resourceCategory: ResourceCategoryCPU,
	}

	bar, _ := screen.renderStackedBarWithSelection(10, 1000, 0, podColors)
	if got := strings.Count(bar, "█"); got != 3 {
		t.Fatalf("zero-usage segments = %d, want 3", got)
	}
}
