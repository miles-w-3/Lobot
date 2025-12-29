package splash

import (
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Phase int

const (
	PhaseDrawing Phase = iota
	PhaseShowingX
	PhaseClearing
	PhaseWaiting // Waiting for connection
	PhaseError   // Connection error occurred
	PhaseDone
)

type TickMsg struct{}

type Model struct {
	sprite       [][]rune
	width        int
	height       int
	step         int
	phase        Phase
	termWidth    int
	termHeight   int
	readyToExit  bool
	errorMessage string // Error message to display in error state
	logger       *slog.Logger
}

func NewModel(logger *slog.Logger) Model {
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
		logger: logger,
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
				m.logger.Debug("Done with clearing", "rte", m.readyToExit)
				// Animation complete - check if we're ready to exit
				if m.readyToExit {
					m.phase = PhaseDone
				} else {
					m.phase = PhaseWaiting
					// tick again so we can handle any waiting logic
					return m, tick()
				}
				m.logger.Debug("Phase is now", "phase", m.phase)
				// Stop ticking once we reach waiting/done phase - face is now cleared
				return m, nil
			}
			return m, tick()
		case PhaseWaiting:
			m.logger.Debug("Errm", "msg", m.errorMessage)
			if m.errorMessage != "" {
				m.phase = PhaseError
				m.logger.Debug("Switching to error phase now that we are in waiting")
			}
			// Stay in waiting phase until external signal
			return m, nil
		case PhaseError:
			m.logger.Debug("In error phase update")
			// Stay in error phase until user takes action
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

// MarkError marks the splash as having an error
func (m *Model) MarkError(err error) {
	m.errorMessage = err.Error()
	// If we're in waiting phase, transition to error immediately
	if m.phase == PhaseWaiting {
		m.phase = PhaseError
	} else {
		m.logger.Debug("We got an error, but we're not in waiting phase (lol)", "phase", m.phase)
	}
	m.logger.Debug("Msg is now", "msg", m.errorMessage)
}

// IsError returns true if splash is in error state
func (m Model) IsError() bool {
	return m.phase == PhaseError
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

	// Add status message based on phase
	switch m.phase {
	case PhaseWaiting:
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Italic(true).
			MarginTop(2)
		out += "\n\n" + statusStyle.Render("Connecting to Kubernetes cluster...")
	case PhaseError:
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true).
			MarginTop(2)

		instructionStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			MarginTop(1)

		out += "\n\n" + errorStyle.Render("⚠ Connection Failed")
		//out += "\n\n" + instructionStyle.Render(m.errorMessage)
		out += "\n\n" + instructionStyle.Render("Press 'c' to switch context | Press 'q' to quit")
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
