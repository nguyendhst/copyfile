package filepicker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

var (
	lastID int
	idMtx  sync.Mutex
	subtle = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
)

// Return the next ID we should use on the Model.
func nextID() int {
	idMtx.Lock()
	defer idMtx.Unlock()
	lastID++
	return lastID
}

// New returns a new filepicker model with default styling and key bindings.
func New() Model {
	return Model{
		id:               nextID(),
		CurrentDirectory: ".",
		Cursor:           ">>",
		AllowedTypes:     []string{},
		selected:         0,
		ShowHidden:       false,
		DirAllowed:       false,
		FileAllowed:      true,
		AutoHeight:       true,
		Height:           0,
		max:              0,
		min:              0,
		selectedStack:    newStack(),
		minStack:         newStack(),
		maxStack:         newStack(),
		KeyMap:           DefaultKeyMap,
		Styles:           DefaultStyles,
	}
}

func NewWithConfig(height, width int, path string) Model {
	return Model{
		id:               nextID(),
		CurrentDirectory: path,
		PathUI:           path,
		Cursor:           ">>",
		AllowedTypes:     []string{},
		selected:         0,
		ShowHidden:       false,
		DirAllowed:       false,
		FileAllowed:      true,
		AutoHeight:       false,
		Height:           height,
		Width:            width,
		max:              0,
		min:              0,
		selectedStack:    newStack(),
		minStack:         newStack(),
		maxStack:         newStack(),
		KeyMap:           DefaultKeyMap,
		Styles:           DefaultStyles,
	}
}

type errorMsg struct {
	err error
}

type readDirMsg []os.DirEntry

const (
	marginBottom  = 5
	fileSizeWidth = 8
	paddingLeft   = 2
)

// KeyMap defines key bindings for each user action.
type KeyMap struct {
	GoToTop  key.Binding
	GoToLast key.Binding
	Down     key.Binding
	Up       key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Back     key.Binding
	Open     key.Binding
	Select   key.Binding
	Quit     key.Binding
}

