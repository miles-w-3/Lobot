package k8s

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// convertArgoApplicationToTrackedObject converts an ArgoCD Application CRD to a TrackedObject
func convertArgoApplicationToTrackedObject(app *unstructured.Unstructured, gvr schema.GroupVersionResource) TrackedObject {
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

	return &ArgoCDApp{
		CoreFields: CoreFields{
			Name:      app.GetName(),
			Namespace: app.GetNamespace(),
			Status:    syncStatus, // Use sync status as primary status
			Age:       age,
			Raw:       app,
		},
		APIVersion:  "argoproj.io/v1alpha1",
		Kind:        "Application",
		Labels:      app.GetLabels(),
		GVR:         gvr,
		SyncStatus:  syncStatus,
		Health:      health,
		SourceRepo:  sourceRepo,
		Revision:    revision,
		Destination: destination,
	}
}
