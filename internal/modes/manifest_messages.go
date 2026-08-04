package modes

import "github.com/miles-w-3/lobot/internal/k8s"

// ManifestRequestedMsg asks RootModel to activate the manifest screen and
// deliver a resource snapshot to it.
type ManifestRequestedMsg struct {
	Resource k8s.TrackedObject
}

// ManifestEditRequestedMsg asks RootModel to run the external editor workflow
// for the resource represented by the manifest screen.
type ManifestEditRequestedMsg struct {
	Resource k8s.TrackedObject
}

// ManifestCopyRequestedMsg asks RootModel to copy the raw manifest to the
// system clipboard.
type ManifestCopyRequestedMsg struct {
	Resource k8s.TrackedObject
}

// ManifestEditFinishedMsg reports completion of the external editor workflow.
type ManifestEditFinishedMsg struct {
	Resource k8s.TrackedObject
	Content  string
	Error    error
}