// DefaultKeyMap defines the default keybindings.
var DefaultKeyMap = KeyMap{
	GoToTop:  key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "first")),
	GoToLast: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "last")),
	Down:     key.NewBinding(key.WithKeys("j", "down", "ctrl+n"), key.WithHelp("j", "down")),
	Up:       key.NewBinding(key.WithKeys("k", "up", "ctrl+p"), key.WithHelp("k", "up")),
	PageUp:   key.NewBinding(key.WithKeys("K", "pgup"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("J", "pgdown"), key.WithHelp("pgdown", "page down")),
	Back:     key.NewBinding(key.WithKeys("h", "backspace", "left", "esc"), key.WithHelp("h", "back")),
	Open:     key.NewBinding(key.WithKeys("l", "right", "enter"), key.WithHelp("l", "open")),
	Select:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// Styles defines the possible customizations for styles in the file picker.
type Styles struct {
	DisabledCursor   lipgloss.Style
	Cursor           lipgloss.Style
	Symlink          lipgloss.Style
	Directory        lipgloss.Style
	File             lipgloss.Style
	DisabledFile     lipgloss.Style
	Permission       lipgloss.Style
	Selected         lipgloss.Style
	DisabledSelected lipgloss.Style
	FileSize         lipgloss.Style
	EmptyDirectory   lipgloss.Style
	MainPath         lipgloss.Style
	MainBox          lipgloss.Style
}

// DefaultStyles defines the default styling for the file picker.
var DefaultStyles = Styles{
	DisabledCursor:   lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
	Cursor:           lipgloss.NewStyle().Foreground(lipgloss.Color("212")),
	Symlink:          lipgloss.NewStyle().Foreground(lipgloss.Color("36")),
	Directory:        lipgloss.NewStyle().Foreground(lipgloss.Color("99")),
	File:             lipgloss.NewStyle(),
	DisabledFile:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
	DisabledSelected: lipgloss.NewStyle().Foreground(lipgloss.Color("247")),
	Permission:       lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
	Selected:         lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
	FileSize:         lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(fileSizeWidth).Align(lipgloss.Right),
	EmptyDirectory:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")).PaddingLeft(paddingLeft).SetString("Bummer. No Files Found."),
	MainPath:         lipgloss.NewStyle().Foreground(lipgloss.Color("240")).PaddingLeft(paddingLeft),
	MainBox: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		BorderTop(true).
		BorderLeft(true).
		BorderRight(true).
		BorderBottom(true),
}

// Model represents a file picker.
type Model struct {
	id int

	// Path is the path which the user has selected with the file picker.
	Path string
	// PathUI is the path currently displayed in the file picker.
	PathUI string

	// CurrentDirectory is the directory that the user is currently in.
	CurrentDirectory string

	// AllowedTypes specifies which file types the user may select.
	// If empty the user may select any file.
	AllowedTypes []string

	KeyMap      KeyMap
	files       []os.DirEntry
	ShowHidden  bool
	DirAllowed  bool
	FileAllowed bool

	FileSelected  string
	selected      int
	selectedStack stack

	min      int
	max      int
	maxStack stack
	minStack stack

	Height     int
	AutoHeight bool
	Width      int

	Cursor string
	Styles Styles
}

type stack struct {
	Push   func(int)
	Pop    func() int
	Length func() int
}

func newStack() stack {
	slice := make([]int, 0)
	return stack{
		Push: func(i int) {
			slice = append(slice, i)
		},
		Pop: func() int {
			res := slice[len(slice)-1]
			slice = slice[:len(slice)-1]
			return res
		},
		Length: func() int {
			return len(slice)
		},
	}
}

func (m Model) pushView() {
	m.minStack.Push(m.min)
	m.maxStack.Push(m.max)
	m.selectedStack.Push(m.selected)
}

func (m Model) popView() (int, int, int) {
	return m.selectedStack.Pop(), m.minStack.Pop(), m.maxStack.Pop()
}

func readDir(path string, showHidden bool) tea.Cmd {
	return func() tea.Msg {
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			return errorMsg{err}
		}

		// sort directories alphabetically
		sort.Slice(dirEntries, func(i, j int) bool {
			if dirEntries[i].IsDir() == dirEntries[j].IsDir() {
				return dirEntries[i].Name() < dirEntries[j].Name()
			}
			return dirEntries[i].IsDir()
		})

		// if hidden files are allowed, return the dirEntries as is
		if showHidden {
			return readDirMsg(dirEntries)
		}
		// otherwise, filter out hidden files
		var sanitizedDirEntries []os.DirEntry
		for _, dirEntry := range dirEntries {
			isHidden, _ := IsHidden(dirEntry.Name())
			if isHidden {
				continue
			}
			sanitizedDirEntries = append(sanitizedDirEntries, dirEntry)
		}
		return readDirMsg(sanitizedDirEntries)
	}
}

// Init initializes the file picker model.
func (m Model) Init() tea.Cmd {
	return readDir(m.CurrentDirectory, m.ShowHidden)
}

