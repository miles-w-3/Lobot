package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/miles-w-3/lobot/internal/helmutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// Common resource types
var (
	// Core resources
	PodResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		"Pods",
		true,
	)
	ServiceResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
		"Services",
		true,
	)
	ConfigMapResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"ConfigMaps",
		true,
	)
	SecretResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
		"Secrets",
		true,
	)
	PersistentVolumeClaimResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"PersistentVolumeClaims",
		true,
	)
	ServiceAccountResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
		"ServiceAccounts",
		true,
	)

	// Apps resources
	DeploymentResource = NewTrackedType(
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		"Deployments",
		true,
	)
	ReplicaSetResource = NewTrackedType(
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		"ReplicaSets",
		true,
	)
	StatefulSetResource = NewTrackedType(
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		"StatefulSets",
		true,
	)
	DaemonSetResource = NewTrackedType(
		schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		"DaemonSets",
		true,
	)

	// Batch resources
	JobResource = NewTrackedType(
		schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
		"Jobs",
		true,
	)
	CronJobResource = NewTrackedType(
		schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
		"CronJobs",
		true,
	)

	// Networking resources
	IngressResource = NewTrackedType(
		schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"Ingresses",
		true,
	)

	// Autoscaling resources
	HorizontalPodAutoscalerResource = NewTrackedType(
		schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		"HorizontalPodAutoscalers",
		true,
	)

	// Cluster-scoped resources
	NamespaceResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		"Namespaces",
		false,
	)
	NodeResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
		"Nodes",
		false,
	)
	PersistentVolumeResource = NewTrackedType(
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"},
		"PersistentVolumes",
		false,
	)

	// Special resource types
	// Helm releases use a pseudo-GVR to avoid conflicting with actual secrets
	HelmReleaseResource = NewCustomTrackedType(
		schema.GroupVersionResource{Group: "helm.sh", Version: "v3", Resource: "releases"},
		"Helm Releases",
		true,
		TableParams{
			columnOverride: []table.Column{
				{Title: "NAME", Width: 30},
				{Title: "NAMESPACE", Width: 20},
				{Title: "STATUS", Width: 15},
				{Title: "VERSION", Width: 30},
			},
			rowBinder: nil, // Use default (DefaultRowBinding on HelmRelease)
		},
	)
	ApplicationResource = NewCustomTrackedType(
		schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"},
		"ArgoCD Applications",
		true,
		TableParams{
			columnOverride: []table.Column{
				{Title: "NAME", Width: 25},
				{Title: "NAMESPACE", Width: 15},
				{Title: "SYNC", Width: 12},
				{Title: "HEALTH", Width: 12},
				{Title: "SOURCE", Width: 35},
			},
			rowBinder: nil, // Use default (DefaultRowBinding on ArgoCDApp)
		},
	)
)

// DefaultResourceTypes returns a list of commonly used resource types
func DefaultResourceTypes() []*TrackedType {
	return []*TrackedType{
		// Special resource types
		HelmReleaseResource,
		ApplicationResource,

		// Core resources (most commonly viewed)
		PodResource,
		DeploymentResource,
		ServiceResource,
		ConfigMapResource,
		SecretResource,

		// Apps resources (important for ownership chains)
		ReplicaSetResource,
		StatefulSetResource,
		DaemonSetResource,

		// Batch resources
		JobResource,
		CronJobResource,

		// Storage resources
		PersistentVolumeClaimResource,

		// Networking resources
		IngressResource,

		// Identity resources
		ServiceAccountResource,

		// Autoscaling resources
		HorizontalPodAutoscalerResource,

		// Cluster-scoped resources
		NamespaceResource,
		NodeResource,
		PersistentVolumeResource,
	}
}

// InformerManager manages dynamic informers for any resource type
type InformerManager struct {
	client             *Client
	logger             *slog.Logger
	dynamicClient      dynamic.Interface
	factory            dynamicinformer.DynamicSharedInformerFactory
	stopCh             chan struct{}
	mu                 sync.RWMutex
	resources          map[schema.GroupVersionResource][]TrackedObject
	activeInformers    map[schema.GroupVersionResource]cache.SharedIndexInformer
	updateCallback     UpdateCallback
	ownerIndex         map[string][]TrackedObject // Maps owner UID to owned resources
	helmResources      []TrackedObject            // Cached Helm releases (decoded from secrets)
	helmPollingStarted bool                       // Tracks if Helm polling goroutine has been started
	isInitialized      bool
	lastUpdateTime     map[schema.GroupVersionResource]time.Time // Tracks when each resource type was last updated
}

