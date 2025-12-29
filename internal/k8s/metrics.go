package k8s

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// MetricsClient wraps the Kubernetes metrics clientset
type MetricsClient struct {
	clientset *metricsclientset.Clientset
	k8sClient *Client
	logger    *slog.Logger
}

// NodeMetrics represents node resource usage with capacity information
type NodeMetrics struct {
	Name            string
	CPUUsage        resource.Quantity // Current usage from metrics API
	MemoryUsage     resource.Quantity
	CPUCapacity     resource.Quantity // Total capacity from Node spec
	MemoryCapacity  resource.Quantity
	CPUAllocatable  resource.Quantity // Allocatable (capacity minus system reserved)
	MemAllocatable  resource.Quantity
	CPURequested    resource.Quantity // Sum of pod requests on this node
	MemoryRequested resource.Quantity
	// Node system info
	OS               string
	Architecture     string
	KernelVersion    string
	ContainerRuntime string
}

// ContainerMetrics represents resource usage for a single container
type ContainerMetrics struct {
	Name        string
	CPUUsage    resource.Quantity
	MemoryUsage resource.Quantity
}

// PodMetrics represents pod resource usage with requests/limits
type PodMetrics struct {
	Name        string
	Namespace   string
	NodeName    string
	Containers  []ContainerMetrics
	CPUUsage    resource.Quantity // Aggregated from containers
	MemoryUsage resource.Quantity
	CPURequest  resource.Quantity
	CPULimit    resource.Quantity
	MemRequest  resource.Quantity
	MemLimit    resource.Quantity
}

// CheckMetricsAPIAvailable checks if the metrics.k8s.io API is available
func (c *Client) CheckMetricsAPIAvailable(ctx context.Context) bool {
	// Check if the metrics.k8s.io API group is available via discovery
	apiGroups, err := c.Clientset.Discovery().ServerGroups()
	if err != nil {
		c.Logger.Warn("Failed to get server groups for metrics check", "error", err)
		return false
	}

	for _, group := range apiGroups.Groups {
		if group.Name == "metrics.k8s.io" {
			c.Logger.Debug("Metrics API is available")
			return true
		}
	}

	c.Logger.Debug("Metrics API is not available")
	return false
}

// NewMetricsClient creates a new metrics client
func NewMetricsClient(k8sClient *Client, logger *slog.Logger) (*MetricsClient, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create metrics clientset using the same config as the main client
	metricsClient, err := metricsclientset.NewForConfig(k8sClient.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics clientset: %w", err)
	}

	return &MetricsClient{
		clientset: metricsClient,
		k8sClient: k8sClient,
		logger:    logger,
	}, nil
}

// GetNodeMetrics fetches metrics for all nodes with capacity information
func (m *MetricsClient) GetNodeMetrics(ctx context.Context) ([]NodeMetrics, error) {
	// Fetch node metrics from metrics API
	nodeMetricsList, err := m.clientset.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch node metrics: %w", err)
	}

	// Fetch node specs for capacity information
	nodeList, err := m.k8sClient.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %w", err)
	}

	// Build a map of node name -> node spec for quick lookup
	nodeSpecMap := make(map[string]*corev1.Node)
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		nodeSpecMap[node.Name] = node
	}

	// Get pod requests per node
	podRequestsPerNode, err := m.getPodRequestsPerNode(ctx)
	if err != nil {
		m.logger.Warn("Failed to fetch pod requests per node", "error", err)
		// Continue without request info
		podRequestsPerNode = make(map[string]struct {
			cpu resource.Quantity
			mem resource.Quantity
		})
	}

	result := make([]NodeMetrics, 0, len(nodeMetricsList.Items))
	for _, nm := range nodeMetricsList.Items {
		nodeMetric := NodeMetrics{
			Name:        nm.Name,
			CPUUsage:    nm.Usage[corev1.ResourceCPU],
			MemoryUsage: nm.Usage[corev1.ResourceMemory],
		}

		// Add capacity and allocatable from node spec
		if nodeSpec, ok := nodeSpecMap[nm.Name]; ok {
			nodeMetric.CPUCapacity = nodeSpec.Status.Capacity[corev1.ResourceCPU]
			nodeMetric.MemoryCapacity = nodeSpec.Status.Capacity[corev1.ResourceMemory]
			nodeMetric.CPUAllocatable = nodeSpec.Status.Allocatable[corev1.ResourceCPU]
			nodeMetric.MemAllocatable = nodeSpec.Status.Allocatable[corev1.ResourceMemory]
			// Node system info
			nodeMetric.OS = nodeSpec.Status.NodeInfo.OperatingSystem
			nodeMetric.Architecture = nodeSpec.Status.NodeInfo.Architecture
			nodeMetric.KernelVersion = nodeSpec.Status.NodeInfo.KernelVersion
			nodeMetric.ContainerRuntime = nodeSpec.Status.NodeInfo.ContainerRuntimeVersion
		}

		// Add pod requests
		if requests, ok := podRequestsPerNode[nm.Name]; ok {
			nodeMetric.CPURequested = requests.cpu
			nodeMetric.MemoryRequested = requests.mem
		}

		result = append(result, nodeMetric)
	}

	return result, nil
}

