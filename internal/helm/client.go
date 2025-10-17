package helm

import (
	"fmt"
	"log/slog"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/client-go/rest"
)

// Client wraps the Helm action configuration for listing releases
type Client struct {
	actionConfig *action.Configuration
	settings     *cli.EnvSettings
	logger       *slog.Logger
}

// NewClient creates a new Helm client
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

	// Initialize action configuration
	actionConfig := new(action.Configuration)

	// Initialize with the Kubernetes config
	// We use "secret" as the storage driver (Helm's default in v3)
	err := actionConfig.Init(
		settings.RESTClientGetter(),
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
