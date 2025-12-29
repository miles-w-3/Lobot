package k8s

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/miles-w-3/lobot/internal/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ObjectCategory identifies the category of tracked object
type ObjectCategory int

const (
	ObjectCategoryArgoCD ObjectCategory = iota
	ObjectCategoryHelm
	ObjectCategoryK8sResource
)

// CoreFields contains fields common to all tracked objects
type CoreFields struct {
	Name      string
	Namespace string
	Status    string
	Age       time.Duration
	Raw       *unstructured.Unstructured
}

// TrackedObject is the interface all resource types implement
type TrackedObject interface {
	GetName() string
	GetNamespace() string
	GetStatus() string
	GetAge() time.Duration
	GetRaw() *unstructured.Unstructured
	GetCategory() ObjectCategory
	GetKind() string
	DefaultRowBinding() table.Row
}

// K8sResource represents a standard Kubernetes resource
type K8sResource struct {
	CoreFields
	APIVersion string
	Kind       string
	Labels     map[string]string
	GVR        schema.GroupVersionResource
}

func (k *K8sResource) GetName() string                    { return k.Name }
func (k *K8sResource) GetNamespace() string               { return k.Namespace }
func (k *K8sResource) GetStatus() string                  { return k.Status }
func (k *K8sResource) GetAge() time.Duration              { return k.Age }
func (k *K8sResource) GetRaw() *unstructured.Unstructured { return k.Raw }
func (k *K8sResource) GetCategory() ObjectCategory        { return ObjectCategoryK8sResource }
func (k *K8sResource) GetKind() string                    { return k.Kind }

func (k *K8sResource) DefaultRowBinding() table.Row {
	if k.Namespace != "" {
		return table.Row{
			util.Truncate(k.Name, 40),
			util.Truncate(k.Namespace, 20),
			k.Status,
			util.FormatAge(k.Age),
		}
	}
	return table.Row{
		util.Truncate(k.Name, 60),
		k.Status,
		util.FormatAge(k.Age),
	}
}

// HelmRelease represents a Helm release
type HelmRelease struct {
	CoreFields
	HelmChart    string
	HelmRevision int
	HelmManifest string
	GVR          schema.GroupVersionResource // Pseudo-GVR
}

func (h *HelmRelease) GetName() string                    { return h.Name }
func (h *HelmRelease) GetNamespace() string               { return h.Namespace }
func (h *HelmRelease) GetStatus() string                  { return h.Status }
func (h *HelmRelease) GetAge() time.Duration              { return h.Age }
func (h *HelmRelease) GetRaw() *unstructured.Unstructured { return h.Raw }
func (h *HelmRelease) GetCategory() ObjectCategory        { return ObjectCategoryHelm }
func (h *HelmRelease) GetKind() string                    { return "HelmRelease" }

func (h *HelmRelease) DefaultRowBinding() table.Row {
	return table.Row{
		util.Truncate(h.Name, 25),
		util.Truncate(h.Namespace, 15),
		h.Status,
		util.Truncate(h.HelmChart, 25),
		fmt.Sprintf("%d", h.HelmRevision),
	}
}

// ArgoCDApp represents an ArgoCD Application
type ArgoCDApp struct {
	CoreFields
	APIVersion  string
	Kind        string
	Labels      map[string]string
	GVR         schema.GroupVersionResource
	SyncStatus  string
	Health      string
	SourceRepo  string
	Revision    string
	Destination string
}

func (a *ArgoCDApp) GetName() string                    { return a.Name }
func (a *ArgoCDApp) GetNamespace() string               { return a.Namespace }
func (a *ArgoCDApp) GetStatus() string                  { return a.Status }
func (a *ArgoCDApp) GetAge() time.Duration              { return a.Age }
func (a *ArgoCDApp) GetRaw() *unstructured.Unstructured { return a.Raw }
func (a *ArgoCDApp) GetCategory() ObjectCategory        { return ObjectCategoryArgoCD }
func (a *ArgoCDApp) GetKind() string                    { return a.Kind }

func (a *ArgoCDApp) DefaultRowBinding() table.Row {
	return table.Row{
		util.Truncate(a.Name, 25),
		util.Truncate(a.Namespace, 15),
		a.SyncStatus,
		a.Health,
		util.Truncate(a.SourceRepo, 35),
	}
}

// ResourceType represents a Tracked resource type. Includes categories like helm and ArgoCD too
type TrackedType struct {
	GVR         schema.GroupVersionResource
	DisplayName string
	Namespaced  bool
	Columns     []table.Column
	RowBinder   RowBinderFunc
}

func NewTrackedType(gvr schema.GroupVersionResource, name string, namespaced bool) *TrackedType {
	params := TableParams{
		columnOverride: nil,
		rowBinder:      nil,
	}
	return NewCustomTrackedType(gvr, name, namespaced, params)
}

type RowBinderFunc func(TrackedObject) table.Row

type TableParams struct {
	columnOverride []table.Column
	rowBinder      RowBinderFunc
}

func defaultRowBinder(object TrackedObject) table.Row {
	return object.DefaultRowBinding()
}

func NewCustomTrackedType(gvr schema.GroupVersionResource, name string, namespaced bool, tableParams TableParams) *TrackedType {
	var columns []table.Column
	var rowBinder RowBinderFunc
	columnOverride := tableParams.columnOverride
	// note go implicitly nil-checks
	if len(columnOverride) > 0 {
		columns = columnOverride
		if tableParams.rowBinder != nil {
			rowBinder = tableParams.rowBinder
		} else {
			rowBinder = defaultRowBinder
		}
	} else {
		if namespaced {
			columns = []table.Column{
				{Title: "NAME", Width: 40},
				{Title: "NAMESPACE", Width: 20},
				{Title: "STATUS", Width: 15},
				{Title: "AGE", Width: 10},
			}
		} else {
			columns = []table.Column{
				{Title: "NAME", Width: 60},
				{Title: "STATUS", Width: 15},
				{Title: "AGE", Width: 10},
			}
		}
		rowBinder = defaultRowBinder
	}

	return &TrackedType{
		DisplayName: name,
		GVR:         gvr,
		Namespaced:  namespaced,
		Columns:     columns,
		RowBinder:   rowBinder,
	}
}