// getPodRequestsPerNode calculates the sum of pod resource requests per node
func (m *MetricsClient) getPodRequestsPerNode(ctx context.Context) (map[string]struct {
	cpu resource.Quantity
	mem resource.Quantity
}, error) {
	result := make(map[string]struct {
		cpu resource.Quantity
		mem resource.Quantity
	})

	pods, err := m.k8sClient.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" || pod.Status.Phase != corev1.PodRunning {
			continue
		}

		nodeName := pod.Spec.NodeName
		entry := result[nodeName]

		for _, container := range pod.Spec.Containers {
			if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				entry.cpu.Add(cpu)
			}
			if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				entry.mem.Add(mem)
			}
		}

		result[nodeName] = entry
	}

	return result, nil
}

// GetPodMetrics fetches metrics for pods, optionally filtered by node
func (m *MetricsClient) GetPodMetrics(ctx context.Context, nodeName string) ([]PodMetrics, error) {
	// Fetch pod metrics from metrics API
	podMetricsList, err := m.clientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pod metrics: %w", err)
	}

	// Fetch pods for request/limit info and node assignment
	pods, err := m.k8sClient.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pods: %w", err)
	}

	// Build a map of pod key -> pod spec
	podSpecMap := make(map[string]*corev1.Pod)
	for i := range pods.Items {
		pod := &pods.Items[i]
		key := pod.Namespace + "/" + pod.Name
		podSpecMap[key] = pod
	}

	result := make([]PodMetrics, 0)
	for _, pm := range podMetricsList.Items {
		key := pm.Namespace + "/" + pm.Name
		podSpec, ok := podSpecMap[key]
		if !ok {
			continue
		}

		// Filter by node if specified
		if nodeName != "" && podSpec.Spec.NodeName != nodeName {
			continue
		}

		podMetric := PodMetrics{
			Name:       pm.Name,
			Namespace:  pm.Namespace,
			NodeName:   podSpec.Spec.NodeName,
			Containers: make([]ContainerMetrics, 0, len(pm.Containers)),
		}

		// Process container metrics
		for _, cm := range pm.Containers {
			containerMetric := ContainerMetrics{
				Name:        cm.Name,
				CPUUsage:    cm.Usage[corev1.ResourceCPU],
				MemoryUsage: cm.Usage[corev1.ResourceMemory],
			}
			podMetric.Containers = append(podMetric.Containers, containerMetric)

			// Aggregate usage
			podMetric.CPUUsage.Add(cm.Usage[corev1.ResourceCPU])
			podMetric.MemoryUsage.Add(cm.Usage[corev1.ResourceMemory])
		}

		// Get requests/limits from pod spec
		podMetric.CPURequest, podMetric.CPULimit, podMetric.MemRequest, podMetric.MemLimit = getPodResourceLimits(podSpec)

		result = append(result, podMetric)
	}

	return result, nil
}

// getPodResourceLimits extracts the aggregated requests and limits from a pod spec
func getPodResourceLimits(pod *corev1.Pod) (cpuReq, cpuLim, memReq, memLim resource.Quantity) {
	for _, container := range pod.Spec.Containers {
		if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			cpuReq.Add(cpu)
		}
		if cpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			cpuLim.Add(cpu)
		}
		if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			memReq.Add(mem)
		}
		if mem, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			memLim.Add(mem)
		}
	}
	return
}

// GetMetricsFromServer is a convenience function to check availability and get metrics
func (m *MetricsClient) GetMetricsFromServer(ctx context.Context) ([]NodeMetrics, []PodMetrics, error) {
	nodeMetrics, err := m.GetNodeMetrics(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get node metrics: %w", err)
	}

	podMetrics, err := m.GetPodMetrics(ctx, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	return nodeMetrics, podMetrics, nil
}

// Helper function to convert metrics API object to our type
func metricsToNodeMetrics(nm *metricsv1beta1.NodeMetrics) NodeMetrics {
	return NodeMetrics{
		Name:        nm.Name,
		CPUUsage:    nm.Usage[corev1.ResourceCPU],
		MemoryUsage: nm.Usage[corev1.ResourceMemory],
	}
}
