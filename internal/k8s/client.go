package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

/*
 * Track the scoped namespace. We are global if we have deselected namespace
 * this is done because kubernetes semantics equate ""/unset with the default namespace,
 * but in Lobot we default to global scope
 */
type CurrentScopedNamespace struct {
	value    string
	isGlobal bool
}

// Client wraps the Kubernetes clientset
type Client struct {
	Clientset   *kubernetes.Clientset
	Config      *rest.Config
	ClusterName string
	Context     string
	Logger      *slog.Logger
	ScopedNS    CurrentScopedNamespace
}

// NewClient creates a new Kubernetes client
// It attempts to load the kubeconfig from the following locations in order:
// 1. KUBECONFIG environment variable
// 2. ~/.kube/config
// 3. In-cluster config (when running inside a pod)
func NewClient(logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	var config *rest.Config
	var err error
	var context, clusterName string

	logger.Info("Loading Kubernetes configuration")

	// Try loading from kubeconfig
	config, context, clusterName, err = loadKubeConfigWithContext()
	logger.Debug("Got kubeconfig", "context", context, "cluster", clusterName)
	if err != nil {
		logger.Warn("Failed to load kubeconfig, falling back to in-cluster config", "error", err)
		// Fall back to in-cluster config TODO: I don't think we want to do this
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig or in-cluster config: %w", err)
		}
		context = "in-cluster"
		clusterName = "in-cluster"
	}

	logger.Info("Loaded Kubernetes configuration", "context", context, "cluster", clusterName)

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	logger.Info("Created Kubernetes clientset successfully")

	return &Client{
		Clientset:   clientset,
		Config:      config,
		ClusterName: clusterName,
		Context:     context,
		Logger:      logger,
		// by default, global scope - don't care what's in k8s context
		// could in future respect user config to load in from config
		ScopedNS: CurrentScopedNamespace{value: "", isGlobal: true},
	}, nil
}

// GetAvailableContexts returns all available contexts from kubeconfig
func GetAvailableContexts() ([]string, string, error) {
	kubeconfigPath := getKubeconfigPath()

	// Check if the file exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	// Load the kubeconfig file
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Get the raw config to extract context info
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load raw kubeconfig: %w", err)
	}

	// Extract all context names
	contexts := make([]string, 0, len(rawConfig.Contexts))
	for name := range rawConfig.Contexts {
		contexts = append(contexts, name)
	}

	return contexts, rawConfig.CurrentContext, nil
}

// NewClientWithContext creates a new Kubernetes client with a specific context
// The context override is in-memory only and does not modify the kubeconfig file
func NewClientWithContext(logger *slog.Logger, contextName string) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("Loading Kubernetes configuration with context override", "context", contextName)

	// Load kubeconfig with context override
	config, clusterName, err := loadKubeConfigWithContextOverride(contextName)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig with context %s: %w", contextName, err)
	}

	logger.Info("Loaded Kubernetes configuration", "context", contextName, "cluster", clusterName)

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	logger.Info("Created Kubernetes clientset successfully")

	return &Client{
		Clientset:   clientset,
		Config:      config,
		ClusterName: clusterName,
		Context:     contextName,
		Logger:      logger,
	}, nil
}

// getKubeconfigPath returns the path to the kubeconfig file
func getKubeconfigPath() string {
	// Check KUBECONFIG environment variable
	kubeconfigPath := os.Getenv("KUBECONFIG")

	// Fall back to default location
	if kubeconfigPath == "" {
		homeDir, _ := os.UserHomeDir()
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	return kubeconfigPath
}

// loadKubeConfigWithContext attempts to load kubeconfig from standard locations
// and returns the config along with context and cluster information
func loadKubeConfigWithContext() (*rest.Config, string, string, error) {
	config, clusterName, err := loadKubeConfigWithContextOverride("")
	if err != nil {
		return nil, "", "", err
	}

	// Get current context from kubeconfig
	_, currentContext, err := GetAvailableContexts()
	if err != nil {
		return nil, "", "", err
	}

	return config, currentContext, clusterName, nil
}

// loadKubeConfigWithContextOverride loads kubeconfig with optional context override
// If contextName is empty, uses the current context from kubeconfig
func loadKubeConfigWithContextOverride(contextName string) (*rest.Config, string, error) {
	kubeconfigPath := getKubeconfigPath()

	// Check if the file exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	// Load the kubeconfig file
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}

	// Apply context override if specified (in-memory only, does not modify file)
	if contextName != "" {
		configOverrides.CurrentContext = contextName
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Get the raw config to extract context and cluster info
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load raw kubeconfig: %w", err)
	}

	// Determine which context we're using
	effectiveContext := rawConfig.CurrentContext
	if contextName != "" {
		effectiveContext = contextName
	}

	// Get cluster name from context
	clusterName := effectiveContext // Default to context name
	if ctx, ok := rawConfig.Contexts[effectiveContext]; ok {
		clusterName = ctx.Cluster
	}

	// Build config from kubeconfig file
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, clusterName, nil
}

// HealthCheck verifies that the cluster is reachable
// Returns error if cluster is unreachable, nil otherwise
func (c *Client) HealthCheck(ctx context.Context) error {
	// Try a simple API call - discovery is lightweight
	// The context timeout is managed by the caller
	_, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("cluster unreachable: %w", err)
	}

	return nil
}
