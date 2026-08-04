package modes

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func overlayCenter(base, overlay string, width, height int) string {
	overlayLines := strings.Split(overlay, "\n")
	overlayWidth := 0
	for _, line := range overlayLines {
		overlayWidth = max(overlayWidth, ansi.StringWidth(line))
	}

	x := (width - overlayWidth) / 2
	y := (height - len(overlayLines)) / 2
	return overlayAt(base, overlay, x, y, height)
}

func overlayAt(base, overlay string, x, y, height int) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	overlayLines := strings.Split(overlay, "\n")
	for i, overlayLine := range overlayLines {
		lineIndex := y + i
		if lineIndex < 0 || lineIndex >= len(baseLines) {
			continue
		}
		baseLines[lineIndex] = overlayLineAt(baseLines[lineIndex], overlayLine, x)
	}

	return strings.Join(baseLines, "\n")
}

func overlayLineAt(base, overlay string, x int) string {
	if x < 0 {
		x = 0
	}

	baseWidth := ansi.StringWidth(base)
	overlayWidth := ansi.StringWidth(overlay)
	if x >= baseWidth {
		return base + strings.Repeat(" ", x-baseWidth) + overlay
	}

	before := ansi.Cut(base, 0, x)
	after := ansi.Cut(base, x+overlayWidth, baseWidth)
	return before + overlay + after
}
