package k8s

import (
	"fmt"
	"time"

	"github.com/miles-w-3/lobot/internal/helmutil"
)

// convertHelmReleaseToTrackedObject converts a decoded Helm release to a TrackedObject
func convertHelmReleaseToTrackedObject(rel *helmutil.HelmRelease) TrackedObject {
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

	return &HelmRelease{
		CoreFields: CoreFields{
			Name:      rel.Name,
			Namespace: rel.Namespace,
			Status:    rel.Info.Status,
			Age:       age,
			Raw:       nil, // Helm releases don't have a k8s object
		},
		HelmChart:    chartName,
		HelmRevision: rel.Version,
		HelmManifest: rel.Manifest,
		GVR:          HelmReleaseResource.GVR,
	}
}

// helmReleasesChanged checks if the Helm releases have actually changed
func helmReleasesChanged(old, new []TrackedObject) bool {
	if len(old) != len(new) {
		return true
	}

	// Create maps for comparison
	oldMap := make(map[string]*HelmRelease)
	for _, res := range old {
		if helmRes, ok := res.(*HelmRelease); ok {
			key := helmRes.GetNamespace() + "/" + helmRes.GetName()
			oldMap[key] = helmRes
		}
	}

	newMap := make(map[string]*HelmRelease)
	for _, res := range new {
		if helmRes, ok := res.(*HelmRelease); ok {
			key := helmRes.GetNamespace() + "/" + helmRes.GetName()
			newMap[key] = helmRes
		}
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