// Update handles user interactions within the file picker model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	case readDirMsg: // If msg in readDirMsg, update the files in the current directory.
		m.files = msg
		m.max = m.Height - 1
	case tea.WindowSizeMsg: // If msg is a WindowSizeMsg, update the height of the file picker.
		if m.AutoHeight {
			m.Height = msg.Height - marginBottom
		}
		m.max = m.Height - 1
		//m.Width = msg.Width // TODO: this line somehow breaks the filepicker

	case tea.KeyMsg: // If msg is a KeyMsg, handle the key press.
		switch {
		case key.Matches(msg, m.KeyMap.GoToTop): // If the msg matches the GoToTop keymap, go to the top of the file list.

			m.selected = 0       // Set the selected file to the first file.
			m.min = 0            // Set the min to 0.
			m.max = m.Height - 1 // Set the max to the height of the file picker.

		case key.Matches(msg, m.KeyMap.GoToLast): // If the msg matches the GoToLast keymap, go to the last file in the list.

			m.selected = len(m.files) - 1   // Set the selected file to the last file.
			m.min = len(m.files) - m.Height // Set the min to the length of the files minus the height of the file picker.
			m.max = len(m.files) - 1        // Set the max to the length of the files minus 1.

		case key.Matches(msg, m.KeyMap.Down): // If the msg matches the Down keymap, go down one file.

			m.selected++
			if m.selected >= len(m.files) {
				m.selected = len(m.files) - 1
			}
			if m.selected > m.max {
				m.min++
				m.max++
			}
			f := m.files[m.selected]
			_, err := f.Info()
			if err != nil {
				break
			}
			isDir := f.IsDir()

			if isDir {
				m.PathUI = filepath.Join(m.CurrentDirectory)
			} else {

				if m.FileAllowed {
					m.PathUI = filepath.Join(m.CurrentDirectory, f.Name())
				}
			}

		case key.Matches(msg, m.KeyMap.Up):

			m.selected--
			if m.selected < 0 {
				m.selected = 0
			}
			if m.selected < m.min {
				m.min--
				m.max--
			}

			f := m.files[m.selected]
			_, err := f.Info()
			if err != nil {
				break
			}
			isDir := f.IsDir()

			if isDir {
				m.PathUI = filepath.Join(m.CurrentDirectory)
			} else {

				if m.FileAllowed {
					m.PathUI = filepath.Join(m.CurrentDirectory, f.Name())
				}
			}

		case key.Matches(msg, m.KeyMap.PageDown):

			m.selected += m.Height
			if m.selected >= len(m.files) {
				m.selected = len(m.files) - 1
			}
			m.min += m.Height
			m.max += m.Height

			if m.max >= len(m.files) {
				m.max = len(m.files) - 1
				m.min = m.max - m.Height
			}

		case key.Matches(msg, m.KeyMap.PageUp):

			m.selected -= m.Height
			if m.selected < 0 {
				m.selected = 0
			}
			m.min -= m.Height
			m.max -= m.Height

			if m.min < 0 {
				m.min = 0
				m.max = m.min + m.Height
			}

		case key.Matches(msg, m.KeyMap.Back):

			m.CurrentDirectory = filepath.Dir(m.CurrentDirectory)
			m.PathUI = m.CurrentDirectory
			if m.selectedStack.Length() > 0 {
				m.selected, m.min, m.max = m.popView()
			} else {
				m.selected = 0
				m.min = 0
				m.max = m.Height - 1
			}
			return m, readDir(m.CurrentDirectory, m.ShowHidden)

		case key.Matches(msg, m.KeyMap.Open):

			// if current dir is empty, do nothing
			if len(m.files) == 0 {
				break
			}

			// The key press was a selection, let's confirm whether the current file could
			// be selected or used for navigating deeper into the stack.
			f := m.files[m.selected]
			info, err := f.Info()
			if err != nil {
				break
			}
			isSymlink := info.Mode()&os.ModeSymlink != 0
			isDir := f.IsDir()

			if isSymlink {
				symlinkPath, _ := filepath.EvalSymlinks(filepath.Join(m.CurrentDirectory, f.Name()))
				info, err := os.Stat(symlinkPath)
				if err != nil {
					break
				}
				if info.IsDir() {
					isDir = true
				}
			}

			if (!isDir && m.FileAllowed) || (isDir && m.DirAllowed) {
				if key.Matches(msg, m.KeyMap.Select) {
					// Select the current path as the selection
					m.Path = filepath.Join(m.CurrentDirectory, f.Name())
					return m, tea.Quit
				}
			}

			if !isDir {
				break
			}

			m.CurrentDirectory = filepath.Join(m.CurrentDirectory, f.Name())
			m.PathUI = m.CurrentDirectory
			m.pushView()
			m.selected = 0
			m.min = 0
			m.max = m.Height - 1
			return m, readDir(m.CurrentDirectory, m.ShowHidden)

			//case key.Matches(msg, m.KeyMap.Quit):
			//	return m, tea.Quit
		}
	}
	return m, nil
}

