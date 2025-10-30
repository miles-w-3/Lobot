package helm

import (
	"fmt"
	"log/slog"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Client wraps the Helm action configuration for listing releases
type Client struct {
	actionConfig *action.Configuration
	settings     *cli.EnvSettings
	logger       *slog.Logger
}

// restConfigGetter implements the genericclioptions.RESTClientGetter interface
// This allows us to use a specific rest.Config instead of loading from kubeconfig file
type restConfigGetter struct {
	restConfig  *rest.Config
	namespace   string
}

func (r *restConfigGetter) ToRESTConfig() (*rest.Config, error) {
	return r.restConfig, nil
}

func (r *restConfigGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(r.restConfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(discoveryClient), nil
}

func (r *restConfigGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := r.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient), nil
}

func (r *restConfigGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return &simpleClientConfig{namespace: r.namespace}
}

// simpleClientConfig implements clientcmd.ClientConfig interface minimally for Helm
type simpleClientConfig struct {
	namespace string
}

func (c *simpleClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, nil
}

func (c *simpleClientConfig) ClientConfig() (*rest.Config, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *simpleClientConfig) Namespace() (string, bool, error) {
	if c.namespace == "" {
		return "default", false, nil
	}
	return c.namespace, false, nil
}

func (c *simpleClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}

// NewClient creates a new Helm client that uses the provided kubeConfig
func NewClient(kubeConfig *rest.Config, namespace string, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create Helm CLI settings
	settings := cli.New()

	// Override namespace if provided (empty string means all namespaces)
	if namespace != "" {
		settings.SetNamespace(namespace)
	}

	// Create a REST client getter that uses the provided kubeConfig
	// This ensures Helm uses the correct cluster context
	restGetter := &restConfigGetter{
		restConfig: kubeConfig,
		namespace:  namespace,
	}

	// Initialize action configuration
	actionConfig := new(action.Configuration)

	// We use "secret" as the storage driver (Helm's default in v3)
	err := actionConfig.Init(
		restGetter,
		settings.Namespace(),
		"secret",
		func(format string, v ...interface{}) {
			logger.Debug(fmt.Sprintf(format, v...))
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	return &Client{
		actionConfig: actionConfig,
		settings:     settings,
		logger:       logger,
	}, nil
}

// ListReleases returns all Helm releases in the configured namespace
// If namespace is empty, it searches all namespaces
func (c *Client) ListReleases(namespace string, allNamespaces bool) ([]*release.Release, error) {
	listAction := action.NewList(c.actionConfig)

	// Configure list options
	listAction.AllNamespaces = allNamespaces
	if namespace != "" && !allNamespaces {
		listAction.SetStateMask() // All states by default
	}

	// Filter to only show latest revisions
	listAction.All = false // Don't show all revisions

	// Execute the list action
	releases, err := listAction.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list Helm releases: %w", err)
	}

	c.logger.Debug("Listed Helm releases", "count", len(releases), "namespace", namespace, "allNamespaces", allNamespaces)

	return releases, nil
}

// GetRelease retrieves a specific Helm release by name
func (c *Client) GetRelease(name, namespace string) (*release.Release, error) {
	getAction := action.NewGet(c.actionConfig)

	rel, err := getAction.Run(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get Helm release %s: %w", name, err)
	}

	return rel, nil
}
