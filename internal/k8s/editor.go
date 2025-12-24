package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

// EditResult contains information about the edit operation
type EditResult struct {
	TmpFilePath     string
	OriginalContent []byte
}

// PrepareEditFile creates a temporary file with the resource YAML for editing
// This should be called BEFORE suspending the TUI
// Only resources with Raw objects can be edited (not Helm releases)
func (c *Client) PrepareEditFile(resource TrackedObject) (*EditResult, error) {
	raw := resource.GetRaw()
	if raw == nil {
		return nil, fmt.Errorf("resource cannot be edited (no underlying Kubernetes object)")
	}

	c.Logger.Info("Preparing resource for editing",
		"name", resource.GetName(),
		"namespace", resource.GetNamespace(),
		"kind", resource.GetKind())

	// Marshal resource to YAML
	yamlBytes, err := yaml.Marshal(raw.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource to YAML: %w", err)
	}

	// Create temporary file
	tmpfile, err := os.CreateTemp("", fmt.Sprintf("lobot-%s-%s-*.yaml", resource.GetKind(), resource.GetName()))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpfilePath := tmpfile.Name()

	// Write YAML to temp file
	if _, err := tmpfile.Write(yamlBytes); err != nil {
		tmpfile.Close()
		os.Remove(tmpfilePath)
		return nil, fmt.Errorf("failed to write to temporary file: %w", err)
	}
	tmpfile.Close()

	c.Logger.Info("Created temporary file", "path", tmpfilePath)

	return &EditResult{
		TmpFilePath:     tmpfilePath,
		OriginalContent: yamlBytes,
	}, nil
}

// ProcessEditedFile reads, validates, and applies the edited resource
// This should be called AFTER the editor exits
func (c *Client) ProcessEditedFile(ctx context.Context, resource TrackedObject, editResult *EditResult) error {
	if editResult == nil {
		return fmt.Errorf("invalid edit result")
	}

	c.Logger.Info("Processing edited file", "path", editResult.TmpFilePath)

	// Read edited content
	editedBytes, err := os.ReadFile(editResult.TmpFilePath)
	if err != nil {
		return fmt.Errorf("failed to read edited file: %w", err)
	}

	// Check if content actually changed
	if bytes.Equal(editedBytes, editResult.OriginalContent) {
		c.Logger.Info("No changes detected, edit cancelled or no modifications made")
		return nil // Not an error - user cancelled or made no changes
	}

	c.Logger.Info("Changes detected, validating edited content")

	// Parse edited YAML
	var editedObj map[string]interface{}
	if err := yaml.Unmarshal(editedBytes, &editedObj); err != nil {
		return fmt.Errorf("failed to parse edited YAML (syntax error): %w", err)
	}

	// Validate the edited manifest
	if err := c.ValidateEditedManifest(resource, editedObj); err != nil {
		return err
	}

	c.Logger.Info("Validation passed, applying changes to cluster")

	// Apply the changes
	if err := c.UpdateResource(ctx, resource, editedObj); err != nil {
		return err
	}

	c.Logger.Info("Resource updated successfully")
	return nil
}

// ValidateEditedManifest validates that the edited manifest is a valid Kubernetes resource
func (c *Client) ValidateEditedManifest(original TrackedObject, editedObj map[string]interface{}) error {
	// Check required fields
	apiVersion, ok := editedObj["apiVersion"].(string)
	if !ok || apiVersion == "" {
		return fmt.Errorf("edited manifest missing required 'apiVersion' field")
	}

	kind, ok := editedObj["kind"].(string)
	if !ok || kind == "" {
		return fmt.Errorf("edited manifest missing required 'kind' field")
	}

	metadata, ok := editedObj["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("edited manifest missing required 'metadata' field")
	}

	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("edited manifest missing required 'metadata.name' field")
	}

	// Verify immutable fields haven't changed
	if kind != original.GetKind() {
		return fmt.Errorf("cannot change resource kind (original: %s, edited: %s)", original.GetKind(), kind)
	}

	if name != original.GetName() {
		return fmt.Errorf("cannot change resource name (original: %s, edited: %s)", original.GetName(), name)
	}

	// If namespaced, verify namespace hasn't changed
	originalNs := original.GetNamespace()
	if originalNs != "" {
		namespace, _ := metadata["namespace"].(string)
		if namespace != originalNs {
			return fmt.Errorf("cannot change resource namespace (original: %s, edited: %s)", originalNs, namespace)
		}
	}

	// Ensure resourceVersion is preserved from original for optimistic locking
	raw := original.GetRaw()
	if raw != nil {
		if origMetadata, ok := raw.Object["metadata"].(map[string]interface{}); ok {
			if resourceVersion, ok := origMetadata["resourceVersion"].(string); ok {
				metadata["resourceVersion"] = resourceVersion
				c.Logger.Debug("Preserved resourceVersion for optimistic locking", "version", resourceVersion)
			}
		}
	}

	c.Logger.Debug("Manifest validation passed", "name", name, "kind", kind)
	return nil
}

// UpdateResource updates a Kubernetes resource with new content
func (c *Client) UpdateResource(ctx context.Context, originalResource TrackedObject, editedObj map[string]interface{}) error {
	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(c.Config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Convert edited object to unstructured
	unstructuredObj := &unstructured.Unstructured{Object: editedObj}

	// Get GVR (GroupVersionResource) from the resource
	// Both K8sResource and ArgoCDApp have GVR fields
	var gvr schema.GroupVersionResource
	switch res := originalResource.(type) {
	case *K8sResource:
		gvr = res.GVR
	case *ArgoCDApp:
		gvr = res.GVR
	default:
		return fmt.Errorf("resource type %T cannot be edited", originalResource)
	}

	// Get resource interface
	var resourceInterface dynamic.ResourceInterface
	namespace := originalResource.GetNamespace()
	if namespace != "" {
		resourceInterface = dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		resourceInterface = dynamicClient.Resource(gvr)
	}

	c.Logger.Debug("Updating resource",
		"gvr", gvr.String(),
		"name", originalResource.GetName(),
		"namespace", namespace)

	// Update the resource
	_, err = resourceInterface.Update(ctx, unstructuredObj, metav1.UpdateOptions{})
	if err != nil {
		// Provide helpful error messages based on error type
		if errors.IsConflict(err) {
			return fmt.Errorf("conflict: resource was modified on the cluster after you opened the editor. "+
				"The resource version has changed. Please try editing again to get the latest version: %w", err)
		}

		if errors.IsInvalid(err) {
			return fmt.Errorf("validation failed: the edited manifest failed Kubernetes validation. "+
				"Check that all required fields are present and valid: %w", err)
		}

		if errors.IsNotFound(err) {
			return fmt.Errorf("not found: resource no longer exists on the cluster. "+
				"It may have been deleted while you were editing: %w", err)
		}

		if errors.IsForbidden(err) {
			return fmt.Errorf("forbidden: you don't have permission to update this resource: %w", err)
		}

		return fmt.Errorf("failed to update resource on cluster: %w", err)
	}

	return nil
}

