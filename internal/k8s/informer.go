package k8s

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// Resource represents a generic Kubernetes resource for display
// Note: This also represents "pseudo-resources" like Helm releases
type Resource struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
	Status     string
	Age        time.Duration
	Labels     map[string]string
	Raw        *unstructured.Unstructured
	GVR        schema.GroupVersionResource // The GVR this resource came from

	// Helm-specific fields (only populated for Helm releases)
	HelmChart     string // Chart name and version (e.g., "nginx-1.2.3")
	HelmManifest  string // The full manifest of deployed resources
	IsHelmRelease bool   // True if this is a Helm release
}

// ResourceType represents a Kubernetes resource type
type ResourceType struct {
	GVR         schema.GroupVersionResource
	DisplayName string
	Namespaced  bool
}

// Common resource types
var (
	// Core resources
	PodResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		DisplayName: "Pods",
		Namespaced:  true,
	}
	ServiceResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
		DisplayName: "Services",
		Namespaced:  true,
	}
	ConfigMapResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		DisplayName: "ConfigMaps",
		Namespaced:  true,
	}
	SecretResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
		DisplayName: "Secrets",
		Namespaced:  true,
	}
	PersistentVolumeClaimResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		DisplayName: "PersistentVolumeClaims",
		Namespaced:  true,
	}
	ServiceAccountResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
		DisplayName: "ServiceAccounts",
		Namespaced:  true,
	}

	// Apps resources
	DeploymentResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		DisplayName: "Deployments",
		Namespaced:  true,
	}
	ReplicaSetResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
		DisplayName: "ReplicaSets",
		Namespaced:  true,
	}
	StatefulSetResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
		DisplayName: "StatefulSets",
		Namespaced:  true,
	}
	DaemonSetResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
		DisplayName: "DaemonSets",
		Namespaced:  true,
	}

	// Batch resources
	JobResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
		DisplayName: "Jobs",
		Namespaced:  true,
	}
	CronJobResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
		DisplayName: "CronJobs",
		Namespaced:  true,
	}

	// Networking resources
	IngressResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		DisplayName: "Ingresses",
		Namespaced:  true,
	}

	// Autoscaling resources
	HorizontalPodAutoscalerResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		DisplayName: "HorizontalPodAutoscalers",
		Namespaced:  true,
	}

	// Cluster-scoped resources
	NamespaceResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
		DisplayName: "Namespaces",
		Namespaced:  false,
	}
	NodeResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"},
		DisplayName: "Nodes",
		Namespaced:  false,
	}
	PersistentVolumeResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"},
		DisplayName: "PersistentVolumes",
		Namespaced:  false,
	}

	// Special resource types
	HelmReleaseResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, // Uses secrets, but filtered
		DisplayName: "Helm Releases",
		Namespaced:  true,
	}
)

// DefaultResourceTypes returns a list of commonly used resource types
func DefaultResourceTypes() []ResourceType {
	return []ResourceType{
		// Special resource types
		HelmReleaseResource,

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
	client          *Client
	dynamicClient   dynamic.Interface
	factory         dynamicinformer.DynamicSharedInformerFactory
	stopCh          chan struct{}
	mu              sync.RWMutex
	resources       map[schema.GroupVersionResource][]Resource
	activeInformers map[schema.GroupVersionResource]cache.SharedIndexInformer
	updateCallback  func()
	ownerIndex      map[string][]Resource // Maps owner UID to owned resources
	helmResources   []Resource            // Cached Helm releases (handled separately)
	helmClient      HelmClientInterface   // Helm client interface (defined in helm_integration.go)
}

// NewInformerManager creates a new dynamic informer manager
func NewInformerManager(client *Client) (*InformerManager, error) {
	dynamicClient, err := dynamic.NewForConfig(client.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 30*time.Second)

	return &InformerManager{
		client:          client,
		dynamicClient:   dynamicClient,
		factory:         factory,
		stopCh:          make(chan struct{}),
		resources:       make(map[schema.GroupVersionResource][]Resource),
		activeInformers: make(map[schema.GroupVersionResource]cache.SharedIndexInformer),
		ownerIndex:      make(map[string][]Resource),
		helmResources:   []Resource{},
	}, nil
}

// SetUpdateCallback sets a callback function that gets called when resources are updated
func (im *InformerManager) SetUpdateCallback(callback func()) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.updateCallback = callback
}

