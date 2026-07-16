package k8s

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

func helmSecret(name, namespace, release, version string) TrackedObject {
	raw := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"type":       "helm.sh/release.v1",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"name":    release,
				"version": version,
			},
		},
	}}
	return &K8sResource{CoreFields: CoreFields{Name: name, Namespace: namespace, Raw: raw}, Kind: "Secret"}
}

func TestLatestHelmReleaseSecretsFiltersOldRevisionsBeforeDecode(t *testing.T) {
	secrets := []TrackedObject{
		helmSecret("sample-v1", "apps", "sample", "1"),
		helmSecret("sample-v2", "apps", "sample", "2"),
		helmSecret("other-v3", "apps", "other", "3"),
		helmSecret("legacy", "legacy", "", ""),
		&K8sResource{CoreFields: CoreFields{Name: "ordinary", Raw: &unstructured.Unstructured{}}, Kind: "Secret"},
	}

	latest, total := latestHelmReleaseSecrets(secrets)
	if total != 4 {
		t.Fatalf("Helm Secret count = %d, want 4", total)
	}
	if len(latest) != 3 {
		t.Fatalf("decode candidate count = %d, want 3", len(latest))
	}
	names := make(map[string]bool, len(latest))
	for _, secret := range latest {
		names[secret.GetName()] = true
	}
	if names["sample-v1"] || !names["sample-v2"] || !names["other-v3"] || !names["legacy"] {
		t.Fatalf("unexpected Helm decode candidates: %#v", names)
	}
}

func TestInitialInformerListReconcilesStoreOnce(t *testing.T) {
	const objectCount = 250
	gvr := ConfigMapResource.GVR
	objects := make([]runtime.Object, 0, objectCount)
	for i := 0; i < objectCount; i++ {
		objects = append(objects, &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      fmt.Sprintf("config-%03d", i),
				"namespace": "test",
			},
		}})
	}

	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "ConfigMapList"},
		objects...,
	)
	var callbacks atomic.Int32
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := &InformerManager{
		logger:          logger,
		dynamicClient:   dynamicClient,
		factory:         dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0),
		stopCh:          make(chan struct{}),
		resources:       make(map[schema.GroupVersionResource][]TrackedObject),
		activeInformers: make(map[schema.GroupVersionResource]cache.SharedIndexInformer),
		ownerIndex:      make(map[string][]TrackedObject),
		lastUpdateTime:  make(map[schema.GroupVersionResource]time.Time),
		readyInformers:  make(map[schema.GroupVersionResource]bool),
		updateCallback: func(ServiceUpdate) {
			callbacks.Add(1)
		},
	}
	manager.markInitialized()
	t.Cleanup(manager.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.StartInformer(ctx, ConfigMapResource); err != nil {
		t.Fatalf("StartInformer() error = %v", err)
	}

	if got := len(manager.GetResources(gvr)); got != objectCount {
		t.Fatalf("cached resource count = %d, want %d", got, objectCount)
	}
	if got := callbacks.Load(); got != 1 {
		t.Fatalf("initial callback count = %d, want one level-based reconciliation", got)
	}
	if !manager.IsResourceReady(gvr) {
		t.Fatal("resource was not marked ready after cache sync")
	}
}
