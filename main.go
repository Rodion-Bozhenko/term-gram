// Telegram TUI client
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/mattn/go-runewidth"
	"github.com/zelenin/go-tdlib/client"
	"golang.org/x/term"
)

type model struct {
	chats        []*client.Chat
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

func truncateStringToDisplayWidth(s string, maxWidth int) string {
	width := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if width+rw > maxWidth {
			return s[:i]
		}
		width += rw
	}
	return s
}

func (m model) View() string {
	leftPaneView := "Chats"
	rightPaneView := m.chat
	chatListWidth := m.windowWidth / 3
	chatContentWidth := m.windowWidth - chatListWidth

	if m.focus == "left" {
		leftPaneView = "[Focused] " + leftPaneView
	} else {
		rightPaneView = "[Focused] " + rightPaneView
	}

	content := leftPaneView + strings.Repeat(" ", chatListWidth-len(leftPaneView)) + " ▏" + rightPaneView + strings.Repeat(" ", chatContentWidth-len(rightPaneView)) + "\n"
	for i := 0; i < len(m.chats); i++ {
		chat := m.chats[i]
		var leftContent string
		displayWidth := runewidth.StringWidth(chat.Title)

		if displayWidth > chatListWidth {
			truncatedTitle := truncateStringToDisplayWidth(chat.Title, chatListWidth)
			leftContent = truncatedTitle + strings.Repeat(" ", chatListWidth-runewidth.StringWidth(truncatedTitle))
		} else {
			leftContent = chat.Title + strings.Repeat(" ", chatListWidth-displayWidth)
		}

		content += leftContent
		content += " ▏"
		content += strconv.Itoa(displayWidth) + " " + strings.Repeat(" ", chatContentWidth-4) + "\n"
	}
	return content
}

func initialModel(chats []*client.Chat) model {
	termFileDescriptor := int(os.Stdin.Fd())

	width, height, err := term.GetSize(termFileDescriptor)

	if err != nil {
		width = 100
		height = 60
	}
	return model{
		chats:        chats,
		chat:         "Messages",
		focus:        "left",
		windowWidth:  width,
		windowHeight: height - 10,
	}
}

func main() {
	tdlibClient := runTelegramClient()
	chats, err := getChatList(tdlibClient)
	if err != nil {
		log.Printf("Error fetching chat list: %s\n", err.Error())
	}

	p := tea.NewProgram(initialModel(chats))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func runTelegramClient() *client.Client {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Cannot load .env file: %s", err.Error())
	}

	authorizer := client.ClientAuthorizer()
	go client.CliInteractor(authorizer)

	var (
		apiIDRaw = os.Getenv("API_ID")
		apiHash  = os.Getenv("API_HASH")
	)
	if apiIDRaw == "" {
		log.Fatalf("No API_ID provided")
	}
	if apiHash == "" {
		log.Fatalf("No API_HASH provided")
	}

	apiID64, err := strconv.ParseInt(apiIDRaw, 10, 32)
	if err != nil {
		log.Fatalf("strconv.Atoi error: %s", err)
	}

	apiID := int32(apiID64)

	authorizer.TdlibParameters <- &client.SetTdlibParametersRequest{
		UseTestDc:              false,
		DatabaseDirectory:      filepath.Join(".tdlib", "database"),
		FilesDirectory:         filepath.Join(".tdlib", "files"),
		UseFileDatabase:        true,
		UseChatInfoDatabase:    true,
		UseMessageDatabase:     true,
		UseSecretChats:         false,
		ApiId:                  apiID,
		ApiHash:                apiHash,
		SystemLanguageCode:     "en",
		DeviceModel:            "Server",
		SystemVersion:          "1.0.0",
		ApplicationVersion:     "1.0.0",
		EnableStorageOptimizer: true,
		IgnoreFileNames:        false,
	}

	_, err = client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{
		NewVerbosityLevel: 1,
	})
	if err != nil {
		log.Fatalf("SetLogVerbosityLevel error: %s", err)
	}

	tdlibClient, err := client.NewClient(authorizer)
	if err != nil {
		log.Fatalf("NewClient error: %s", err)
	}

	optionValue, err := client.GetOption(&client.GetOptionRequest{
		Name: "version",
	})
	if err != nil {
		log.Fatalf("GetOption error: %s", err)
	}

	log.Printf("TDLib version: %s", optionValue.(*client.OptionValueString).Value)

	me, err := tdlibClient.GetMe()
	if err != nil {
		log.Fatalf("GetMe error: %s", err)
	}

	log.Printf("Me: %s %s [%v]", me.FirstName, me.LastName, me.Usernames)

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		tdlibClient.Stop()
		os.Exit(1)
	}()

	return tdlibClient
}

func getChatList(tdlibClient *client.Client) ([]*client.Chat, error) {
	chatList, err := tdlibClient.GetChats(&client.GetChatsRequest{
		ChatList: nil,
		Limit:    100,
	})
	if err != nil {
		return nil, err
	}

	var chats []*client.Chat
	for _, chatID := range chatList.ChatIds {
		chat, err := tdlibClient.GetChat(&client.GetChatRequest{ChatId: chatID})
		if err != nil {
			continue
		}
		chats = append(chats, chat)
	}

	return chats, nil
}
