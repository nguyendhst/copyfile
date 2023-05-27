package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nguyendhst/copyfile/filepicker"

	"github.com/buger/goterm"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	filepicker   filepicker.Model
	selectedFile string
	quitting     bool
}

type path struct {
	absolute bool
	truePath string
}

func NewPath(x string) path {
	return path{absolute: _isAbsolutePath(x), truePath: _truePath(x)}
}

func (m model) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		m.selectedFile = path
	}

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	var s strings.Builder
	s.WriteString("\n  ")
	if m.selectedFile == "" {
		s.WriteString("Pick a file:")
	} else {
		s.WriteString("Selected file: " + m.filepicker.Styles.Selected.Render(m.selectedFile))
	}
	s.WriteString("\n\n" + m.filepicker.View() + "\n")
	return s.String()
}

// TODO: add a flag to show hidden files
func main() {

	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		fmt.Println("Sorry, this program is not supported on " + runtime.GOOS + ".")
		return
	}

	path := ""
	if len(os.Args) > 1 {
		path = os.Args[1]
	} else {
		path, _ = os.Getwd()
	}

	p := NewPath(path)

	fp := filepicker.NewWithConfig(10, goterm.Width()-2, p.truePath)

	m := model{
		filepicker: fp,
	}
	tm, _ := tea.NewProgram(&m, tea.WithOutput(os.Stderr)).Run()
	mm := tm.(model)

	if mm.selectedFile != "" {
		if runtime.GOOS == "darwin" {
			exec.Command("cp", mm.selectedFile, ".").Run()
		} else {
			exec.Command("copy", mm.selectedFile, ".").Run()
		}
		fmt.Println("\n  Copied: " + m.filepicker.Styles.Selected.Render(mm.selectedFile) + "\n")
	}
}

func _isAbsolutePath(path string) bool {
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~")
}

func _truePath(path string) string {
	if strings.HasPrefix(path, "~") {
		path = strings.Replace(path, "~", os.Getenv("HOME"), 1)
	} else if !strings.HasPrefix(path, "/") {
		curr, _ := os.Getwd()
		path = curr + "/" + path
		path = strings.Replace(path, "//", "/", -1)
		path, _ = filepath.Abs(path)
	}
	return path
}
