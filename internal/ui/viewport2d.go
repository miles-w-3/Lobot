package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Viewport2DConfig contains configuration options for Viewport2D
type Viewport2DConfig struct {
	// ShowScrollIndicators enables visual indicators when content extends beyond the viewport
	ShowScrollIndicators bool

	// IndicatorStyle is the style applied to scroll indicators (optional)
	IndicatorStyle lipgloss.Style
}

// DefaultViewport2DConfig returns a config with sensible defaults
func DefaultViewport2DConfig() Viewport2DConfig {
	return Viewport2DConfig{
		ShowScrollIndicators: true,
		IndicatorStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("8")), // Gray
	}
}

// Viewport2D is a viewport that supports both horizontal and vertical scrolling
// with proper ANSI escape sequence handling.
type Viewport2D struct {
	// Viewport dimensions (visible area)
	Width  int
	Height int

	// Scroll offsets
	XOffset int
	YOffset int

	// Configuration
	config Viewport2DConfig

	// Content storage
	lines        []string
	contentWidth int // max visual width of any line
}

// NewViewport2D creates a new 2D viewport with the given dimensions
func NewViewport2D(width, height int) *Viewport2D {
	return NewViewport2DWithConfig(width, height, DefaultViewport2DConfig())
}

// NewViewport2DWithConfig creates a new 2D viewport with custom configuration
func NewViewport2DWithConfig(width, height int, config Viewport2DConfig) *Viewport2D {
	return &Viewport2D{
		Width:  width,
		Height: height,
		config: config,
		lines:  make([]string, 0),
	}
}

// SetConfig updates the viewport configuration
func (v *Viewport2D) SetConfig(config Viewport2DConfig) {
	v.config = config
}

// SetContent sets the viewport content from a multi-line string
func (v *Viewport2D) SetContent(content string) {
	v.lines = strings.Split(content, "\n")
	v.recalculateContentWidth()
}

// SetLines sets the viewport content from a slice of lines
func (v *Viewport2D) SetLines(lines []string) {
	v.lines = lines
	v.recalculateContentWidth()
}

// recalculateContentWidth finds the maximum visual width of all lines
func (v *Viewport2D) recalculateContentWidth() {
	v.contentWidth = 0
	for _, line := range v.lines {
		w := ansi.StringWidth(line)
		if w > v.contentWidth {
			v.contentWidth = w
		}
	}
}

// ContentWidth returns the maximum visual width of the content
func (v *Viewport2D) ContentWidth() int {
	return v.contentWidth
}

// ContentHeight returns the number of lines in the content
func (v *Viewport2D) ContentHeight() int {
	return len(v.lines)
}

