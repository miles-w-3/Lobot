package k8s

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ServiceUpdateType represents the type of service update
type ServiceUpdateType int

const (
	ServiceUpdateResources ServiceUpdateType = iota
	ServiceUpdateReady
	ServiceUpdateError
)

// ServiceUpdate represents an update from the service
type ServiceUpdate struct {
	Type    ServiceUpdateType
	Context string
	Error   error
}

// UpdateCallback is called when the resource service has updates
type UpdateCallback func(ServiceUpdate)

const defaultInformerStartupConcurrency = 4

// ResourceService manages Kubernetes resources and abstracts the informer lifecycle.
// It handles context switching by recreating the underlying client and informer.
type ResourceService struct {
	onUpdate  UpdateCallback
	mu        sync.Mutex // Only protects Stop() and pointer swaps during context switch
	client    *Client
	informer  *InformerManager
	discovery *ResourceDiscovery
	logger    *slog.Logger
	ctx       context.Context
}

// NewResourceService creates a new resource service
func NewResourceService(ctx context.Context, client *Client, logger *slog.Logger) (*ResourceService, error) {
	if logger == nil {
		logger = slog.Default()
	}

	svc := &ResourceService{
		client: client,
		logger: logger,
		ctx:    ctx,
	}

	return svc, nil
}

// Set the callback and begin processing informers
func (svc *ResourceService) FinalizeConfiguration(onUpdate UpdateCallback) error {
	svc.onUpdate = onUpdate

	// Perform health check BEFORE initializing informers
	if !svc.performHealthCheck() {
		// Health check failed, error already sent to UI
		return nil
	}

	// Health check passed - proceed with initialization
	if err := svc.initializeInformer(); err != nil {
		svc.onUpdate(ServiceUpdate{
			Type:    ServiceUpdateError,
			Context: svc.client.Context,
			Error:   fmt.Errorf("failed to initialize informer: %w", err),
		})
		return nil
	}

	svc.logger.Debug("Initialization finished. Proceeding to mark as initialized")

	svc.informer.markInitialized()
	svc.logger.Debug("InformerManager Initialization complete")

	// Notify UI that initialization is complete and ready to use
	svc.onUpdate(ServiceUpdate{
		Type:    ServiceUpdateReady,
		Context: svc.client.Context,
	})

	return nil
}

// GetResources returns resources for the given GVR
func (svc *ResourceService) GetResources(gvr schema.GroupVersionResource) []TrackedObject {
	svc.mu.Lock()
	informer := svc.informer
	svc.mu.Unlock()
	if informer == nil {
		return nil
	}
	return informer.GetResources(gvr)
}

// GetNamespaces returns all namespaces in the cluster
func (svc *ResourceService) GetNamespaces() []string {
	svc.mu.Lock()
	informer := svc.informer
	svc.mu.Unlock()
	if informer == nil {
		return nil
	}
	return informer.GetNamespaces()
}

// GetClient returns the current Kubernetes client
func (svc *ResourceService) GetClient() *Client {
	return svc.client
}

// GetClusterName returns the current cluster name
func (svc *ResourceService) GetClusterName() string {
	if svc.client == nil {
		return "<Unknown Cluster>"
	}
	if svc.client.Context != "" {
		return svc.client.Context
	}
	if svc.client.ClusterName != "" {
		return svc.client.ClusterName
	}
	return "<Unknown Cluster>"
}

// GetResourcesByOwnerUID returns resources owned by the given UID
func (svc *ResourceService) GetResourcesByOwnerUID(uid string) []TrackedObject {
	svc.mu.Lock()
	informer := svc.informer
	svc.mu.Unlock()
	if informer == nil {
		return nil
	}
	return informer.GetResourcesByOwnerUID(uid)
}

// FetchResource fetches a resource using the dynamic client
// This is used for resources not in the cache (e.g., owner references)
func (svc *ResourceService) FetchResource(gvr schema.GroupVersionResource, name, namespace string, expectedUID string) TrackedObject {
	svc.mu.Lock()
	informer := svc.informer
	svc.mu.Unlock()
	if informer == nil {
		return nil
	}
	return informer.FetchResource(svc.ctx, gvr, name, namespace, expectedUID)
}

// DiscoverResourceName uses the discovery API to find the resource name for a given Kind
func (svc *ResourceService) DiscoverResourceName(gv schema.GroupVersion, kind string) (string, error) {
	if svc.discovery == nil {
		return "", fmt.Errorf("resource discovery is unavailable")
	}
	return svc.discovery.DiscoverResourceName(gv, kind)
}