// NewInformerManager creates a new dynamic informer manager
func NewInformerManager(client *Client, logger *slog.Logger, updateCallback UpdateCallback) (*InformerManager, error) {
	dynamicClient, err := dynamic.NewForConfig(client.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Resync period of 5 minutes reduces API load while still ensuring consistency
	// Previous 30-second period caused excessive API traffic for resources that rarely change
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 5*time.Minute)

	logger.Debug("Initializing informer...")

	return &InformerManager{
		logger:          logger,
		client:          client,
		dynamicClient:   dynamicClient,
		factory:         factory,
		stopCh:          make(chan struct{}),
		resources:       make(map[schema.GroupVersionResource][]TrackedObject),
		activeInformers: make(map[schema.GroupVersionResource]cache.SharedIndexInformer),
		ownerIndex:      make(map[string][]TrackedObject),
		helmResources:   []TrackedObject{},
		updateCallback:  updateCallback,
		isInitialized:   false,
		lastUpdateTime:  make(map[schema.GroupVersionResource]time.Time),
	}, nil
}

// SetUpdateCallback sets a callback function that gets called when resources are updated

// StartInformer starts an informer for a specific resource type
func (im *InformerManager) StartInformer(ctx context.Context, resourceType *TrackedType) error {
	// Special handling for Helm releases - they use a polling mechanism instead of informers
	if resourceType.DisplayName == "Helm Releases" {
		return im.startHelmReleasePolling(ctx)
	}

	// For CRD-based resources (like ArgoCD Applications), check if the CRD exists
	// Skip if not installed to avoid errors
	if resourceType.GVR.Group == "argoproj.io" {
		exists, err := im.checkResourceExists(resourceType.GVR)
		if err != nil {
			im.logger.Warn("Failed to check if resource exists",
				"resource", resourceType.DisplayName,
				"error", err)
			return nil // Don't fail, just skip
		}
		if !exists {
			im.logger.Debug("CRD not installed, skipping",
				"resource", resourceType.DisplayName,
				"gvr", resourceType.GVR)
			return nil // Not an error - CRD just isn't installed
		}
	}

	im.mu.Lock()

	// Check if informer already exists
	if _, exists := im.activeInformers[resourceType.GVR]; exists {
		im.mu.Unlock()
		// Informer already running, force a refresh from API server
		return im.forceRefreshFromAPI(ctx, resourceType.GVR)
	}

	// Create informer for this resource type
	informer := im.factory.ForResource(resourceType.GVR).Informer()

	// Add event handlers
	// Special case: For Secrets, also watch for Helm release secrets and trigger refresh
	if resourceType.GVR == SecretResource.GVR {
		_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				im.handleSecretUpdate(obj, resourceType.GVR)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				im.handleSecretUpdate(newObj, resourceType.GVR)
			},
			DeleteFunc: func(obj interface{}) {
				im.handleSecretUpdate(obj, resourceType.GVR)
			},
		})
		if err != nil {
			im.mu.Unlock()
			return fmt.Errorf("failed to add event handlers: %w", err)
		}
	} else {
		_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				im.handleResourceUpdate(resourceType.GVR)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				im.handleResourceUpdate(resourceType.GVR)
			},
			DeleteFunc: func(obj interface{}) {
				im.handleResourceUpdate(resourceType.GVR)
			},
		})
		if err != nil {
			im.mu.Unlock()
			return fmt.Errorf("failed to add event handlers: %w", err)
		}
	}

	im.activeInformers[resourceType.GVR] = informer
	im.mu.Unlock()

	// Start the informer
	go informer.Run(im.stopCh)

	// Wait for cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return fmt.Errorf("failed to sync cache for %s", resourceType.GVR.Resource)
	}

	// Do initial load
	im.handleResourceUpdate(resourceType.GVR)

	return nil
}

