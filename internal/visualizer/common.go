package visualizer

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/graph"
	"github.com/miles-w-3/lobot/internal/k8s"
	"github.com/miles-w-3/lobot/internal/util"
)

func findRootNodes(resourceGraph *graph.ResourceGraph) []*graph.Node {
	if resourceGraph == nil {
		return nil
	}

	hasParent := make(map[*graph.Node]bool)
	for _, edge := range resourceGraph.Edges {
		if edge != nil && edge.To != nil {
			hasParent[edge.To] = true
		}
	}

	var roots []*graph.Node
	for _, node := range resourceGraph.Nodes {
		if node != nil && !hasParent[node] {
			roots = append(roots, node)
		}
	}
	return roots
}

func addNamespaceLabel(node *graph.Node, rootResource k8s.TrackedObject, baseName string) string {
	if node == nil || node.Resource == nil || node.Resource.GetNamespace() == "" || rootResource == nil {
		return baseName
	}

	if helmRes, ok := rootResource.(*k8s.HelmRelease); ok {
		if node.Resource.GetNamespace() != helmRes.GetNamespace() {
			return fmt.Sprintf("%s (ns: %s)", baseName, node.Resource.GetNamespace())
		}
		return baseName
	}

	if !node.IsRoot && node.Resource.GetNamespace() != rootResource.GetNamespace() {
		return fmt.Sprintf("%s (ns: %s)", baseName, node.Resource.GetNamespace())
	}
	return baseName
}

func formatResourceDesc(node *graph.Node) string {
	if node == nil || node.Resource == nil {
		return ""
	}

	status := node.Resource.GetStatus()
	style := lipgloss.NewStyle()
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "running"), strings.Contains(lower, "ready"), strings.Contains(lower, "active"):
		style = style.Foreground(util.ColorSuccess)
	case strings.Contains(lower, "pending"), strings.Contains(lower, "creating"):
		style = style.Foreground(util.ColorWarning)
	case strings.Contains(lower, "failed"), strings.Contains(lower, "error"):
		style = style.Foreground(util.ColorDanger)
	default:
		style = style.Foreground(util.ColorMuted)
	}

	return style.Render(fmt.Sprintf("[%s] %s", status, getStatusIndicator(status)))
}

func formatResourceDescPlain(node *graph.Node) string {
	if node == nil || node.Resource == nil {
		return ""
	}
	status := node.Resource.GetStatus()
	return fmt.Sprintf("[%s] %s", status, getStatusIndicator(status))
}

func formatNodeReference(node *graph.Node) string {
	if node == nil || node.Resource == nil {
		return ""
	}

	resource := node.Resource
	reference := fmt.Sprintf("%s/%s", resource.GetKind(), resource.GetName())
	if namespace := resource.GetNamespace(); namespace != "" {
		reference = namespace + "/" + reference
	}
	return reference
}

func getStatusIndicator(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "running"), strings.Contains(lower, "ready"), strings.Contains(lower, "active"):
		return "●"
	case strings.Contains(lower, "pending"), strings.Contains(lower, "creating"):
		return "◐"
	case strings.Contains(lower, "failed"), strings.Contains(lower, "error"):
		return "✗"
	default:
		return "○"
	}
}

// TODO: Improve this color association
func getColorForKind(kind string) string {
	switch kind {
	case "Pod":
		return "39"
	case "Deployment":
		return "33"
	case "ReplicaSet":
		return "37"
	case "Service":
		return "35"
	case "StatefulSet":
		return "34"
	case "DaemonSet":
		return "36"
	case "Job":
		return "32"
	case "CronJob":
		return "38"
	case "ConfigMap":
		return "220"
	case "Secret":
		return "208"
	case "Ingress":
		return "213"
	case "PersistentVolumeClaim":
		return "178"
	case "ServiceAccount":
		return "141"
	case "HorizontalPodAutoscaler":
		return "118"
	default:
		return "252"
	}
}
