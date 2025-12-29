package ui

import (
	"strings"
)

// Canvas represents a 2D character grid for drawing lines and shapes
type Canvas struct {
	width  int
	height int
	grid   [][]rune
}

// NewCanvas creates a new canvas with the specified dimensions
func NewCanvas(width, height int) *Canvas {
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = make([]rune, width)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}
	return &Canvas{
		width:  width,
		height: height,
		grid:   grid,
	}
}

// Set sets a character at the specified position
func (c *Canvas) Set(x, y int, ch rune) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		c.grid[y][x] = ch
	}
}

// Get gets a character at the specified position
func (c *Canvas) Get(x, y int) rune {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		return c.grid[y][x]
	}
	return ' '
}

// Row returns the raw rune slice for a specific row
// Use this for read-only access to the grid for performance
func (c *Canvas) Row(y int) []rune {
	if y >= 0 && y < c.height {
		return c.grid[y]
	}
	return nil
}

// DrawEdge draws a connection line from one position to another
func (c *Canvas) DrawEdge(from, to Position) {
	fromCenterX := from.X + from.Width/2
	fromBottomY := from.Y + from.Height
	toCenterX := to.X + to.Width/2
	toTopY := to.Y

	// Case 1: Vertically aligned (straight line)
	if fromCenterX == toCenterX {
		for y := fromBottomY; y < toTopY; y++ {
			c.Set(fromCenterX, y, '│')
		}
		if toTopY > 0 {
			c.Set(toCenterX, toTopY-1, '▼')
		}
		return
	}

	// Case 2: Offset horizontally (L-shaped line)
	midY := fromBottomY + (toTopY-fromBottomY)/2

	// Vertical segment from parent
	for y := fromBottomY; y < midY; y++ {
		c.Set(fromCenterX, y, '│')
	}

	// Horizontal segment
	startX := min(fromCenterX, toCenterX)
	endX := max(fromCenterX, toCenterX)
	for x := startX; x <= endX; x++ {
		existing := c.Get(x, midY)
		if x == fromCenterX || x == toCenterX {
			// Junction point
			if existing == '─' {
				c.Set(x, midY, '┼')
			} else if existing == '│' {
				c.Set(x, midY, '┼')
			} else {
				c.Set(x, midY, '┼')
			}
		} else {
			// Horizontal line
			if existing == '│' || existing == '┼' {
				c.Set(x, midY, '┼')
			} else {
				c.Set(x, midY, '─')
			}
		}
	}

	// Vertical segment to child
	for y := midY + 1; y < toTopY; y++ {
		c.Set(toCenterX, y, '│')
	}
	if toTopY > 0 {
		c.Set(toCenterX, toTopY-1, '▼')
	}
}

// String converts the canvas to a string representation
func (c *Canvas) String() string {
	var b strings.Builder
	for _, row := range c.grid {
		b.WriteString(string(row))
		b.WriteRune('\n')
	}
	return b.String()
}

// Lines returns the canvas as an array of strings (one per line)
func (c *Canvas) Lines() []string {
	lines := make([]string, len(c.grid))
	for i, row := range c.grid {
		lines[i] = string(row)
	}
	return lines
}

// ReplaceRegion replaces a rectangular region with the given content
// This is used to overlay Lipgloss-rendered boxes on top of the canvas
func (c *Canvas) ReplaceRegion(x, y int, content string) {
	lines := strings.Split(content, "\n")
	for dy, line := range lines {
		if y+dy >= c.height {
			break
		}
		// Convert line to runes to handle multi-byte characters properly
		runes := []rune(line)
		for dx, ch := range runes {
			if x+dx >= c.width {
				break
			}
			c.Set(x+dx, y+dy, ch)
		}
	}
}
