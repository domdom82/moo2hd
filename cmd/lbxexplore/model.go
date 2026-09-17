package main

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/domdom82/moo2hd/internal/lbx"
)

type appMode int

const (
	modeList    appMode = iota
	modeExtract         // extract-dialog overlay is active
)

// soundDoneMsg is sent when the audio subprocess finishes.
type soundDoneMsg struct{ err error }

// AppModel is the root bubbletea model.
type AppModel struct {
	filename string
	archive  *lbx.Archive
	palette  color.Palette // may be nil

	list  list.Model
	input textinput.Model
	mode  appMode

	width, height int
	previewCache  map[int]string

	playCmd      *exec.Cmd
	playTempFile string
	statusMsg    string
}

// recordItem wraps *lbx.Record to satisfy bubbles/list.Item.
type recordItem struct{ r *lbx.Record }

func (ri recordItem) Title() string {
	return fmt.Sprintf("[%03d] %s", ri.r.Index, ri.r.Type)
}
func (ri recordItem) Description() string {
	name := ri.r.Name
	if name == "" {
		name = "—"
	}
	return fmt.Sprintf("%d bytes  %s", len(ri.r.Data), name)
}
func (ri recordItem) FilterValue() string { return ri.r.Name }

func NewAppModel(filename string, arc *lbx.Archive, palette color.Palette) AppModel {
	items := make([]list.Item, len(arc.Records))
	for i, r := range arc.Records {
		items[i] = recordItem{r}
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = filename
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	ti := textinput.New()
	ti.Placeholder = "filename"
	ti.CharLimit = 256

	return AppModel{
		filename:     filename,
		archive:      arc,
		palette:      palette,
		list:         l,
		input:        ti,
		mode:         modeList,
		previewCache: make(map[int]string),
	}
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		leftW := m.width / 3
		contentH := m.height - 1
		m.list.SetSize(leftW, contentH)
		// Invalidate preview cache on resize (image dimensions change).
		m.previewCache = make(map[int]string)
		return m, nil

	case soundDoneMsg:
		os.Remove(m.playTempFile)
		m.playTempFile = ""
		m.playCmd = nil
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("playback error: %v", msg.err)
		} else {
			m.statusMsg = "playback finished"
		}
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeExtract {
			return m.updateExtract(msg)
		}
		return m.updateList(msg)
	}

	if m.mode == modeList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m AppModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.stopPlayback()
		return m, tea.Quit

	case " ":
		r := m.selectedRecord()
		if r == nil {
			return m, nil
		}
		if r.Type != lbx.RecordVOC && r.Type != lbx.RecordWAV && r.Type != lbx.RecordXMI {
			m.statusMsg = "not an audio record"
			return m, nil
		}
		m.stopPlayback()
		cmd, tmp, err := playSound(r)
		if err != nil {
			m.statusMsg = fmt.Sprintf("play error: %v", err)
			return m, nil
		}
		m.playCmd = cmd
		m.playTempFile = tmp
		m.statusMsg = "playing…"
		return m, waitForSound(cmd)

	case "x":
		r := m.selectedRecord()
		if r == nil {
			return m, nil
		}
		m.mode = modeExtract
		m.input.SetValue(fmt.Sprintf("%03d.%s", r.Index, strings.ToLower(r.Type.String())))
		m.input.CursorEnd()
		return m, m.input.Focus()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m AppModel) updateExtract(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		m.input.Blur()
		return m, nil

	case "enter":
		r := m.selectedRecord()
		if r == nil {
			m.mode = modeList
			return m, nil
		}
		dest := m.input.Value()
		if dest == "" {
			dest = fmt.Sprintf("%03d.%s", r.Index, strings.ToLower(r.Type.String()))
		}
		if err := os.WriteFile(dest, r.Data, 0o644); err != nil {
			m.statusMsg = fmt.Sprintf("extract error: %v", err)
		} else {
			m.statusMsg = fmt.Sprintf("extracted → %s", dest)
		}
		m.mode = modeList
		m.input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m AppModel) View() string {
	if m.width == 0 {
		return "loading…"
	}

	leftW := m.width / 3
	rightW := m.width - leftW - 1
	contentH := m.height - 1

	// Left pane: record list.
	leftStyle := lipgloss.NewStyle().Width(leftW).Height(contentH)
	left := leftStyle.Render(m.list.View())

	// Right pane: preview.
	r := m.selectedRecord()
	var previewStr string
	if r != nil {
		previewStr = renderPreview(r, m.palette, rightW, contentH, m.previewCache)
	}
	rightStyle := lipgloss.NewStyle().Width(rightW).Height(contentH).
		BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).PaddingLeft(1)
	right := rightStyle.Render(previewStr)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	// Help bar.
	help := "↑↓ navigate   space play   x extract   esc/ctrl+c quit"
	if m.statusMsg != "" {
		help = m.statusMsg + "   |   " + help
	}
	helpStyle := lipgloss.NewStyle().Faint(true).Width(m.width)
	helpBar := helpStyle.Render(help)

	ui := lipgloss.JoinVertical(lipgloss.Left, body, helpBar)

	// Extract dialog overlay.
	if m.mode == modeExtract {
		dialog := renderExtractDialog(m.input, m.width, m.height)
		ui = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog,
			lipgloss.WithWhitespaceChars(" "))
		// Re-draw body behind dialog — just overlay on top.
		_ = body
	}

	return ui
}

func renderExtractDialog(ti textinput.Model, w, h int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(50)

	content := "Extract record to file:\n\n" + ti.View() + "\n\nEnter to confirm   Esc to cancel"
	return boxStyle.Render(content)
}

func (m *AppModel) selectedRecord() *lbx.Record {
	item, ok := m.list.SelectedItem().(recordItem)
	if !ok {
		return nil
	}
	return item.r
}

func (m *AppModel) stopPlayback() {
	if m.playCmd != nil {
		m.playCmd.Process.Kill()
		m.playCmd = nil
	}
	if m.playTempFile != "" {
		os.Remove(m.playTempFile)
		m.playTempFile = ""
	}
}

// waitForSound returns a tea.Cmd that waits for the audio process to exit.
func waitForSound(cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		err := cmd.Wait()
		return soundDoneMsg{err}
	}
}
