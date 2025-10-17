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

func longTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return TickMsg{} })
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		switch m.phase {
		case PhaseDrawing:
			m.step++
			// Draw bottom to top (step goes from 0 to height)
			if m.step >= m.height {
				m.phase = PhaseShowingX
			}
			return m, tick()
		case PhaseShowingX:
			// Reset step for clearing phase
			m.step = m.height
			m.phase = PhaseClearing
			return m, longTick() // Pause for effect before clearing
		case PhaseClearing:
			m.step--
			// Clear top to bottom (step goes from height down to 0)
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
		title := `__/\\\___________________/\\\\\_______/\\\\\\\\\\\\\_________/\\\\\_______/\\\\\\\\\\\\\\\_
 _\/\\\_________________/\\\///\\\____\/\\\/////////\\\_____/\\\///\\\____\///////\\\/////__
  _\/\\\_______________/\\\/__\///\\\__\/\\\_______\/\\\___/\\\/__\///\\\________\/\\\_______
   _\/\\\______________/\\\______\//\\\_\/\\\\\\\\\\\\\\___/\\\______\//\\\_______\/\\\_______
    _\/\\\_____________\/\\\_______\/\\\_\/\\\/////////\\\_\/\\\_______\/\\\_______\/\\\_______
     _\/\\\_____________\//\\\______/\\\__\/\\\_______\/\\\_\//\\\______/\\\________\/\\\_______
      _\/\\\______________\///\\\__/\\\____\/\\\_______\/\\\__\///\\\__/\\\__________\/\\\_______
       _\/\\\\\\\\\\\\\\\____\///\\\\\/_____\/\\\\\\\\\\\\\/_____\///\\\\\/___________\/\\\_______
        _\///////////////_______\/////_______\/////////////_________\/////_____________\///________`
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

// Show G pixels from bottom to top
func shouldShowG(m Model, x, y int) bool {
	switch m.phase {
	case PhaseDrawing:
		// Draw from bottom to top: reveal rows from bottom (higher y) to top (lower y)
		// step goes from 0 to height-1
		return y >= (m.height - m.step)
	case PhaseShowingX:
		return true
	case PhaseClearing:
		// Clear from top to bottom: hide rows from top (lower y) to bottom (higher y)
		// step goes from height-1 to 0
		return y >= (m.height - m.step)
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
		// Hide Xs from top to bottom, same as Gs
		return y >= (m.height - m.step)
	default:
		return false
	}
}
