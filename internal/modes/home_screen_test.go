package modes

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/miles-w-3/lobot/internal/k8s"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNextResourceTypeClearsRowsBeforeChangingColumns(t *testing.T) {
	screen := NewHomeScreen(nil, nil)
	secondType := k8s.NewTrackedType(
		schema.GroupVersionResource{Group: "example.test", Version: "v1", Resource: "things"},
		"Things",
		false,
	)
	screen.trackedTypes = []*k8s.TrackedType{k8s.PodResource, secondType}
	screen.table.SetRows([]table.Row{{"pod", "default", "Running", "1m"}})

	// Moving left from the first type wraps to the last type. The table must
	// not render old four-column rows while the new three-column schema is set.
	screen.nextResourceType(-1)

	if screen.currentType != 1 {
		t.Fatalf("current type = %d, want 1", screen.currentType)
	}
	if got := len(screen.table.Rows()); got != 0 {
		t.Fatalf("table rows = %d, want 0", got)
	}
	if got := len(screen.table.Columns()); got != len(secondType.Columns) {
		t.Fatalf("table columns = %d, want %d", got, len(secondType.Columns))
	}
}