// checkResourceExists checks if a resource type exists on the API server
// Returns false if the CRD is not installed, true if it exists
func (im *InformerManager) checkResourceExists(gvr schema.GroupVersionResource) (bool, error) {
	// Try to list the resource with a limit of 1 to check if it exists
	// This is more efficient than checking the discovery API
	ctx := context.Background()
	_, err := im.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})

	if err != nil {
		// Check if it's a "not found" error (CRD doesn't exist)
		if apierrors.IsNotFound(err) ||
			(err.Error() != "" && (
			// Check for common "resource not found" error messages
			// Different k8s versions may return different error types
			err.Error() == "the server could not find the requested resource" ||
				err.Error() == "the server doesn't have a resource type \"applications\"")) {
			return false, nil // CRD doesn't exist, not an error
		}
		// Some other error occurred
		return false, err
	}

	// Resource exists and we can list it
	return true, nil
}

// handleSecretUpdate handles Secret updates and checks for Helm release secrets
func (im *InformerManager) handleSecretUpdate(obj interface{}, gvr schema.GroupVersionResource) {
	// First, handle the secret update normally
	im.handleResourceUpdate(gvr)

	// Check if this is a Helm release secret
	if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
		secretType, found, _ := unstructured.NestedString(unstructuredObj.Object, "type")
		if found && secretType == "helm.sh/release.v1" {
			// This is a Helm release secret - trigger Helm refresh
			im.logger.Debug("Helm release secret detected, triggering refresh",
				"name", unstructuredObj.GetName(),
				"namespace", unstructuredObj.GetNamespace())

			// Refresh Helm releases asynchronously to avoid blocking the informer
			go func() {
				if err := im.refreshHelmReleases(); err != nil {
					im.logger.Error("Failed to refresh Helm releases after secret change", "error", err)
				}
			}()
		}
	}
}

// handleResourceUpdate updates the cached resources and triggers the callback
func (im *InformerManager) handleResourceUpdate(gvr schema.GroupVersionResource) {
	im.mu.Lock()

	informer, exists := im.activeInformers[gvr]
	if !exists {
		im.mu.Unlock()
		return
	}

	// Get all objects from the cache
	var resources []TrackedObject
	store := informer.GetStore()

	for _, obj := range store.List() {
		if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
			resources = append(resources, ConvertUnstructuredToTrackedObject(unstructuredObj, gvr))
		}
	}

	// Sort resources by namespace and name for consistent ordering
	// This prevents resources from jumping around in the UI on refresh
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].GetNamespace() != resources[j].GetNamespace() {
			return resources[i].GetNamespace() < resources[j].GetNamespace()
		}
		return resources[i].GetName() < resources[j].GetName()
	})

	// Get old resources to determine what changed
	oldResources := im.resources[gvr]

	// Update cached resources
	im.resources[gvr] = resources

	// Update last update time
	im.lastUpdateTime[gvr] = time.Now()

	// Incrementally update owner index instead of full rebuild
	im.updateOwnerIndexForGVR(gvr, oldResources, resources)
	im.mu.Unlock()

	im.sendCallback(ServiceUpdate{Type: ServiceUpdateResources})
}

// forceRefreshFromAPI forces a refresh by directly querying the API server
// This bypasses the informer cache and provides up-to-date data when user explicitly refreshes
func (im *InformerManager) forceRefreshFromAPI(ctx context.Context, gvr schema.GroupVersionResource) error {
	im.logger.Debug("Forcing refresh from API server", "gvr", gvr)

	// List all resources from the API server
	list, err := im.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		im.logger.Error("Failed to list resources from API", "gvr", gvr, "error", err)
		return err
	}

	// Convert to our Resource type
	var resources []TrackedObject
	for _, item := range list.Items {
		resources = append(resources, ConvertUnstructuredToTrackedObject(&item, gvr))
	}

	// Sort resources by namespace and name for consistent ordering
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].GetNamespace() != resources[j].GetNamespace() {
			return resources[i].GetNamespace() < resources[j].GetNamespace()
		}
		return resources[i].GetName() < resources[j].GetName()
	})

	// Update the cache
	im.mu.Lock()
	oldResources := im.resources[gvr]
	im.resources[gvr] = resources
	im.lastUpdateTime[gvr] = time.Now()
	im.updateOwnerIndexForGVR(gvr, oldResources, resources)
	im.mu.Unlock()

	// Notify UI of the update
	im.sendCallback(ServiceUpdate{Type: ServiceUpdateResources})
	im.logger.Debug("Forced refresh complete", "gvr", gvr, "count", len(resources))

	return nil
}

