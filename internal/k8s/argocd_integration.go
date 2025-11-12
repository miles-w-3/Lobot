package k8s

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// convertArgoApplicationToResource converts an ArgoCD Application CRD to a k8s.Resource
func convertArgoApplicationToResource(app *unstructured.Unstructured, gvr schema.GroupVersionResource) Resource {
	// Extract sync status from status.sync.status
	syncStatus, _, _ := unstructured.NestedString(app.Object, "status", "sync", "status")
	if syncStatus == "" {
		syncStatus = "Unknown"
	}

	// Extract health status from status.health.status
	health, _, _ := unstructured.NestedString(app.Object, "status", "health", "status")
	if health == "" {
		health = "Unknown"
	}

	// Extract source repository from spec.source.repoURL
	sourceRepo, _, _ := unstructured.NestedString(app.Object, "spec", "source", "repoURL")

	// Extract revision from status.sync.revision (actual deployed revision)
	revision, _, _ := unstructured.NestedString(app.Object, "status", "sync", "revision")
	if revision == "" {
		// Fall back to target revision from spec
		revision, _, _ = unstructured.NestedString(app.Object, "spec", "source", "targetRevision")
	}

	// Extract destination
	destServer, _, _ := unstructured.NestedString(app.Object, "spec", "destination", "server")
	destNamespace, _, _ := unstructured.NestedString(app.Object, "spec", "destination", "namespace")
	destination := fmt.Sprintf("%s/%s", destServer, destNamespace)
	if destServer == "" {
		destination = destNamespace
	}

	// Calculate age
	age := time.Duration(0)
	creationTime := app.GetCreationTimestamp()
	if !creationTime.IsZero() {
		age = time.Since(creationTime.Time)
	}

	return Resource{
		Name:              app.GetName(),
		Namespace:         app.GetNamespace(),
		Kind:              "Application",
		APIVersion:        "argoproj.io/v1alpha1",
		Status:            syncStatus, // Use sync status as primary status
		Age:               age,
		Labels:            app.GetLabels(),
		Raw:               app,
		GVR:               gvr,
		ArgoCDSyncStatus:  syncStatus,
		ArgoCDHealth:      health,
		ArgoCDSourceRepo:  sourceRepo,
		ArgoCDRevision:    revision,
		ArgoCDDestination: destination,
		IsArgoApplication: true,
	}
}

// isArgoManagedResource checks if a resource is managed by ArgoCD
func isArgoManagedResource(resource *Resource) bool {
	if resource.Labels == nil {
		return false
	}
	_, exists := resource.Labels["argocd.argoproj.io/instance"]
	return exists
}

// getArgoApplicationName returns the ArgoCD Application name that manages this resource
func getArgoApplicationName(resource *Resource) string {
	if resource.Labels == nil {
		return ""
	}
	return resource.Labels["argocd.argoproj.io/instance"]
}
