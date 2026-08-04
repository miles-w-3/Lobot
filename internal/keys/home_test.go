package keys

import "testing"

func TestHomeRegistryDispatchesHomeCommands(t *testing.T) {
	registry := NewHomeRegistry()

	tests := []struct {
		key  string
		want HomeCmd
	}{
		{key: "up", want: HomeCmdMoveUp},
		{key: "k", want: HomeCmdMoveUp},
		{key: "left", want: HomeCmdPrevType},
		{key: "h", want: HomeCmdPrevType},
		{key: "/", want: HomeCmdFilter},
		{key: "enter", want: HomeCmdOpenManifest},
		{key: "E", want: HomeCmdEdit},
		{key: "ctrl+n", want: HomeCmdOpenNamespaceSelector},
		{key: "ctrl+t", want: HomeCmdOpenResourceTypeSelector},
		{key: "ctrl+k", want: HomeCmdOpenContextSelector},
		{key: "tab", want: HomeCmdToggleFavorites},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			got, err := registry.DispatchString(test.key)
			if err != nil {
				t.Fatalf("DispatchString(%q) error = %v", test.key, err)
			}
			if got != test.want {
				t.Fatalf("DispatchString(%q) = %v, want %v", test.key, got, test.want)
			}
		})
	}
}
