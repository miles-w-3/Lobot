package filters

import (
	"regexp"
	"strings"

	"github.com/miles-w-3/lobot/internal/k8s"
)

// FilterMode represents the type of filtering to apply
type FilterMode int

const (
	FilterModePrefix FilterMode = iota
	FilterModeContains
	FilterModeRegex
)

// NamespaceFilter handles filtering of resources by namespace
type NamespaceFilter struct {
	pattern    string
	mode       FilterMode
	regex      *regexp.Regexp
	allNamespaces bool
}

// NewNamespaceFilter creates a new namespace filter
func NewNamespaceFilter() *NamespaceFilter {
	return &NamespaceFilter{
		pattern:       "",
		mode:          FilterModeContains,
		allNamespaces: true,
	}
}

// SetPattern updates the filter pattern
func (nf *NamespaceFilter) SetPattern(pattern string) error {
	nf.pattern = pattern

	// Empty pattern means show all
	if pattern == "" {
		nf.allNamespaces = true
		nf.regex = nil
		return nil
	}

	nf.allNamespaces = false

	// Try to compile as regex for regex mode
	if nf.mode == FilterModeRegex {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
		nf.regex = regex
	} else {
		nf.regex = nil
	}

	return nil
}

// SetMode updates the filter mode
func (nf *NamespaceFilter) SetMode(mode FilterMode) {
	nf.mode = mode
	// Recompile pattern with new mode
	_ = nf.SetPattern(nf.pattern)
}

// GetPattern returns the current filter pattern
func (nf *NamespaceFilter) GetPattern() string {
	return nf.pattern
}

// GetMode returns the current filter mode
func (nf *NamespaceFilter) GetMode() FilterMode {
	return nf.mode
}

// Matches checks if a namespace matches the filter
func (nf *NamespaceFilter) Matches(namespace string) bool {
	// Empty filter matches everything
	if nf.allNamespaces {
		return true
	}

	switch nf.mode {
	case FilterModePrefix:
		return strings.HasPrefix(namespace, nf.pattern)
	case FilterModeContains:
		return strings.Contains(namespace, nf.pattern)
	case FilterModeRegex:
		if nf.regex != nil {
			return nf.regex.MatchString(namespace)
		}
		return true
	default:
		return true
	}
}

// FilterResources filters a list of resources by namespace
func (nf *NamespaceFilter) FilterResources(resources []k8s.Resource) []k8s.Resource {
	if nf.allNamespaces {
		return resources
	}

	filtered := make([]k8s.Resource, 0, len(resources))
	for _, resource := range resources {
		// Cluster-scoped resources (empty namespace) are always included
		if resource.Namespace == "" || nf.Matches(resource.Namespace) {
			filtered = append(filtered, resource)
		}
	}

	return filtered
}

// FilterNamespaces filters a list of namespace names
func (nf *NamespaceFilter) FilterNamespaces(namespaces []string) []string {
	if nf.allNamespaces {
		return namespaces
	}

	filtered := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		if nf.Matches(ns) {
			filtered = append(filtered, ns)
		}
	}

	return filtered
}