// GetAllResourceTypes discovers all available resource types in the cluster
func (svc *ResourceService) GetAllResourceTypes() ([]*TrackedType, error) {
	if svc.discovery == nil {
		return nil, fmt.Errorf("resource discovery is unavailable")
	}
	return svc.discovery.DiscoverAllResources()
}

// PrepareEditFile prepares a resource for editing
func (svc *ResourceService) PrepareEditFile(resource TrackedObject) (*EditResult, error) {
	return svc.client.PrepareEditFile(resource)
}

// ProcessEditedFile processes an edited resource file
func (svc *ResourceService) ProcessEditedFile(ctx context.Context, resource TrackedObject, editResult *EditResult) error {
	return svc.client.ProcessEditedFile(ctx, resource, editResult)
}

// GetAllNamespaces queries the Kubernetes API for all namespace names
func (svc *ResourceService) GetAllNamespaces(ctx context.Context) ([]string, error) {
	if namespaces := svc.GetNamespaces(); len(namespaces) > 0 {
		return namespaces, nil
	}

	// Fall back to a direct request only while the Namespace informer is not
	// available (for example during partial startup).
	namespaceList, err := svc.client.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	namespaces := make([]string, 0, len(namespaceList.Items))
	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

// GetAvailableContexts returns all available contexts from kubeconfig
func (svc *ResourceService) GetAvailableContexts() ([]string, string, error) {
	return GetAvailableContexts()
}

// GetCurrentContext returns the current context name
func (svc *ResourceService) GetCurrentContext() string {
	return svc.client.Context
}

// SwitchContext switches to a new Kubernetes context
// This recreates the client and informer
func (svc *ResourceService) SwitchContext(contextName string) error {
	svc.logger.Info("Switching context", "context", contextName)

	// Stop old informer
	svc.mu.Lock()
	if svc.informer != nil {
		svc.informer.Stop()
		svc.informer = nil
	}
	svc.discovery = nil
	svc.mu.Unlock()

	// Create new client for the context
	newClient, err := NewClientWithContext(svc.logger, contextName)
	if err != nil {
		svc.logger.Error("Failed to create client for new context", "context", contextName, "error", err)
		svc.onUpdate(ServiceUpdate{
			Type:  ServiceUpdateError,
			Error: fmt.Errorf("failed to create client for context '%s': %w", contextName, err),
		})
		return err
	}

	// Swap in new client
	svc.mu.Lock()
	svc.client = newClient
	svc.mu.Unlock()

	// Perform health check on new context
	if !svc.performHealthCheck() {
		// Health check failed, error already sent to UI.
		// Return an error so callers do not continue with resource refreshes while
		// the informer is intentionally unavailable.
		return fmt.Errorf("unable to connect to context '%s'", contextName)
	}

	// Health check passed - initialize with new context
	if err := svc.initializeInformer(); err != nil {
		svc.onUpdate(ServiceUpdate{
			Type:  ServiceUpdateError,
			Error: fmt.Errorf("failed to initialize informer for context '%s': %w", contextName, err),
		})
		return err
	}

	svc.logger.Info("Successfully switched context", "context", contextName)

	// Mark that the informer manager can start sending callbacks
	svc.informer.markInitialized()

	// Notify observers that context switch is complete and system is ready
	svc.onUpdate(ServiceUpdate{
		Type:    ServiceUpdateReady,
		Context: contextName,
	})

	return nil
}

// StartInformer starts an informer for the given resource type
func (svc *ResourceService) StartInformer(resourceType *TrackedType) error {
	svc.mu.Lock()
	informer := svc.informer
	svc.mu.Unlock()
	if informer == nil {
		return fmt.Errorf("informer is unavailable")
	}
	return informer.StartInformer(svc.ctx, resourceType)
}

// IsResourceReady reports whether a resource type completed its initial sync.
func (svc *ResourceService) IsResourceReady(gvr schema.GroupVersionResource) bool {
	svc.mu.Lock()
	informer := svc.informer
	svc.mu.Unlock()
	return informer != nil && informer.IsResourceReady(gvr)
}

// GetLastUpdateTime returns the last time a resource type was updated
func (svc *ResourceService) GetLastUpdateTime(gvr schema.GroupVersionResource) time.Time {
	svc.mu.Lock()
	informer := svc.informer
	svc.mu.Unlock()
	if informer == nil {
		return time.Time{}
	}
	return informer.GetLastUpdateTime(gvr)
}

// Close cleans up the service
func (svc *ResourceService) Close() {
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.informer != nil {
		svc.informer.Stop()
	}

}

// performHealthCheck checks if the cluster is reachable and sends error to UI if not
// Returns true if healthy, false if unhealthy (error sent to UI)
func (svc *ResourceService) performHealthCheck() bool {
	healthCtx, cancel := context.WithTimeout(svc.ctx, 5*time.Second)
	defer cancel()

	if err := svc.client.HealthCheck(healthCtx); err != nil {
		// Send error to UI immediately
		svc.onUpdate(ServiceUpdate{
			Type:    ServiceUpdateError,
			Context: svc.client.Context,
			Error:   fmt.Errorf("unable to connect to cluster '%s': %w", svc.client.Context, err),
		})
		return false
	}

	return true
}

// initializeInformer creates and initializes the informer manager and graph builder
func (svc *ResourceService) initializeInformer() error {
	informer, err := NewInformerManager(svc.client, svc.logger, svc.onUpdate)
	if err != nil {
		return err
	}

	svc.logger.Debug("Initializing informer manager")

	svc.mu.Lock()
	svc.informer = informer
	svc.discovery = NewResourceDiscovery(svc.client, svc.logger)
	svc.mu.Unlock()

	startupTypes := startupResourceTypes()
	svc.logger.Debug("Starting foreground informers", "count", len(startupTypes), "concurrency", defaultInformerStartupConcurrency)
	startedAt := time.Now()
	svc.startInformers(informer, startupTypes)
	svc.logger.Debug("Foreground informers ready", "duration", time.Since(startedAt))

	backgroundTypes := backgroundResourceTypes(startupTypes)
	go func() {
		backgroundStartedAt := time.Now()
		svc.logger.Debug("Warming background informers", "count", len(backgroundTypes))
		svc.startInformers(informer, backgroundTypes)
		svc.logger.Debug("Background informers ready", "duration", time.Since(backgroundStartedAt))
	}()

	return nil
}

func startupResourceTypes() []*TrackedType {
	return []*TrackedType{
		PodResource,
		DeploymentResource,
		ServiceResource,
		ReplicaSetResource,
	}
}

func backgroundResourceTypes(startupTypes []*TrackedType) []*TrackedType {
	startup := make(map[schema.GroupVersionResource]bool, len(startupTypes))
	for _, resourceType := range startupTypes {
		startup[resourceType.GVR] = true
	}

	background := make([]*TrackedType, 0, len(DefaultResourceTypes())-len(startupTypes))
	for _, resourceType := range DefaultResourceTypes() {
		if !startup[resourceType.GVR] {
			background = append(background, resourceType)
		}
	}
	return background
}

func (svc *ResourceService) startInformers(informer *InformerManager, resourceTypes []*TrackedType) {
	// Cache synchronization is independent for each resource type. Bound the
	// concurrency so LIST requests overlap without overwhelming the API server.
	semaphore := make(chan struct{}, defaultInformerStartupConcurrency)
	var wg sync.WaitGroup
	for _, resourceType := range resourceTypes {
		wg.Add(1)
		go func(rt *TrackedType) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if err := informer.StartInformer(svc.ctx, rt); err != nil {
				svc.logger.Warn("Failed to start informer", "resource", rt.DisplayName, "error", err)
			}
		}(resourceType)
	}
	wg.Wait()
}

func (svc *ResourceService) GetPodLogs(podName, namespace string) []byte {
	pods := svc.client.Clientset.CoreV1().Pods(namespace)
	req := pods.GetLogs(podName, &v1.PodLogOptions{})
	body, err := req.DoRaw(context.Background())
	if err != nil { // TODO: Error handling
		return []byte{}
	}
	return body
}

func (svc *ResourceService) StreamPodLogs(podName string, namespace string, ctx context.Context) *io.ReadCloser {
	pods := svc.client.Clientset.CoreV1().Pods(namespace)
	req := pods.GetLogs(podName, &v1.PodLogOptions{
		Timestamps: true,
		Follow:     true,
	})
	reader, err := req.Stream(ctx)
	if err == nil {
		return nil
	}
	return &reader
}
