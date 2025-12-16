package helmutil

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// HelmRelease represents a Helm v3 release decoded from a Secret
type HelmRelease struct {
	Name      string      `json:"name"`
	Namespace string      `json:"namespace"`
	Info      ReleaseInfo `json:"info"`
	Chart     Chart       `json:"chart"`
	Manifest  string      `json:"manifest"`
	Version   int         `json:"version"`
}

// ReleaseInfo contains information about the release
type ReleaseInfo struct {
	Status        string    `json:"status"`
	FirstDeployed time.Time `json:"first_deployed"`
	LastDeployed  time.Time `json:"last_deployed"`
	Description   string    `json:"description"`
}

// Chart represents the Helm chart metadata
type Chart struct {
	Metadata ChartMetadata `json:"metadata"`
}

// ChartMetadata contains chart metadata
type ChartMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// DecodeHelmSecretTyped decodes Helm release from a typed Kubernetes Secret
func DecodeHelmSecret(secret *corev1.Secret, logger *slog.Logger) (*HelmRelease, error) {
	// Verify this is a Helm release secret
	if secret.Type != "helm.sh/release.v1" {
		return nil, fmt.Errorf("not a Helm release secret, type: %s", secret.Type)
	}

	logger.Debug("Secret", "name", secret.Name)

	// StringData will be base64 decoded, and then we can put it in bytes
	releaseData, ok := secret.Data["release"]
	if !ok {
		return nil, fmt.Errorf("release field not found in secret data")
	}

	releaseString := string(releaseData)
	decodedData, err := base64.StdEncoding.DecodeString(releaseString)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode helm secret data for %s", secret.Name)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(decodedData))
	if err != nil {
		return nil, fmt.Errorf("gzip decompress failed: %w", err)
	}
	defer gzipReader.Close()

	jsonData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("read decompressed data failed: %w", err)
	}

	var release HelmRelease
	if err := json.Unmarshal(jsonData, &release); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	// Ensure namespace is set (fall back to secret's namespace if not in data)
	if release.Namespace == "" {
		release.Namespace = secret.Namespace
	}

	return &release, nil
}

// IsHelmReleaseSecret checks if a Secret is a Helm release secret
func IsHelmReleaseSecret(secret *unstructured.Unstructured) bool {
	secretType, found, _ := unstructured.NestedString(secret.Object, "type")
	return found && secretType == "helm.sh/release.v1"
}

// DecodeHelmSecretFromUnstructured decodes Helm release from cached unstructured Secret data
// This avoids making additional API calls by using data already in the informer cache
func DecodeHelmSecretFromUnstructured(secret *unstructured.Unstructured, logger *slog.Logger) (*HelmRelease, error) {
	// Convert unstructured to typed Secret
	// This handles the first level of base64 decoding automatically
	var typedSecret corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(secret.Object, &typedSecret); err != nil {
		return nil, fmt.Errorf("failed to convert unstructured to typed secret: %w", err)
	}

	// Use the original decoding logic
	return DecodeHelmSecret(&typedSecret, logger)
}