// GetResources returns the cached resources for a specific type
func (im *InformerManager) GetResources(gvr schema.GroupVersionResource) []TrackedObject {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Special case: Helm releases are stored separately
	if gvr == HelmReleaseResource.GVR {
		result := make([]TrackedObject, len(im.helmResources))
		copy(result, im.helmResources)
		return result
	}

	resources := im.resources[gvr]
	result := make([]TrackedObject, len(resources))
	copy(result, resources)
	return result
}

// Stop stops all informers
func (im *InformerManager) Stop() {
	close(im.stopCh)
}

// ConvertUnstructuredToTrackedObject converts an unstructured object to a TrackedObject
func ConvertUnstructuredToTrackedObject(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) TrackedObject {
	// Special handling for ArgoCD Applications
	if gvr.Group == "argoproj.io" && gvr.Resource == "applications" {
		return convertArgoApplicationToTrackedObject(obj, gvr)
	}

	// Extract status if available
	status := "Unknown"
	if statusMap, found, _ := unstructured.NestedMap(obj.Object, "status"); found {
		// Try common status fields
		if phase, found, _ := unstructured.NestedString(statusMap, "phase"); found {
			status = phase
		} else if conditions, found, _ := unstructured.NestedSlice(statusMap, "conditions"); found && len(conditions) > 0 {
			// Get the last condition status
			if lastCond, ok := conditions[len(conditions)-1].(map[string]interface{}); ok {
				if condStatus, found := lastCond["status"]; found {
					status = fmt.Sprintf("%v", condStatus)
				}
			}
		}
	}

	return &K8sResource{
		CoreFields: CoreFields{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
			Status:    status,
			Age:       time.Since(obj.GetCreationTimestamp().Time),
			Raw:       obj,
		},
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Labels:     obj.GetLabels(),
		GVR:        gvr,
	}
}

// GetNamespaces returns all namespaces in the cluster
func (im *InformerManager) GetNamespaces() []string {
	resources := im.GetResources(NamespaceResource.GVR)
	namespaces := make([]string, len(resources))
	for i, ns := range resources {
		namespaces[i] = ns.GetName()
	}
	return namespaces
}

// rebuildOwnerIndex rebuilds the owner UID index from all cached resources
// Must be called with im.mu locked
// Only used during initialization - prefer updateOwnerIndexForGVR for incremental updates
func (im *InformerManager) rebuildOwnerIndex() {
	// Clear existing index
	im.ownerIndex = make(map[string][]TrackedObject)

	// Iterate through all cached resources across all GVRs
	for _, resources := range im.resources {
		for _, resource := range resources {
			// Check if this resource has owner references
			raw := resource.GetRaw()
			if raw != nil {
				owners := raw.GetOwnerReferences()
				for _, owner := range owners {
					ownerUID := string(owner.UID)
					im.ownerIndex[ownerUID] = append(im.ownerIndex[ownerUID], resource)
				}
			}
		}
	}
}

// updateOwnerIndexForGVR incrementally updates the owner index for a specific GVR
// This is much more efficient than rebuilding the entire index
// Must be called with im.mu locked
func (im *InformerManager) updateOwnerIndexForGVR(gvr schema.GroupVersionResource, oldResources, newResources []TrackedObject) {
	// Create maps for quick lookup
	oldMap := make(map[string]TrackedObject)
	for _, res := range oldResources {
		raw := res.GetRaw()
		if raw != nil {
			key := string(raw.GetUID())
			oldMap[key] = res
		}
	}

	newMap := make(map[string]TrackedObject)
	for _, res := range newResources {
		raw := res.GetRaw()
		if raw != nil {
			key := string(raw.GetUID())
			newMap[key] = res
		}
	}

	// Remove entries for deleted resources
	for uid, oldRes := range oldMap {
		if _, exists := newMap[uid]; !exists {
			// Resource was deleted - remove it from owner index
			im.removeResourceFromOwnerIndex(oldRes)
		}
	}

	// Add/update entries for new or changed resources
	for uid, newRes := range newMap {
		if oldRes, exists := oldMap[uid]; exists {
			// Resource exists - check if owner references changed
			oldRaw := oldRes.GetRaw()
			newRaw := newRes.GetRaw()
			if !ownerReferencesEqual(oldRaw, newRaw) {
				// Owner references changed - remove old, add new
				im.removeResourceFromOwnerIndex(oldRes)
				im.addResourceToOwnerIndex(newRes)
			}
			// Otherwise, owner references unchanged - no index update needed
		} else {
			// New resource - add to index
			im.addResourceToOwnerIndex(newRes)
		}
	}
}

