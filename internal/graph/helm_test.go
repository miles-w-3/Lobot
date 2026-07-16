package graph

import (
	"testing"

	"github.com/miles-w-3/lobot/internal/k8s"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type helmTestProvider struct {
	resources map[schema.GroupVersionResource][]k8s.TrackedObject
}

func (p *helmTestProvider) GetResources(gvr schema.GroupVersionResource) []k8s.TrackedObject {
	return p.resources[gvr]
}

func (*helmTestProvider) GetResourcesByOwnerUID(string) []k8s.TrackedObject { return nil }

func (*helmTestProvider) FetchResource(schema.GroupVersionResource, string, string, string) k8s.TrackedObject {
	return nil
}

func (*helmTestProvider) DiscoverResourceName(schema.GroupVersion, string) (string, error) {
	return "", nil
}

func helmTestBuilder(provider ResourceProvider) *Builder {
	builder := NewBuilder(provider, nil)
	builder.kindToGVR["v1/Service"] = schema.GroupVersionResource{Version: "v1", Resource: "services"}
	builder.kindToGVR["apps/v1/Deployment"] = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	return builder
}

func helmTestRelease() *k8s.HelmRelease {
	return &k8s.HelmRelease{
		CoreFields: k8s.CoreFields{
			Name:      "sample",
			Namespace: "release-ns",
			Status:    "deployed",
		},
		HelmManifest: `apiVersion: v1
kind: Service
metadata:
  name: sample
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample
`,
	}
}

func TestParseHelmManifestAppliesReleaseNamespace(t *testing.T) {
	builder := helmTestBuilder(&helmTestProvider{})
	resources := builder.parseHelmManifest(helmTestRelease().HelmManifest, "release-ns")
	if len(resources) != 2 {
		t.Fatalf("parsed resource count = %d, want 2", len(resources))
	}
	for _, resource := range resources {
		if got := resource.GetNamespace(); got != "release-ns" {
			t.Fatalf("%s namespace = %q, want release-ns", resource.GetKind(), got)
		}
	}
}

func TestBuildHelmGraphResolvesNamespacedResources(t *testing.T) {
	serviceGVR := schema.GroupVersionResource{Version: "v1", Resource: "services"}
	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	provider := &helmTestProvider{resources: map[schema.GroupVersionResource][]k8s.TrackedObject{
		serviceGVR: {
			&k8s.K8sResource{CoreFields: k8s.CoreFields{Name: "sample", Namespace: "release-ns", Status: "Active"}, Kind: "Service", GVR: serviceGVR},
		},
		deploymentGVR: {
			&k8s.K8sResource{CoreFields: k8s.CoreFields{Name: "sample", Namespace: "release-ns", Status: "Available"}, Kind: "Deployment", GVR: deploymentGVR},
		},
	}}

	resourceGraph := helmTestBuilder(provider).BuildHelmGraph(helmTestRelease())
	if len(resourceGraph.Nodes) != 3 {
		t.Fatalf("graph node count = %d, want release plus two resources", len(resourceGraph.Nodes))
	}
	for _, node := range resourceGraph.Nodes[1:] {
		if node.Metadata["missing"] == "true" || node.Resource.GetStatus() == "Missing" {
			t.Fatalf("present resource marked missing: %s/%s", node.Resource.GetKind(), node.Resource.GetName())
		}
	}
}

func TestBuildHelmGraphUsesStatusOnlyForMissingResources(t *testing.T) {
	resourceGraph := helmTestBuilder(&helmTestProvider{}).BuildHelmGraph(helmTestRelease())
	if len(resourceGraph.Nodes) != 3 {
		t.Fatalf("graph node count = %d, want release plus two resources", len(resourceGraph.Nodes))
	}
	for _, node := range resourceGraph.Nodes[1:] {
		if node.Metadata["missing"] != "true" {
			t.Fatalf("missing metadata not set for %s", node.Resource.GetKind())
		}
		if node.Resource.GetStatus() != "Missing" {
			t.Fatalf("status = %q, want Missing", node.Resource.GetStatus())
		}
		if node.Resource.GetKind() == "Service [Missing]" || node.Resource.GetKind() == "Deployment [Missing]" {
			t.Fatalf("missing state was redundantly appended to kind: %q", node.Resource.GetKind())
		}
	}
}
