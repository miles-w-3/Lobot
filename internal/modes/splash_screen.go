package modes

import (
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/miles-w-3/lobot/internal/command"
	"github.com/miles-w-3/lobot/internal/util"
)

type splashPhase uint8

const (
	phaseDrawing splashPhase = iota
	phaseShowingX
	phaseClearing
	phaseWaiting
	phaseError
	phaseDone
)

type splashTickMsg struct{}

// SplashScreen displays the startup animation while the application initializes.
type SplashScreen struct {
	sprite       [][]rune
	width        int
	height       int
	step         int
	phase        splashPhase
	termWidth    int
	termHeight   int
	readyToExit  bool
	errorMessage string
	log          *slog.Logger
}

var _ Screen = (*SplashScreen)(nil)

func NewSplashScreen(log *slog.Logger) *SplashScreen {
	if log == nil {
		log = slog.Default()
	}

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

	return &SplashScreen{
		sprite: grid,
		width:  len(grid[0]),
		height: len(grid),
		phase:  phaseDrawing,
		log:    log.With("component", "splash"),
	}
}

// start returns the first animation command. RootModel owns when it is
// scheduled; this is intentionally not part of the Screen contract.
func (s *SplashScreen) start() tea.Cmd {
	return splashTick()
}

func splashTick() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg {
		return splashTickMsg{}
	})
}

func splashLongTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return splashTickMsg{}
	})
}

func (s *SplashScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.termWidth = msg.Width
		s.termHeight = msg.Height
		return nil
	case SplashServiceReadyMsg:
		s.MarkReady()
		if s.IsDone() {
			return navigateTo(ScreenHome)
		}
		return nil

	case ErrorMsg:
		s.MarkError(msg.Error)
		return nil

	case splashTickMsg:
		// Continue below.

	default:
		return nil
	}

	switch s.phase {
	case phaseDrawing:
		s.step++
		if s.step >= s.height {
			s.phase = phaseShowingX
		}
		return splashTick()

	case phaseShowingX:
		s.step = s.height
		s.phase = phaseClearing
		return splashLongTick()

	case phaseClearing:
		s.step--
		if s.step > 0 {
			return splashTick()
		}

		if s.readyToExit {
			s.phase = phaseDone
			return navigateTo(ScreenHome)
		}

		s.phase = phaseWaiting
		return splashTick()

	case phaseWaiting:
		if s.errorMessage != "" {
			s.phase = phaseError
		}
		return nil

	case phaseError, phaseDone:
		return nil
	}

	return nil
}

// CommandPresentation exposes splash command metadata to the root view.
func (s *SplashScreen) CommandPresentation() command.Presentation {
	return command.Presentation{}
}

// MarkReady allows the animation to finish and leave the splash screen.
func (s *SplashScreen) MarkReady() {
	s.readyToExit = true
	if s.phase == phaseWaiting {
		s.phase = phaseDone
	}
}

// MarkError puts the splash screen into its connection-error state.
func (s *SplashScreen) MarkError(err error) {
	if err == nil {
		return
	}

	s.errorMessage = err.Error()
	if s.phase == phaseWaiting {
		s.phase = phaseError
		return
	}

	s.log.Debug("received splash error before waiting phase", "phase", s.phase)
}

func (s *SplashScreen) IsError() bool {
	return s.phase == phaseError
}

func (s *SplashScreen) IsDone() bool {
	return s.phase == phaseDone
}

func (s *SplashScreen) View() string {
	return s.render()
}

func (s *SplashScreen) render() string {
	green := lipgloss.NewStyle().Foreground(util.ColorPrimary)
	gray := lipgloss.NewStyle().Foreground(util.ColorMuted)

	var out strings.Builder
	out.Grow(s.width*s.height*2 + 512)

	if s.phase < phaseWaiting {
		for y := 0; y < s.height; y++ {
			for x := 0; x < s.width; x++ {
				switch s.sprite[y][x] {
				case 'G':
					if s.shouldShowG(y) {
						out.WriteString(green.Render("██"))
					} else {
						out.WriteString("  ")
					}
				case 'X':
					if s.shouldShowX(y) {
						out.WriteString(gray.Render("██"))
					} else {
						out.WriteString("  ")
					}
				default:
					out.WriteString("  ")
				}
			}
			out.WriteByte('\n')
		}
		out.WriteByte('\n')
	}

	out.WriteString(green.Render(splashTitle))

	switch s.phase {
	case phaseWaiting:
		statusStyle := lipgloss.NewStyle().
			Foreground(util.ColorSecondary).
			Italic(true).
			MarginTop(2)
		out.WriteString("\n\n")
		out.WriteString(statusStyle.Render("Connecting to Kubernetes cluster..."))
	case phaseError:
		errorStyle := lipgloss.NewStyle().
			Foreground(util.ColorDanger).
			Bold(true).
			MarginTop(2)
		out.WriteString("\n\n")
		out.WriteString(errorStyle.Render("⚠ Connection Failed"))
	}

	content := out.String()
	if s.termWidth > 0 && s.termHeight > 0 {
		return lipgloss.Place(
			s.termWidth,
			s.termHeight,
			lipgloss.Center,
			lipgloss.Center,
			content,
		)
	}
	return content
}

func (s *SplashScreen) shouldShowG(y int) bool {
	switch s.phase {
	case phaseDrawing:
		return y >= s.height-s.step
	case phaseShowingX:
		return true
	case phaseClearing:
		return y >= s.height-s.step
	default:
		return false
	}
}

func (s *SplashScreen) shouldShowX(y int) bool {
	switch s.phase {
	case phaseShowingX:
		return true
	case phaseClearing:
		return y >= s.height-s.step
	default:
		return false
	}
}

const splashTitle = `__/\\\___________________/\\\\\_______/\\\\\\\\\\\\\_________/\\\\\_______/\\\\\\\\\\\\\\\_
 _\/\\\_________________/\\\///\\\____\/\\\/////////\\\_____/\\\///\\\____\///////\\\/////__
  _\/\\\_______________/\\\/__\///\\\__\/\\\_______\/\\\___/\\\/__\///\\\________\/\\\_______
   _\/\\\______________/\\\______\//\\\_\/\\\\\\\\\\\\\\___/\\\______\//\\\_______\/\\\_______
    _\/\\\_____________\/\\\_______\/\\\_\/\\\/////////\\\_\/\\\_______\/\\\_______\/\\\_______
     _\/\\\_____________\//\\\______/\\\__\/\\\_______\/\\\_\//\\\______/\\\________\/\\\_______
      _\/\\\______________\///\\\__/\\\____\/\\\_______\/\\\__\///\\\__/\\\__________\/\\\_______
       _\/\\\\\\\\\\\\\\\____\///\\\\\/_____\/\\\\\\\\\\\\\/_____\///\\\\\/___________\/\\\_______
        _\///////////////_______\/////_______\/////////////_________\/////_____________\///________`
