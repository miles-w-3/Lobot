package ui

import (
	"charm.land/bubbles/v2/viewport"
	"github.com/miles-w-3/lobot/internal/k8s"
)

type WorkloadLogsModel struct {
	viewport viewport.Model
	// TODO: pass in client!
	resourceService *k8s.ResourceService
	// contextCancel   *context.CancelFunc
}

// TODO: First round is just pod logs
func NewWorkloadLogsModel(width, height int, resourceService *k8s.ResourceService) WorkloadLogsModel {
	logsViewport := viewport.New(viewport.WithHeight(height), viewport.WithWidth(width))

	model := WorkloadLogsModel{viewport: logsViewport, resourceService: resourceService}

	return model
}

func (m *WorkloadLogsModel) PopulateLogs(podName, namespace string) {
	// ctx, cancel := context.WithCancel(context.Background())
	// m.contextCancel = &cancel

	result := m.resourceService.GetPodLogs(podName, namespace)
	// var b strings.Builder
	// for _, line := range result {

	// }

	m.viewport.SetContent(string(result))
}

func (m *WorkloadLogsModel) View() string {
	return m.viewport.View()
}
