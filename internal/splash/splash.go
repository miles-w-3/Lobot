package splash

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Phase int

const (
	PhaseDrawing Phase = iota
	PhaseShowingX
	PhaseClearing
	PhaseWaiting // New phase: waiting for connection
	PhaseDone
)

type TickMsg struct{}

type Model struct {
	sprite      [][]rune
	width       int
	height      int
	step        int
	phase       Phase
	termWidth   int
	termHeight  int
	readyToExit bool
}

func NewModel() Model {
	sprite := []string{
		"           GGGGGGGGGGG           ",
		"           G         G           ",
		"          XG         GX          ",
		"          XG         GX          ",
		"          XG         GX          ",
		"          XG         GX          ",
		"           G         G           ",
		"           GGGGGGGGGGG           ",
		"               GGG               ",
		"        GGGGGGGGGGGGGGGGG        ",
		"        G               G        ",
	}

	grid := make([][]rune, len(sprite))
	for i, line := range sprite {
		grid[i] = []rune(line)
	}

	return Model{
		sprite: grid,
		width:  len(grid[0]),
		height: len(grid),
		phase:  PhaseDrawing,
	}
}

func (m Model) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg { return TickMsg{} })
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		switch m.phase {
		case PhaseDrawing:
			m.step++
			if m.step >= m.width+m.height {
				m.phase = PhaseShowingX
			}
			return m, tick()
		case PhaseShowingX:
			m.phase = PhaseClearing
			return m, tick()
		case PhaseClearing:
			m.step--
			if m.step <= 0 {
				// Animation complete - check if we're ready to exit
				if m.readyToExit {
					m.phase = PhaseDone
				} else {
					m.phase = PhaseWaiting
				}
				// Stop ticking once we reach waiting/done phase - face is now cleared
				return m, nil
			}
			return m, tick()
		case PhaseWaiting:
			// Stay in waiting phase until external signal
			return m, nil
		case PhaseDone:
			return m, nil
		}
	}
	return m, nil
}

// MarkReady marks the splash as ready to exit
func (m *Model) MarkReady() {
	m.readyToExit = true
	// If we're already in waiting phase, transition to done immediately
	if m.phase == PhaseWaiting {
		m.phase = PhaseDone
	}
	// Otherwise, we'll transition to done when clearing completes
}

// IsDone returns true if the splash is complete
func (m Model) IsDone() bool {
	return m.phase == PhaseDone
}

func (m Model) View() string {
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#06bf88"))
	gray := lipgloss.NewStyle().Foreground(lipgloss.Color("#929296"))

	var out string

	// === Draw face (only during animation phases, not during waiting) ===
	if m.phase < PhaseWaiting {
		for y := 0; y < m.height; y++ {
			for x := 0; x < m.width; x++ {
				cell := m.sprite[y][x]
				switch cell {
				case 'G':
					if shouldShowG(m, x, y) {
						out += green.Render("██")
					} else {
						out += "  "
					}
				case 'X':
					if shouldShowX(m, x, y) {
						out += gray.Render("██")
					} else {
						out += "  "
					}
				default:
					out += "  "
				}
			}
			out += "\n"
		}
		out += "\n"
	}

	// === Add title when appropriate ===
	if true { //m.phase >= PhaseShowingX {
		title := `██╗      ██████╗ ██████╗  ██████╗ ████████╗
██║     ██╔═══██╗██╔══██╗██╔═══██╗╚══██╔══╝
██║     ██║   ██║██████╔╝██║   ██║   ██║
██║     ██║   ██║██╔══██╗██║   ██║   ██║
███████╗╚██████╔╝██████╔╝╚██████╔╝   ██║
╚══════╝ ╚═════╝ ╚═════╝  ╚═════╝    ╚═╝`
		out += green.Render(title)

		// Add "Connecting..." message during waiting phase
		if m.phase == PhaseWaiting {
			statusStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Italic(true).
				MarginTop(2)
			out += "\n\n" + statusStyle.Render("Connecting to Kubernetes cluster...")
		}
	}

	// Center the content horizontally and vertically in the terminal
	if m.termWidth > 0 && m.termHeight > 0 {
		return lipgloss.Place(
			m.termWidth,
			m.termHeight,
			lipgloss.Center,
			lipgloss.Center,
			out,
		)
	}
	return out
}

// SetSize sets the terminal dimensions for centering
func (m *Model) SetSize(width, height int) {
	m.termWidth = width
	m.termHeight = height
}

// Show G pixels along a diagonal
func shouldShowG(m Model, x, y int) bool {
	line := x + (m.height - 1 - y)
	switch m.phase {
	case PhaseDrawing:
		return line <= m.step
	case PhaseShowingX:
		return true
	case PhaseClearing:
		return line <= m.step
	default:
		return false
	}
}

// Show X pixels only after all Gs are drawn, and hide them during clear
func shouldShowX(m Model, x, y int) bool {
	switch m.phase {
	case PhaseShowingX:
		return true
	case PhaseClearing:
		// Hide Xs progressively along the same diagonal pattern
		line := x + (m.height - 1 - y)
		return line <= m.step
	default:
		return false
	}
}
