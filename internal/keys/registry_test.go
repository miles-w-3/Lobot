package keys

import "testing"

func TestV2KeyStringsDispatchThroughRegistries(t *testing.T) {
	tests := []struct {
		name string
		key  string
		run  func(string) error
	}{
		{
			name: "tree space alternate",
			key:  "space",
			run: func(key string) error {
				cmd, err := NewTreeRegistry().DispatchString(key)
				if err == nil && cmd != TreeCmdToggle {
					t.Fatalf("tree command = %v, want %v", cmd, TreeCmdToggle)
				}
				return err
			},
		},
		{
			name: "modal enter alternate",
			key:  "enter",
			run: func(key string) error {
				cmd, err := NewModalRegistry().DispatchString(key)
				if err == nil && cmd != ModalCmdBack {
					t.Fatalf("modal command = %v, want %v", cmd, ModalCmdBack)
				}
				return err
			},
		},
		{
			name: "selector accept",
			key:  "enter",
			run: func(key string) error {
				cmd, err := NewSelectorRegistry().DispatchString(key)
				if err == nil && cmd != SelectorCmdAccept {
					t.Fatalf("selector command = %v, want %v", cmd, SelectorCmdAccept)
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(tt.key); err != nil {
				t.Fatalf("dispatch %q: %v", tt.key, err)
			}
		})
	}
}
