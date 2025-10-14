package k8s

import (
	"context"
	"fmt"
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
type Resource struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
	Status     string
	Age        time.Duration
	Labels     map[string]string
	Raw        *unstructured.Unstructured
}

// ResourceType represents a Kubernetes resource type
type ResourceType struct {
	GVR         schema.GroupVersionResource
	DisplayName string
	Namespaced  bool
}

// Common resource types
var (
	PodResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		DisplayName: "Pods",
		Namespaced:  true,
	}
	DeploymentResource = ResourceType{
		GVR:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		DisplayName: "Deployments",
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
)

// DefaultResourceTypes returns a list of commonly used resource types
func DefaultResourceTypes() []ResourceType {
	return []ResourceType{
		PodResource,
		DeploymentResource,
		ServiceResource,
		ConfigMapResource,
		SecretResource,
		NamespaceResource,
		NodeResource,
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
			resources = append(resources, convertUnstructuredToResource(unstructuredObj))
		}
	}

	im.resources[gvr] = resources
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
func convertUnstructuredToResource(obj *unstructured.Unstructured) Resource {
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
