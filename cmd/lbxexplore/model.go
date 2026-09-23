package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/lbx"
	"github.com/domdom82/moo2hd/internal/svg"
)

type appMode int

const (
	modeList    appMode = iota
	modeExtract         // extract-dialog overlay is active
	modeConvert         // convert-to-SVG dialog overlay is active
)

// soundDoneMsg is sent when the audio subprocess finishes.
type soundDoneMsg struct{ err error }

// animTickMsg is sent on each animation frame advance.
type animTickMsg struct{}

// AppModel is the root bubbletea model.
type AppModel struct {
	filename string
	archive  *lbx.Archive
	lbxName  string          // uppercase basename of the open LBX, e.g. "SHIPS.LBX"
	assetsDir string
	reg      *config.Registry // may be nil
	palette  color.Palette    // fallback palette; may be nil

	list  list.Model
	input textinput.Model
	mode  appMode

	width, height int
	previewCache  map[int]string

	playCmd      *exec.Cmd
	playTempFile string
	statusMsg    string

	// animation state
	animFrames  []image.Image
	animFrame   int
	animPlaying bool
	animDelay   time.Duration
	animRecIdx  int // record index the current animation belongs to
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

func NewAppModel(filename string, arc *lbx.Archive, lbxName, assetsDir string, reg *config.Registry, palette color.Palette) AppModel {
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
		lbxName:      lbxName,
		assetsDir:    assetsDir,
		reg:          reg,
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

	case animTickMsg:
		if m.animPlaying && len(m.animFrames) > 0 {
			m.animFrame = (m.animFrame + 1) % len(m.animFrames)
			return m, animTickCmd(m.animDelay)
		}
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeExtract {
			return m.updateExtract(msg)
		}
		if m.mode == modeConvert {
			return m.updateConvert(msg)
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
		// Sprite animation takes priority over audio for multi-frame sprites.
		if r.Type == lbx.RecordLBX {
			hdr, err := lbx.ParseSpriteHeader(r.Data)
			if err == nil && hdr.FrameCount > 1 {
				return m.toggleAnimation(r)
			}
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

	case "c":
		r := m.selectedRecord()
		if r == nil {
			return m, nil
		}
		if r.Type != lbx.RecordLBX {
			m.statusMsg = "not a sprite record"
			return m, nil
		}
		if _, err := lbx.ParseSpriteHeader(r.Data); err != nil {
			m.statusMsg = "not a sprite record"
			return m, nil
		}
		m.mode = modeConvert
		m.input.SetValue(fmt.Sprintf("%03d.svg", r.Index))
		m.input.CursorEnd()
		return m, m.input.Focus()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m AppModel) updateConvert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			dest = fmt.Sprintf("%03d.svg", r.Index)
		}
		pal := m.paletteForRecord(r.Index)
		if err := convertToSVG(r, pal, dest); err != nil {
			m.statusMsg = fmt.Sprintf("convert error: %v", err)
		} else {
			m.statusMsg = fmt.Sprintf("converted → %s", dest)
		}
		m.mode = modeList
		m.input.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// convertToSVG decodes all frames of r and writes each as a separate SVG file.
// For a single-frame sprite the output is <dest>; for multi-frame sprites the
// frame index is inserted before the extension: <stem>_frame000.svg, etc.
func convertToSVG(r *lbx.Record, pal color.Palette, dest string) error {
	frames, err := lbx.DecodeFrames(r.Data, pal)
	if err != nil {
		return fmt.Errorf("decoding frames: %w", err)
	}

	upscaler := svg.XBRZUpscaler{}
	ext := ".svg"
	stem := strings.TrimSuffix(dest, ext)
	if !strings.HasSuffix(strings.ToLower(dest), ext) {
		stem = dest
	}

	for i, frame := range frames {
		doc, err := upscaler.Upscale(frame, nil)
		if err != nil {
			return fmt.Errorf("upscaling frame %d: %w", i, err)
		}

		var outPath string
		if len(frames) == 1 {
			outPath = stem + ext
		} else {
			outPath = fmt.Sprintf("%s_frame%03d%s", stem, i, ext)
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating %s: %w", outPath, err)
		}
		writeErr := doc.Write(f)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
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
		pal := m.paletteForRecord(r.Index)
		animFrame := -1
		if m.animPlaying && m.animRecIdx == r.Index {
			animFrame = m.animFrame
		}
		previewStr = renderPreview(r, pal, rightW, contentH, m.previewCache, m.animFrames, animFrame)
	}
	rightStyle := lipgloss.NewStyle().Width(rightW).Height(contentH).
		BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).PaddingLeft(1)
	right := rightStyle.Render(previewStr)

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	// Help bar.
	help := "↑↓ navigate   space play/animate   x extract   c convert SVG   esc/ctrl+c quit"
	if m.statusMsg != "" {
		help = m.statusMsg + "   |   " + help
	}
	helpStyle := lipgloss.NewStyle().Faint(true).Width(m.width)
	helpBar := helpStyle.Render(help)

	ui := lipgloss.JoinVertical(lipgloss.Left, body, helpBar)

	// Extract / convert dialog overlay.
	if m.mode == modeExtract {
		dialog := renderExtractDialog(m.input, m.width, m.height)
		ui = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog,
			lipgloss.WithWhitespaceChars(" "))
		_ = body
	} else if m.mode == modeConvert {
		dialog := renderConvertDialog(m.input, m.width, m.height)
		ui = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog,
			lipgloss.WithWhitespaceChars(" "))
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

func renderConvertDialog(ti textinput.Model, w, h int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Width(50)

	content := "Convert sprite to SVG:\n\n" + ti.View() + "\n\nEnter to confirm   Esc to cancel"
	return boxStyle.Render(content)
}

func (m *AppModel) paletteForRecord(recordIdx int) color.Palette {
	if m.reg != nil {
		refs := m.reg.LBXPaletteFor(m.lbxName, recordIdx)
		if refs != nil {
			if pal, err := lbx.BuildPalette(m.assetsDir, refs); err == nil {
				return pal
			}
		}
	}
	return m.palette
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

func (m AppModel) toggleAnimation(r *lbx.Record) (tea.Model, tea.Cmd) {
	// Stop animation if already playing this record.
	if m.animPlaying && m.animRecIdx == r.Index {
		m.animPlaying = false
		m.animFrames = nil
		m.animFrame = 0
		m.statusMsg = "animation stopped"
		return m, nil
	}

	pal := m.paletteForRecord(r.Index)
	frames, err := lbx.DecodeFrames(r.Data, pal)
	if err != nil {
		m.statusMsg = fmt.Sprintf("decode error: %v", err)
		return m, nil
	}
	hdr, _ := lbx.ParseSpriteHeader(r.Data)

	// FrameDelay is in game ticks (~1/24 s each); clamp to a sane range.
	delay := time.Duration(hdr.FrameDelay) * (time.Second / 24)
	if delay < 50*time.Millisecond {
		delay = 100 * time.Millisecond
	}

	m.animFrames = frames
	m.animFrame = 0
	m.animPlaying = true
	m.animDelay = delay
	m.animRecIdx = r.Index
	m.statusMsg = fmt.Sprintf("animating %d frames @ %v/frame", len(frames), delay)
	return m, animTickCmd(delay)
}

func animTickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return animTickMsg{} })
}

// waitForSound returns a tea.Cmd that waits for the audio process to exit.
func waitForSound(cmd *exec.Cmd) tea.Cmd {
	return func() tea.Msg {
		err := cmd.Wait()
		return soundDoneMsg{err}
	}
}