// addResourceToOwnerIndex adds a resource's owner references to the index
// Must be called with im.mu locked
func (im *InformerManager) addResourceToOwnerIndex(resource TrackedObject) {
	raw := resource.GetRaw()
	if raw == nil {
		return
	}

	owners := raw.GetOwnerReferences()
	for _, owner := range owners {
		ownerUID := string(owner.UID)
		im.ownerIndex[ownerUID] = append(im.ownerIndex[ownerUID], resource)
	}
}

// removeResourceFromOwnerIndex removes a resource from the owner index
// Must be called with im.mu locked
func (im *InformerManager) removeResourceFromOwnerIndex(resource TrackedObject) {
	raw := resource.GetRaw()
	if raw == nil {
		return
	}

	resourceUID := string(raw.GetUID())
	owners := raw.GetOwnerReferences()

	for _, owner := range owners {
		ownerUID := string(owner.UID)
		// Remove this resource from the owner's owned list
		owned := im.ownerIndex[ownerUID]
		filtered := make([]TrackedObject, 0, len(owned))
		for _, r := range owned {
			rRaw := r.GetRaw()
			if rRaw != nil && string(rRaw.GetUID()) != resourceUID {
				filtered = append(filtered, r)
			}
		}

		if len(filtered) == 0 {
			// No more owned resources - remove the entry
			delete(im.ownerIndex, ownerUID)
		} else {
			im.ownerIndex[ownerUID] = filtered
		}
	}
}

// ownerReferencesEqual compares owner references between two resources
func ownerReferencesEqual(a, b *unstructured.Unstructured) bool {
	if a == nil || b == nil {
		return a == b
	}

	aOwners := a.GetOwnerReferences()
	bOwners := b.GetOwnerReferences()

	if len(aOwners) != len(bOwners) {
		return false
	}

	// Create map of UIDs for comparison
	aUIDs := make(map[string]bool)
	for _, owner := range aOwners {
		aUIDs[string(owner.UID)] = true
	}

	for _, owner := range bOwners {
		if !aUIDs[string(owner.UID)] {
			return false
		}
	}

	return true
}

// GetResourcesByOwnerUID returns all resources owned by a specific UID
func (im *InformerManager) GetResourcesByOwnerUID(ownerUID string) []TrackedObject {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Return a copy to avoid concurrent modification issues
	owned := im.ownerIndex[ownerUID]
	result := make([]TrackedObject, len(owned))
	copy(result, owned)
	return result
}

// GetLastUpdateTime returns the last time a resource type was updated
func (im *InformerManager) GetLastUpdateTime(gvr schema.GroupVersionResource) time.Time {
	im.mu.RLock()
	defer im.mu.RUnlock()

	return im.lastUpdateTime[gvr]
}

// GetDynamicClient returns the dynamic client for direct API calls
func (im *InformerManager) GetDynamicClient() dynamic.Interface {
	return im.dynamicClient
}

// FetchResource fetches a resource using the dynamic client
// This is used for resources not in the cache (e.g., owner references to resources we're not watching)
func (im *InformerManager) FetchResource(ctx context.Context, gvr schema.GroupVersionResource, name, namespace string, expectedUID string) TrackedObject {
	var obj *unstructured.Unstructured
	var err error

	obj, err = im.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// Try cluster-scoped if namespace lookup failed
		obj, err = im.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			im.logger.Debug("Failed to fetch resource via dynamic client",
				"gvr", gvr.String(),
				"name", name,
				"namespace", namespace,
				"error", err)
			return nil
		}
	}

	// If we were given a UID, verify UID matches (ensure we got the right resource)
	if expectedUID != "" && string(obj.GetUID()) != expectedUID {
		im.logger.Warn("Fetched resource UID mismatch",
			"expected", expectedUID,
			"got", obj.GetUID())
		return nil
	}

	resource := ConvertUnstructuredToTrackedObject(obj, gvr)
	return resource
}