// MaxXOffset returns the maximum valid X offset
func (v *Viewport2D) MaxXOffset() int {
	maxOffset := v.contentWidth - v.innerWidth()
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

// MaxYOffset returns the maximum valid Y offset
func (v *Viewport2D) MaxYOffset() int {
	maxOffset := len(v.lines) - v.innerHeight()
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

// innerWidth returns the width available for content (accounting for indicators)
func (v *Viewport2D) innerWidth() int {
	if v.config.ShowScrollIndicators {
		return v.Width - 2 // Reserve 1 char on each side for ◀ ▶
	}
	return v.Width
}

// innerHeight returns the height available for content (accounting for indicators)
func (v *Viewport2D) innerHeight() int {
	if v.config.ShowScrollIndicators {
		return v.Height - 2 // Reserve 1 row on top and bottom for ▲ ▼
	}
	return v.Height
}

// Scroll position helpers

// AtLeft returns true if scrolled to the leftmost position
func (v *Viewport2D) AtLeft() bool {
	return v.XOffset <= 0
}

// AtRight returns true if scrolled to the rightmost position
func (v *Viewport2D) AtRight() bool {
	return v.XOffset >= v.MaxXOffset()
}

// AtTop returns true if scrolled to the topmost position
func (v *Viewport2D) AtTop() bool {
	return v.YOffset <= 0
}

// AtBottom returns true if scrolled to the bottommost position
func (v *Viewport2D) AtBottom() bool {
	return v.YOffset >= v.MaxYOffset()
}

// CanScrollLeft returns true if there's content to the left
func (v *Viewport2D) CanScrollLeft() bool {
	return !v.AtLeft()
}

// CanScrollRight returns true if there's content to the right
func (v *Viewport2D) CanScrollRight() bool {
	return !v.AtRight()
}

// CanScrollUp returns true if there's content above
func (v *Viewport2D) CanScrollUp() bool {
	return !v.AtTop()
}

// CanScrollDown returns true if there's content below
func (v *Viewport2D) CanScrollDown() bool {
	return !v.AtBottom()
}

// ScrollTo sets the scroll position, clamping to valid bounds
func (v *Viewport2D) ScrollTo(x, y int) {
	v.XOffset = clamp(x, 0, v.MaxXOffset())
	v.YOffset = clamp(y, 0, v.MaxYOffset())
}

// ScrollBy adjusts the scroll position by the given delta
func (v *Viewport2D) ScrollBy(dx, dy int) {
	v.ScrollTo(v.XOffset+dx, v.YOffset+dy)
}

// ScrollLeft scrolls left by n columns
func (v *Viewport2D) ScrollLeft(n int) {
	v.ScrollBy(-n, 0)
}

// ScrollRight scrolls right by n columns
func (v *Viewport2D) ScrollRight(n int) {
	v.ScrollBy(n, 0)
}

// ScrollUp scrolls up by n lines
func (v *Viewport2D) ScrollUp(n int) {
	v.ScrollBy(0, -n)
}

// ScrollDown scrolls down by n lines
func (v *Viewport2D) ScrollDown(n int) {
	v.ScrollBy(0, n)
}

// EnsureVisible scrolls the viewport to ensure the given rectangle is visible
// x, y are the top-left corner of the rectangle (in content coordinates)
// w, h are the dimensions of the rectangle
func (v *Viewport2D) EnsureVisible(x, y, w, h int) {
	innerW := v.innerWidth()
	innerH := v.innerHeight()

	// Adjust X offset
	if x < v.XOffset {
		v.XOffset = x
	} else if x+w > v.XOffset+innerW {
		v.XOffset = x + w - innerW
	}

	// Adjust Y offset
	if y < v.YOffset {
		v.YOffset = y
	} else if y+h > v.YOffset+innerH {
		v.YOffset = y + h - innerH
	}

	// Clamp to valid bounds
	v.ScrollTo(v.XOffset, v.YOffset)
}

// View renders the visible portion of the content
func (v *Viewport2D) View() string {
	if v.config.ShowScrollIndicators {
		return v.viewWithIndicators()
	}
	return v.viewPlain()
}

// viewPlain renders without scroll indicators
func (v *Viewport2D) viewPlain() string {
	if len(v.lines) == 0 {
		return strings.Repeat(strings.Repeat(" ", v.Width)+"\n", v.Height)
	}

	startLine := v.YOffset
	endLine := v.YOffset + v.Height
	if endLine > len(v.lines) {
		endLine = len(v.lines)
	}

	visibleLines := make([]string, 0, v.Height)

	for i := startLine; i < endLine; i++ {
		line := v.lines[i]
		sliced := sliceLineVisual(line, v.XOffset, v.Width)
		visibleLines = append(visibleLines, sliced)
	}

	for len(visibleLines) < v.Height {
		visibleLines = append(visibleLines, strings.Repeat(" ", v.Width))
	}

	return strings.Join(visibleLines, "\n")
}

// viewWithIndicators renders with scroll indicators on edges
func (v *Viewport2D) viewWithIndicators() string {
	innerW := v.innerWidth()
	innerH := v.innerHeight()

	style := v.config.IndicatorStyle

	// Indicator characters
	leftIndicator := style.Render("◀")
	rightIndicator := style.Render("▶")
	upIndicator := style.Render("▲")
	downIndicator := style.Render("▼")
	emptyH := " " // Empty horizontal indicator space
	// emptyV := " " // Empty vertical indicator space

	// Determine which indicators to show
	showLeft := v.CanScrollLeft()
	showRight := v.CanScrollRight()
	showUp := v.CanScrollUp()
	showDown := v.CanScrollDown()

	var result strings.Builder

	// Top row with up indicator
	topRow := strings.Repeat(" ", v.Width)
	if showUp {
		// Center the up indicator
		centerPos := v.Width / 2
		topRow = strings.Repeat(" ", centerPos) + upIndicator + strings.Repeat(" ", v.Width-centerPos-1)
	}
	result.WriteString(topRow)
	result.WriteString("\n")

	// Content rows with left/right indicators
	startLine := v.YOffset
	endLine := v.YOffset + innerH
	if endLine > len(v.lines) {
		endLine = len(v.lines)
	}

	for i := 0; i < innerH; i++ {
		lineIdx := startLine + i

		// Left indicator
		if showLeft {
			result.WriteString(leftIndicator)
		} else {
			result.WriteString(emptyH)
		}

		// Content
		if lineIdx < len(v.lines) {
			sliced := sliceLineVisual(v.lines[lineIdx], v.XOffset, innerW)
			result.WriteString(sliced)
		} else {
			result.WriteString(strings.Repeat(" ", innerW))
		}

		// Right indicator
		if showRight {
			result.WriteString(rightIndicator)
		} else {
			result.WriteString(emptyH)
		}

		result.WriteString("\n")
	}

	// Bottom row with down indicator
	bottomRow := strings.Repeat(" ", v.Width)
	if showDown {
		centerPos := v.Width / 2
		bottomRow = strings.Repeat(" ", centerPos) + downIndicator + strings.Repeat(" ", v.Width-centerPos-1)
	}
	result.WriteString(bottomRow)

	return result.String()
}

// sliceLineVisual extracts a horizontal slice of a line at visual column positions.
// Uses ansi.Cut for proper ANSI handling and pads to exact width.
func sliceLineVisual(line string, start, width int) string {
	lineWidth := ansi.StringWidth(line)

	// If start is beyond the line, return padded empty string
	if start >= lineWidth {
		return strings.Repeat(" ", width)
	}

	// Calculate end position
	end := start + width
	if end > lineWidth {
		end = lineWidth
	}

	// Use ansi.Cut - it handles ANSI state properly
	sliced := ansi.Cut(line, start, end)

	// Always add a reset to prevent any style bleeding to next line
	sliced = sliced + "\x1b[0m"

	// Measure and pad to exact width
	slicedWidth := ansi.StringWidth(sliced)
	if slicedWidth < width {
		sliced = sliced + strings.Repeat(" ", width-slicedWidth)
	}

	return sliced
}

// clamp constrains a value to the range [min, max]
func clamp(value, minVal, maxVal int) int {
	if value < minVal {
		return minVal
	}
	if value > maxVal {
		return maxVal
	}
	return value
}
