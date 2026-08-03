package k8s

import "testing"

func TestDefaultResourceTypesStartWithForegroundResources(t *testing.T) {
	defaults, startup := DefaultResourceTypes(), startupResourceTypes()
	if len(defaults) < len(startup) {
		t.Fatalf("default count %d < startup count %d", len(defaults), len(startup))
	}
	for i, want := range startup {
		if defaults[i].GVR != want.GVR {
			t.Fatalf("default %d = %s, want %s", i, defaults[i].DisplayName, want.DisplayName)
		}
	}
}

func TestBackgroundResourceTypesExcludeStartupSet(t *testing.T) {
	startup := startupResourceTypes()
	background := backgroundResourceTypes(startup)
	seen := make(map[string]bool)
	for _, rt := range startup {
		seen[rt.GVR.String()] = true
	}
	for _, rt := range background {
		key := rt.GVR.String()
		if seen[key] {
			t.Fatalf("%s appears in both startup sets", rt.DisplayName)
		}
		seen[key] = true
	}
	if len(seen) != len(DefaultResourceTypes()) {
		t.Fatalf("partition has %d resources, want %d", len(seen), len(DefaultResourceTypes()))
	}
}
