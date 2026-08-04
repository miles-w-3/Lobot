package modes

import "github.com/miles-w-3/lobot/internal/k8s"

// NamespaceOptionsReadyMsg carries namespace choices fetched for HomeScreen.
type NamespaceOptionsReadyMsg struct {
	RequestID uint64
	Options   []string
	Current   string
}

// ContextOptionsReadyMsg carries kubeconfig context choices fetched for
// HomeScreen.
type ContextOptionsReadyMsg struct {
	RequestID uint64
	Options   []string
	Current   string
}

// ResourceTypeOptionsReadyMsg carries discovered resource types for the Home
// resource-type selector.
type ResourceTypeOptionsReadyMsg struct {
	RequestID uint64
	Types     []*k8s.TrackedType
	Error     error
}

// HomeActivatedMsg is emitted by the persistent Home factory when Home is
// shown again. It refreshes the informer snapshot and clears stale selector
// requests that completed while another screen was active.
type HomeActivatedMsg struct{}

// ContextSwitchRequestedMsg is the only selector result that RootModel owns:
// switching contexts changes the application lifecycle and restarts Splash.
type ContextSwitchRequestedMsg struct {
	ContextName string
}

// ContextSwitchStartedMsg tells HomeScreen to discard cluster-specific local
// state before RootModel starts the new context lifecycle.
type ContextSwitchStartedMsg struct{}
