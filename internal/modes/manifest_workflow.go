package modes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"sigs.k8s.io/yaml"
)

func (r *RootModel) handleManifestRequest(msg ManifestRequestedMsg) tea.Cmd {
	if msg.Resource == nil || msg.Resource.GetRaw() == nil {
		r.showModal("Manifest Unavailable", "The selected resource has no manifest to display.")
		return nil
	}

	// The factory creates the destination screen without payload state. The
	// request is then delivered to that screen through its normal Update path.
	activateCmd := r.activateScreen(ScreenManifest)
	return tea.Batch(activateCmd, r.updateCurrentScreen(msg))
}

func (r *RootModel) startManifestEdit(msg ManifestEditRequestedMsg) tea.Cmd {
	resource := msg.Resource
	if resource == nil || resource.GetRaw() == nil {
		return func() tea.Msg {
			return ManifestEditFinishedMsg{Resource: resource, Error: fmt.Errorf("resource cannot be edited (no underlying Kubernetes object)")}
		}
	}
	if r.resourceService == nil {
		return func() tea.Msg {
			return ManifestEditFinishedMsg{Resource: resource, Error: fmt.Errorf("resource service is unavailable")}
		}
	}

	editResult, err := r.resourceService.PrepareEditFile(resource)
	if err != nil {
		return func() tea.Msg {
			return ManifestEditFinishedMsg{Resource: resource, Error: fmt.Errorf("failed to prepare edit: %w", err)}
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	service := r.resourceService
	resourceCopy := resource

	return tea.ExecProcess(exec.Command(editor, editResult.TmpFilePath), func(err error) tea.Msg {
		defer os.Remove(editResult.TmpFilePath)
		if err != nil {
			return ManifestEditFinishedMsg{
				Resource: resourceCopy,
				Error:    fmt.Errorf("editor exited with error: %w", err),
			}
		}

		processErr := service.ProcessEditedFile(context.Background(), resourceCopy, editResult)
		if processErr != nil {
			return ManifestEditFinishedMsg{Resource: resourceCopy, Error: processErr}
		}

		editedBytes, readErr := os.ReadFile(editResult.TmpFilePath)
		if readErr != nil {
			return ManifestEditFinishedMsg{
				Resource: resourceCopy,
				Error:    fmt.Errorf("failed to read edited manifest: %w", readErr),
			}
		}
		var editedObject map[string]interface{}
		if unmarshalErr := yaml.Unmarshal(editedBytes, &editedObject); unmarshalErr != nil {
			return ManifestEditFinishedMsg{
				Resource: resourceCopy,
				Error:    fmt.Errorf("failed to parse edited YAML: %w", unmarshalErr),
			}
		}
		return ManifestEditFinishedMsg{
			Resource: resourceCopy,
			Content:  formatManifestObject(editedObject),
		}
	})
}

func (r *RootModel) copyManifest(msg ManifestCopyRequestedMsg) tea.Cmd {
	resource := msg.Resource
	if resource == nil || resource.GetRaw() == nil {
		r.showModal("Copy Failed", "The selected resource has no raw manifest.")
		return nil
	}

	yamlBytes, err := yaml.Marshal(resource.GetRaw().Object)
	if err != nil {
		r.showModal("Copy Failed", "Failed to marshal YAML: "+err.Error())
		return nil
	}
	if err := clipboard.WriteAll(string(yamlBytes)); err != nil {
		r.showModal("Copy Failed", "Failed to copy to clipboard: "+err.Error())
	}
	return nil
}

func (r *RootModel) handleManifestEditFinished(msg ManifestEditFinishedMsg) tea.Cmd {
	if msg.Error == nil {
		if r.currentID == ScreenManifest {
			return r.updateCurrentScreen(msg)
		}
		return nil
	}

	title, message := manifestEditError(msg.Error)
	r.showModal(title, message)
	return nil
}

func manifestEditError(err error) (string, string) {
	errString := err.Error()
	switch {
	case strings.Contains(errString, "conflict:"):
		return "Conflict Detected", "The resource was modified on the cluster after you opened the editor.\n\n" +
			"The resource version has changed. Please try editing again to get the latest version."
	case strings.Contains(errString, "validation failed:"):
		return "Validation Failed", "The edited manifest failed Kubernetes validation.\n\n" +
			"Please check that all required fields are present and valid."
	case strings.Contains(errString, "not found:"):
		return "Resource Not Found", "The resource no longer exists on the cluster.\n\n" +
			"It may have been deleted while you were editing."
	case strings.Contains(errString, "cannot change resource"):
		return "Invalid Edit", "Cannot change immutable fields (name, kind, or namespace).\n\n" +
			"These fields are read-only after resource creation."
	case strings.Contains(errString, "failed to parse edited YAML"):
		return "YAML Syntax Error", "The edited YAML contains syntax errors.\n\n" +
			"Please check your YAML formatting."
	case strings.Contains(errString, "editor exited with error"):
		return "Editor Error", "The editor exited with an error.\n\n" +
			"Your changes were not saved."
	case strings.Contains(errString, "forbidden:"):
		return "Permission Denied", "You don't have permission to update this resource.\n\n" +
			"Check your RBAC permissions."
	default:
		return "Edit Failed", fmt.Sprintf("An error occurred while editing the resource:\n\n%s", errString)
	}
}
