package k8s

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset
type Client struct {
	Clientset   *kubernetes.Clientset
	Config      *rest.Config
	ClusterName string
	Context     string
	Logger      *slog.Logger
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
		// Fall back to in-cluster config
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
	}, nil
}

// loadKubeConfigWithContext attempts to load kubeconfig from standard locations
// and returns the config along with context and cluster information
func loadKubeConfigWithContext() (*rest.Config, string, string, error) {
	// Check KUBECONFIG environment variable
	kubeconfigPath := os.Getenv("KUBECONFIG")

	// Fall back to default location
	if kubeconfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	// Check if the file exists
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil, "", "", fmt.Errorf("kubeconfig file not found at %s", kubeconfigPath)
	}

	// Load the kubeconfig file
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Get the raw config to extract context and cluster info
	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to load raw kubeconfig: %w", err)
	}

	// Get current context
	currentContext := rawConfig.CurrentContext
	clusterName := currentContext // Default to context name

	// Try to get the actual cluster name from the context
	if ctx, ok := rawConfig.Contexts[currentContext]; ok {
		clusterName = ctx.Cluster
	}

	// Build config from kubeconfig file
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, currentContext, clusterName, nil
}