// View returns the view of the file picker.
func (m Model) View() string {
	if len(m.files) == 0 {
		return m.Styles.EmptyDirectory.String()
	}
	var s strings.Builder

	main := lipgloss.NewStyle().Width(50).Align(lipgloss.Center).Render(m.PathUI)
	ui := lipgloss.JoinVertical(lipgloss.Center, main)

	dialog := lipgloss.Place(m.Width, 4,
		lipgloss.Center, lipgloss.Center,
		m.Styles.MainBox.Render(ui),
		lipgloss.WithWhitespaceForeground(subtle),
	)

	s.WriteString(dialog + "\n\n")

	for i, f := range m.files {
		// Skip files that are out of the range of the current view.
		if i < m.min {
			continue
		}
		// If we've reached the end of the view, stop.
		if i > m.max {
			break
		}
		// symlinkPath is the path that the symlink points to.
		var symlinkPath string
		info, _ := f.Info()
		isSymlink := info.Mode()&os.ModeSymlink != 0
		size := humanize.Bytes(uint64(info.Size()))
		name := f.Name()

		// If the file is a symlink, get the path that it points to.
		if isSymlink {
			symlinkPath, _ = filepath.EvalSymlinks(filepath.Join(m.CurrentDirectory, name))
		}

		// If the file is disabled, it cannot be selected.
		// User can define which file types are allowed to be selected via the AllowedTypes field.
		disabled := !m.canSelect(name) && !f.IsDir()

		if m.selected == i {
			selected := fmt.Sprintf(" %s %"+fmt.Sprint(m.Styles.FileSize.GetWidth())+"s %s", info.Mode().String(), size, name)
			if isSymlink {
				selected = fmt.Sprintf("%s → %s", selected, symlinkPath)
			}
			if disabled {
				s.WriteString(m.Styles.DisabledSelected.Render(m.Cursor) + m.Styles.DisabledSelected.Render(selected))
			} else {
				s.WriteString(m.Styles.Cursor.Render(m.Cursor) + m.Styles.Selected.Render(selected))
			}
			s.WriteRune('\n')
			continue
		}

		// Select the correct style for the name.
		style := m.Styles.File
		if f.IsDir() {
			style = m.Styles.Directory
		} else if isSymlink {
			style = m.Styles.Symlink
		} else if disabled {
			style = m.Styles.DisabledFile
		}

		fileName := style.Render(name)
		if isSymlink {
			fileName = fmt.Sprintf("%s → %s", fileName, symlinkPath)
		}
		s.WriteString(fmt.Sprintf("  %s %s %s", m.Styles.Permission.Render(info.Mode().String()), m.Styles.FileSize.Render(size), fileName))
		s.WriteRune('\n')
	}

	return s.String()
}

// SetHeight sets the height of the file picker. If AutoHeight is true, this
// will set AutoHeight to false.
func (m *Model) SetHeight(height int) {
	m.AutoHeight = false
	m.Height = height
}

func (m *Model) SetWidth(width int) {
	m.Width = width
}

// DidSelectFile returns whether a user has selected a file (on this msg).
func (m Model) DidSelectFile(msg tea.Msg) (bool, string) {
	didSelect, path := m.didSelectFile(msg)
	if didSelect && m.canSelect(path) {
		return true, path
	}
	return false, ""
}

// DidSelectDisabledFile returns whether a user tried to select a disabled file
// (on this msg). This is necessary only if you would like to warn the user that
// they tried to select a disabled file.
func (m Model) DidSelectDisabledFile(msg tea.Msg) (bool, string) {
	didSelect, path := m.didSelectFile(msg)
	if didSelect && !m.canSelect(path) {
		return true, path
	}
	return false, ""
}

func (m Model) didSelectFile(msg tea.Msg) (bool, string) {
	if len(m.files) == 0 {
		return false, ""
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If the msg does not match the Select keymap then this could not have been a selection.
		if !key.Matches(msg, m.KeyMap.Select) {
			return false, ""
		}

		// The key press was a selection, let's confirm whether the current file could
		// be selected or used for navigating deeper into the stack.
		f := m.files[m.selected]
		info, err := f.Info()
		if err != nil {
			return false, ""
		}
		isSymlink := info.Mode()&os.ModeSymlink != 0
		isDir := f.IsDir()

		if isSymlink {
			symlinkPath, _ := filepath.EvalSymlinks(filepath.Join(m.CurrentDirectory, f.Name()))
			info, err := os.Stat(symlinkPath)
			if err != nil {
				break
			}
			if info.IsDir() {
				isDir = true
			}
		}

		if (!isDir && m.FileAllowed) || (isDir && m.DirAllowed) && m.Path != "" {
			return true, m.Path
		}

		// If the msg was not a KeyMsg, then the file could not have been selected this iteration.
		// Only a KeyMsg can select a file.
	default:
		return false, ""
	}
	return false, ""
}

func (m Model) canSelect(file string) bool {
	if len(m.AllowedTypes) <= 0 {
		return true
	}

	for _, ext := range m.AllowedTypes {
		if strings.HasSuffix(file, ext) {
			return true
		}
	}
	return false
}