// StartInformer starts an informer for a specific resource type
func (im *InformerManager) StartInformer(ctx context.Context, resourceType ResourceType) error {
	// Special handling for Helm releases - they use a polling mechanism instead of informers
	if resourceType.DisplayName == "Helm Releases" {
		return im.startHelmReleasePolling(ctx)
	}

	im.mu.Lock()

	// Check if informer already exists
	if _, exists := im.activeInformers[resourceType.GVR]; exists {
		im.mu.Unlock()
		return nil // Already running
	}

	// Create informer for this resource type
	informer := im.factory.ForResource(resourceType.GVR).Informer()

	// Add event handlers
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

// handleResourceUpdate updates the cached resources and triggers the callback
func (im *InformerManager) handleResourceUpdate(gvr schema.GroupVersionResource) {
	im.mu.Lock()

	informer, exists := im.activeInformers[gvr]
	if !exists {
		im.mu.Unlock()
		return
	}

	// Get all objects from the cache
	var resources []Resource
	store := informer.GetStore()

	for _, obj := range store.List() {
		if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
			resources = append(resources, convertUnstructuredToResource(unstructuredObj, gvr))
		}
	}

	// Sort resources by namespace and name for consistent ordering
	// This prevents resources from jumping around in the UI on refresh
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Namespace != resources[j].Namespace {
			return resources[i].Namespace < resources[j].Namespace
		}
		return resources[i].Name < resources[j].Name
	})

	im.resources[gvr] = resources

	// Rebuild owner index for fast lookups
	im.rebuildOwnerIndex()

	callback := im.updateCallback
	im.mu.Unlock()

	// Trigger update callback outside the lock
	if callback != nil {
		go callback()
	}
}

// GetResources returns the cached resources for a specific type
func (im *InformerManager) GetResources(gvr schema.GroupVersionResource) []Resource {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Special handling for Helm releases
	if gvr == HelmReleaseResource.GVR {
		result := make([]Resource, len(im.helmResources))
		copy(result, im.helmResources)
		return result
	}

	resources := im.resources[gvr]
	result := make([]Resource, len(resources))
	copy(result, resources)
	return result
}

// Stop stops all informers
func (im *InformerManager) Stop() {
	close(im.stopCh)
}

// convertUnstructuredToResource converts an unstructured object to our Resource type
func convertUnstructuredToResource(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) Resource {
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

	return Resource{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Kind:       obj.GetKind(),
		APIVersion: obj.GetAPIVersion(),
		Status:     status,
		Age:        time.Since(obj.GetCreationTimestamp().Time),
		Labels:     obj.GetLabels(),
		Raw:        obj,
		GVR:        gvr, // Include the GVR this resource came from
	}
}

// GetNamespaces returns all namespaces in the cluster
func (im *InformerManager) GetNamespaces() []string {
	resources := im.GetResources(NamespaceResource.GVR)
	namespaces := make([]string, len(resources))
	for i, ns := range resources {
		namespaces[i] = ns.Name
	}
	return namespaces
}

// rebuildOwnerIndex rebuilds the owner UID index from all cached resources
// Must be called with im.mu locked
func (im *InformerManager) rebuildOwnerIndex() {
	// Clear existing index
	im.ownerIndex = make(map[string][]Resource)

	// Iterate through all cached resources across all GVRs
	for _, resources := range im.resources {
		for _, resource := range resources {
			// Check if this resource has owner references
			if resource.Raw != nil {
				owners := resource.Raw.GetOwnerReferences()
				for _, owner := range owners {
					ownerUID := string(owner.UID)
					im.ownerIndex[ownerUID] = append(im.ownerIndex[ownerUID], resource)
				}
			}
		}
	}
}

// GetResourcesByOwnerUID returns all resources owned by a specific UID
func (im *InformerManager) GetResourcesByOwnerUID(ownerUID string) []Resource {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Return a copy to avoid concurrent modification issues
	owned := im.ownerIndex[ownerUID]
	result := make([]Resource, len(owned))
	copy(result, owned)
	return result
}

// GetDynamicClient returns the dynamic client for direct API calls
func (im *InformerManager) GetDynamicClient() dynamic.Interface {
	return im.dynamicClient
}

// startHelmReleasePolling starts a background goroutine that polls for Helm releases
func (im *InformerManager) startHelmReleasePolling(ctx context.Context) error {
	// Import helm package here to avoid circular dependency
	// We'll do the initial load synchronously
	if err := im.refreshHelmReleases(); err != nil {
		return fmt.Errorf("failed initial Helm release fetch: %w", err)
	}

	// Start background polling
	go func() {
		ticker := time.NewTicker(30 * time.Second)
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
					fmt.Printf("Error refreshing Helm releases: %v\n", err)
				}
			}
		}
	}()

	return nil
}

// refreshHelmReleases fetches the latest Helm releases and updates the cache
func (im *InformerManager) refreshHelmReleases() error {
	im.mu.RLock()
	helmClient := im.helmClient
	im.mu.RUnlock()

	if helmClient == nil {
		// No helm client configured, keep empty list
		return nil
	}

	// Call the implementation in helm_integration.go
	return im.refreshHelmReleasesImpl(helmClient)
}
