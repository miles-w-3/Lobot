package k8s

import (
	"fmt"
	"time"

	"helm.sh/helm/v3/pkg/release"
)

// HelmClientInterface allows us to work with helm client without import cycle
type HelmClientInterface interface {
	ListReleases(namespace string, allNamespaces bool) ([]*release.Release, error)
}

// refreshHelmReleasesImpl is the actual implementation that uses the helm client
func (im *InformerManager) refreshHelmReleasesImpl(helmClient HelmClientInterface) error {
	if helmClient == nil {
		return nil
	}

	// List all helm releases across all namespaces
	releases, err := helmClient.ListReleases("", true)
	if err != nil {
		return err
	}

	// Convert to Resource objects
	helmResources := make([]Resource, 0, len(releases))
	for _, rel := range releases {
		resource := convertReleaseToResource(rel)
		helmResources = append(helmResources, resource)
	}

	im.mu.Lock()
	im.helmResources = helmResources
	im.mu.Unlock()
	// im.logger.Debug("About to send update callback")
	im.sendCallback(ServiceUpdate{Type: ServiceUpdateResources})
	im.logger.Debug("Refreshed Helm releases", "count", len(helmResources))

	return nil
}

// convertReleaseToResource converts a Helm release to a k8s.Resource for display
// in the future, we could decouple the display from the k8s resource type
func convertReleaseToResource(rel *release.Release) Resource {
	// Format chart name and version
	chartName := "unknown"
	if rel.Chart != nil && rel.Chart.Metadata != nil {
		chartName = fmt.Sprintf("%s-%s", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
	}

	// Calculate age
	age := time.Duration(0)
	if rel.Info != nil && !rel.Info.FirstDeployed.IsZero() {
		age = time.Since(rel.Info.FirstDeployed.Time)
	}

	// Get status
	status := "unknown"
	if rel.Info != nil {
		status = rel.Info.Status.String()
	}

	// Get manifest
	manifest := ""
	if rel.Manifest != "" {
		manifest = rel.Manifest
	}

	return Resource{
		Name:          rel.Name,
		Namespace:     rel.Namespace,
		Kind:          "HelmRelease",
		APIVersion:    "helm.sh/v3",
		Status:        status,
		Age:           age,
		Labels:        nil, // Helm releases don't have labels in the traditional sense
		Raw:           nil, // We don't have an unstructured.Unstructured for Helm releases
		GVR:           HelmReleaseResource.GVR,
		HelmChart:     chartName,
		HelmRevision:  rel.Version,
		HelmManifest:  manifest,
		IsHelmRelease: true,
	}
}
