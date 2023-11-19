// Telegram TUI client
package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

type model struct {
	chats        string
	chat         string
	focus        string // left or right
	windowWidth  int
	windowHeight int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+l":
			{
				if m.focus == "right" {
					break
				}
				m.focus = "right"
			}
		case "ctrl+h":
			{
				if m.focus == "left" {
					break
				}
				m.focus = "left"
			}
		case "ctrl+c":
			{
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
	}

	return m, nil
}

func (m model) View() string {
	leftPaneView := m.chats
	rightPaneView := m.chat
	chatListWidth := m.windowWidth / 3
	chatContentWidth := m.windowWidth - chatListWidth

	if m.focus == "left" {
		leftPaneView = "[Focused] " + leftPaneView
	} else {
		rightPaneView = "[Focused] " + rightPaneView
	}

	content := leftPaneView + strings.Repeat(" ", chatListWidth-len(leftPaneView)) + " ▏" + rightPaneView + strings.Repeat(" ", chatContentWidth-len(rightPaneView)) + "\n"
	for i := 0; i < m.windowHeight; i++ {
		content += strings.Repeat(" ", chatListWidth)
		content += " ▏"
		content += strings.Repeat(" ", chatContentWidth) + "\n"
	}

	return content
}

func initialModel() model {
	termFileDescriptor := int(os.Stdin.Fd())

	width, height, err := term.GetSize(termFileDescriptor)

	fmt.Printf("Width: %d, Height: %d\n", width, height)
	if err != nil {
		width = 100
		height = 60
	}
	return model{
		chats:        "Chats",
		chat:         "Messages",
		focus:        "left",
		windowWidth:  width,
		windowHeight: height - 10,
	}
}