// startHelmReleasePolling starts a background goroutine that polls for Helm releases
// Note: Primary updates come from watching Helm release Secrets (type: helm.sh/release.v1)
// This polling is just a safety net to catch any missed events
func (im *InformerManager) startHelmReleasePolling(ctx context.Context) error {
	im.mu.Lock()
	// Check if polling is already started to prevent multiple goroutines
	if im.helmPollingStarted {
		im.mu.Unlock()
		// Still refresh to update the data when user presses refresh
		// Force timestamp update so user sees the refresh happened
		return im.refreshHelmReleasesWithTimestamp(true)
	}
	im.helmPollingStarted = true
	im.mu.Unlock()

	// Import helm package here to avoid circular dependency
	// We'll do the initial load synchronously
	if err := im.refreshHelmReleases(); err != nil {
		return fmt.Errorf("failed initial Helm release fetch: %w", err)
	}

	// Start background polling as a safety net
	// Real-time updates come from Secret informer watching helm.sh/release.v1 secrets
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-im.stopCh:
				return
			case <-ticker.C:
				if err := im.refreshHelmReleases(); err != nil {
					// Log error but don't stop polling
					im.logger.Error("Error refreshing Helm releases during safety poll", "error", err)
				}
			}
		}
	}()

	return nil
}

// refreshHelmReleases decodes Helm releases from Secrets and updates the cache
// This reads helm.sh/release.v1 secrets directly instead of using the Helm SDK
// If forceUpdateTimestamp is true, the timestamp will be updated even if data hasn't changed
func (im *InformerManager) refreshHelmReleasesWithTimestamp(forceUpdateTimestamp bool) error {
	// Get all secrets from the cache
	secrets := im.GetResources(SecretResource.GVR)

	im.logger.Debug("Refreshing Helm releases", "totalSecrets", len(secrets))

	helmResources := []TrackedObject{}
	helmSecretCount := 0

	// Decode each Helm release secret
	for _, secret := range secrets {
		// Check if this is a Helm release secret
		raw := secret.GetRaw()
		if raw == nil {
			continue
		}

		secretType, found, _ := unstructured.NestedString(raw.Object, "type")
		if !found || secretType != "helm.sh/release.v1" {
			continue
		}

		helmSecretCount++
		im.logger.Debug("Found Helm release secret",
			"name", secret.GetName(),
			"namespace", secret.GetNamespace())

		// fetch release secret data
		ctx := context.Background()
		typedSecret, err := im.client.Clientset.CoreV1().Secrets(secret.GetNamespace()).Get(ctx, secret.GetName(), metav1.GetOptions{})
		if err != nil {
			im.logger.Warn("Failed to fetch typed Secret",
				"name", secret.GetName(),
				"namespace", secret.GetNamespace(),
				"error", err)
			continue
		}

		// Decode the Helm release from the typed Secret
		release, err := helmutil.DecodeHelmSecret(typedSecret, im.logger)
		if err != nil {
			im.logger.Warn("Failed to decode Helm secret",
				"name", secret.GetName(),
				"namespace", secret.GetNamespace(),
				"error", err)
			continue
		}

		// Convert to our Resource type
		helmResource := convertHelmReleaseToTrackedObject(release)
		helmResources = append(helmResources, helmResource)
	}

	im.logger.Debug("Helm release enumeration complete",
		"helmSecretsFound", helmSecretCount,
		"releasesDecoded", len(helmResources))

	// Update the cache and notify if changed
	im.mu.Lock()
	oldHelmResources := im.helmResources

	changed := helmReleasesChanged(oldHelmResources, helmResources)

	if changed {
		im.helmResources = helmResources
		im.lastUpdateTime[HelmReleaseResource.GVR] = time.Now()
		im.mu.Unlock()

		im.sendCallback(ServiceUpdate{Type: ServiceUpdateResources})
		im.logger.Debug("Helm releases changed", "count", len(helmResources))
	} else if forceUpdateTimestamp {
		// User explicitly refreshed, update timestamp even if data hasn't changed
		im.lastUpdateTime[HelmReleaseResource.GVR] = time.Now()
		im.mu.Unlock()
		im.logger.Debug("Helm releases unchanged but timestamp updated", "count", len(helmResources))
	} else {
		im.mu.Unlock()
		im.logger.Debug("Helm releases unchanged", "count", len(helmResources))
	}

	return nil
}

// refreshHelmReleases is a convenience wrapper for automatic polling
func (im *InformerManager) refreshHelmReleases() error {
	return im.refreshHelmReleasesWithTimestamp(false)
}

func (im *InformerManager) markInitialized() {
	im.isInitialized = true
}

func (im *InformerManager) sendCallback(callbackDetails ServiceUpdate) {
	if im.isInitialized {
		im.logger.Debug("Sending update callback")
		im.updateCallback(callbackDetails)
	} else {
		im.logger.Debug("Not sending update callback")
	}
}
