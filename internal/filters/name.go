package filters

import (
	"regexp"
	"strings"

	"github.com/miles-w-3/lobot/internal/k8s"
)

// ResourceNameFilter filters resources by name
type ResourceNameFilter struct {
	pattern string
	mode    FilterMode
	regex   *regexp.Regexp
}

// NewResourceNameFilter creates a new resource name filter
func NewResourceNameFilter() *ResourceNameFilter {
	return &ResourceNameFilter{
		pattern: "",
		mode:    FilterModeContains,
	}
}

// SetPattern updates the filter pattern
func (rf *ResourceNameFilter) SetPattern(pattern string) error {
	rf.pattern = pattern

	// Empty pattern means show all
	if pattern == "" {
		rf.mode = FilterModeContains
		rf.regex = nil
		return nil
	}

	// Determine filter mode
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) > 2 {
		// Regex mode: /pattern/
		rf.mode = FilterModeRegex
		regexPattern := pattern[1 : len(pattern)-1]
		regex, err := regexp.Compile(regexPattern)
		if err != nil {
			return err
		}
		rf.regex = regex
	} else {
		// Contains mode (default)
		rf.mode = FilterModeContains
		rf.regex = nil
	}

	return nil
}

// GetPattern returns the current filter pattern
func (rf *ResourceNameFilter) GetPattern() string {
	return rf.pattern
}

// FilterResources filters a list of resources by name
func (rf *ResourceNameFilter) FilterResources(resources []k8s.Resource) []k8s.Resource {
	// If no pattern, return all resources
	if rf.pattern == "" {
		return resources
	}

	filtered := make([]k8s.Resource, 0, len(resources))

	for _, resource := range resources {
		if rf.matches(resource.Name) {
			filtered = append(filtered, resource)
		}
	}

	return filtered
}

// matches checks if a resource name matches the filter
func (rf *ResourceNameFilter) matches(name string) bool {
	switch rf.mode {
	case FilterModeRegex:
		return rf.regex.MatchString(name)
	case FilterModeContains:
		return strings.Contains(strings.ToLower(name), strings.ToLower(rf.pattern))
	default:
		return true
	}
}
