package k8s

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// ResourceDiscovery handles discovering all available resource types in the cluster
type ResourceDiscovery struct {
	discoveryClient discovery.DiscoveryInterface
	cache           []ResourceType
	lastRefresh     time.Time
	mu              sync.RWMutex
	logger          *slog.Logger
}

// NewResourceDiscovery creates a new resource discovery service
func NewResourceDiscovery(client *Client, logger *slog.Logger) *ResourceDiscovery {
	if logger == nil {
		logger = slog.Default()
	}

	return &ResourceDiscovery{
		discoveryClient: client.Clientset.Discovery(),
		cache:           []ResourceType{},
		logger:          logger,
	}
}

// DiscoverAllResources discovers all available resource types in the cluster
// Returns both built-in types and CRDs, sorted alphabetically by display name
func (rd *ResourceDiscovery) DiscoverAllResources() ([]ResourceType, error) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	// Use cached results if fresh (less than 5 minutes old)
	if time.Since(rd.lastRefresh) < 5*time.Minute && len(rd.cache) > 0 {
		rd.logger.Debug("Using cached resource discovery results")
		return rd.cache, nil
	}

	rd.logger.Debug("Discovering all API resources")

	// Get preferred resources (one version per group)
	_, apiResourceLists, err := rd.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// Partial errors are common (some APIs may be unavailable)
		// We can still proceed with what we got
		rd.logger.Debug("Discovery returned partial results", "error", err)
	}

	resourceMap := make(map[string]ResourceType)

	for _, apiResourceList := range apiResourceLists {
		// Parse group/version from GroupVersion string
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			rd.logger.Debug("Failed to parse GroupVersion", "gv", apiResourceList.GroupVersion, "error", err)
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			// Skip subresources (e.g., "pods/log", "pods/status")
			if strings.Contains(apiResource.Name, "/") {
				continue
			}

			// Only include resources that support list and watch
			if !supportsVerb(apiResource.Verbs, "list") {
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: apiResource.Name,
			}

			// Create display name
			displayName := apiResource.Kind
			if displayName == "" {
				displayName = apiResource.Name
			}

			// Add group suffix for CRDs to distinguish them
			if gv.Group != "" && gv.Group != "apps" && gv.Group != "batch" &&
				gv.Group != "networking.k8s.io" && gv.Group != "autoscaling" {
				displayName = fmt.Sprintf("%s (%s)", displayName, gv.Group)
			}

			// Use a unique key to avoid duplicates (prefer newer versions)
			key := fmt.Sprintf("%s/%s", gv.Group, apiResource.Kind)
			resourceMap[key] = ResourceType{
				GVR:         gvr,
				DisplayName: displayName,
				Namespaced:  apiResource.Namespaced,
			}
		}
	}

	// Convert map to slice
	resources := make([]ResourceType, 0, len(resourceMap))
	for _, rt := range resourceMap {
		resources = append(resources, rt)
	}

	// Sort alphabetically by display name
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].DisplayName < resources[j].DisplayName
	})

	rd.cache = resources
	rd.lastRefresh = time.Now()

	rd.logger.Debug("Discovered API resources", "count", len(resources))

	return resources, nil
}

// RefreshCache forces a refresh of the resource cache
func (rd *ResourceDiscovery) RefreshCache() error {
	rd.mu.Lock()
	rd.lastRefresh = time.Time{} // Reset to force refresh
	rd.mu.Unlock()

	_, err := rd.DiscoverAllResources()
	return err
}

// supportsVerb checks if a resource supports a specific verb
func supportsVerb(verbs []string, verb string) bool {
	return slices.Contains(verbs, verb)
}

// DiscoverResourceName uses the discovery API to find the resource name for a given Kind
// This is used to convert owner references (which use Kind) to GVRs (which use resource name)
func (rd *ResourceDiscovery) DiscoverResourceName(gv schema.GroupVersion, kind string) (string, error) {
	// Get API resources for this group/version
	apiResourceList, err := rd.discoveryClient.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return "", err
	}

	// Find the resource that matches this Kind
	for _, apiResource := range apiResourceList.APIResources {
		if apiResource.Kind == kind {
			return apiResource.Name, nil
		}
	}

	// Fallback: try simple pluralization
	rd.logger.Debug("Kind not found in discovery, using simple pluralization",
		"kind", kind,
		"groupVersion", gv.String())
	return strings.ToLower(kind) + "s", nil
}
