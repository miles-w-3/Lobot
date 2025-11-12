package k8s

import (
	"fmt"
	"time"

	"github.com/miles-w-3/lobot/internal/helmutil"
)

// convertHelmReleaseToResource converts a decoded Helm release to a k8s.Resource for display
func convertHelmReleaseToResource(rel *helmutil.HelmRelease) Resource {
	// Format chart name and version
	chartName := "unknown"
	if rel.Chart.Metadata.Name != "" {
		chartName = fmt.Sprintf("%s-%s", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
	}

	// Calculate age
	age := time.Duration(0)
	if !rel.Info.FirstDeployed.IsZero() {
		age = time.Since(rel.Info.FirstDeployed)
	}

	return Resource{
		Name:          rel.Name,
		Namespace:     rel.Namespace,
		Kind:          "HelmRelease",
		APIVersion:    "helm.sh/v3",
		Status:        rel.Info.Status,
		Age:           age,
		Labels:        nil, // Helm releases don't have labels in the traditional sense
		Raw:           nil, // We don't have an unstructured.Unstructured for Helm releases
		GVR:           HelmReleaseResource.GVR,
		HelmChart:     chartName,
		HelmRevision:  rel.Version,
		HelmManifest:  rel.Manifest,
		IsHelmRelease: true,
	}
}

// helmReleasesChanged checks if the Helm releases have actually changed
func helmReleasesChanged(old, new []Resource) bool {
	if len(old) != len(new) {
		return true
	}

	// Create maps for comparison
	oldMap := make(map[string]Resource)
	for _, res := range old {
		key := res.Namespace + "/" + res.Name
		oldMap[key] = res
	}

	newMap := make(map[string]Resource)
	for _, res := range new {
		key := res.Namespace + "/" + res.Name
		newMap[key] = res
	}

	// Check if any release is missing or different
	for key, newRes := range newMap {
		oldRes, exists := oldMap[key]
		if !exists {
			return true // New release
		}

		// Compare relevant fields
		if oldRes.Status != newRes.Status ||
			oldRes.HelmChart != newRes.HelmChart ||
			oldRes.HelmRevision != newRes.HelmRevision {
			return true // Release changed
		}
	}

	// Check for deleted releases
	for key := range oldMap {
		if _, exists := newMap[key]; !exists {
			return true // Release deleted
		}
	}

	return false // No changes
}
