// Telegram TUI client
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
	"github.com/zelenin/go-tdlib/client"
)

var tdlibClient *client.Client

type focus uint

const (
	chatsFocus focus = iota
	messagesFocus
)

var (
	modelStyle = lipgloss.NewStyle().
			Align(lipgloss.Center, lipgloss.Center).
			BorderStyle(lipgloss.HiddenBorder())
	focusedModelStyle = lipgloss.NewStyle().
				Align(lipgloss.Center, lipgloss.Center).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("69"))
)

type model struct {
	chatListModel    list.Model
	msgListModel     list.Model
	chatListItems    []list.Item
	chats            []*client.Chat
	currentChatIndex int
	msgListItems     []list.Item
	messages         *client.Messages
	focus            focus
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type item struct {
	title, desc string
}

type msgItem struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			if m.focus == chatsFocus {
				m.focus = messagesFocus
			} else {
				m.focus = chatsFocus
			}
		case "enter":
			if m.focus == chatsFocus {
				m.msgListItems = nil
				m.msgListModel.SetItems([]list.Item{})
				m.currentChatIndex = m.chatListModel.Index()
				currentChat := m.chats[m.currentChatIndex]
				messages, err := tdlibClient.GetChatHistory(&client.GetChatHistoryRequest{ChatId: currentChat.Id, OnlyLocal: false, Limit: 100})
				if err != nil {
					log.Printf("Cannot fetch messages: %s\n", err.Error())
				}
				m.messages = messages
				for _, msg := range messages.Messages {
					msgContent := ""
					switch content := msg.Content.(type) {
					case *client.MessageText:
						msgContent = content.Text.Text
					case *client.MessageAnimatedEmoji:
						msgContent = content.Emoji
					case *client.MessageSticker:
						msgContent = content.Sticker.Emoji
					case *client.MessageAnimation:
						msgContent = fmt.Sprintf("Animation %s", content.Caption.Text)
					case *client.MessagePhoto:
						msgContent = "Image"
					case *client.MessageAudio:
						msgContent = fmt.Sprintf("Audio %s (%d sec)", content.Audio.Title, content.Audio.Duration)
					case *client.MessageVideo:
						msgContent = fmt.Sprintf("Video (%d sec), %s", content.Video.Duration, content.Caption.Text)
					case *client.MessageContactRegistered:
						msgContent = "Joined Telegram"
					case *client.MessageCall:
						msgContent = fmt.Sprintf("Call (%d sec)", content.Duration)
					}
					m.msgListItems = append(m.msgListItems, item{title: msgContent})
				}

				m.msgListModel.SetItems(m.msgListItems)
			}
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.chatListModel.SetSize((msg.Width-h)/2, msg.Height-v)
		m.msgListModel.SetSize((msg.Width-h)/2, msg.Height-v)
	}

	m.msgListModel, cmd = m.msgListModel.Update(msg)
	m.chatListModel, cmd = m.chatListModel.Update(msg)

	return m, cmd
}

func (m model) View() string {
	var s string
	if m.focus == chatsFocus {
		s += lipgloss.JoinHorizontal(lipgloss.Top, focusedModelStyle.Render(m.chatListModel.View()), modelStyle.Render(m.msgListModel.View()))
	} else {
		s += lipgloss.JoinHorizontal(lipgloss.Top, modelStyle.Render(m.chatListModel.View()), focusedModelStyle.Render(m.msgListModel.View()))
	}

	return s
}

func main() {
	tdlibClient = runTelegramClient()
	chats, err := getChatList(tdlibClient)
	if err != nil {
		log.Printf("Error fetching chat list: %s\n", err.Error())
	}

	items := make([]list.Item, 0, len(chats))
	for _, chat := range chats {
		lastMsg := ""
		if chat.LastMessage != nil {
			switch content := chat.LastMessage.Content.(type) {
			case *client.MessageText:
				lastMsg = content.Text.Text
			case *client.MessageAnimatedEmoji:
				lastMsg = content.Emoji
			case *client.MessageSticker:
				lastMsg = content.Sticker.Emoji
			case *client.MessageAnimation:
				lastMsg = fmt.Sprintf("Animation %s", content.Caption.Text)
			case *client.MessagePhoto:
				lastMsg = "Image"
			case *client.MessageAudio:
				lastMsg = fmt.Sprintf("Audio %s (%d sec)", content.Audio.Title, content.Audio.Duration)
			case *client.MessageVideo:
				lastMsg = fmt.Sprintf("Video (%d sec), %s", content.Video.Duration, content.Caption.Text)
			case *client.MessageContactRegistered:
				lastMsg = "Joined Telegram"
			case *client.MessageCall:
				lastMsg = fmt.Sprintf("Call (%d sec)", content.Duration)
			}
		}
		items = append(items, item{title: chat.Title, desc: lastMsg})
	}

	p := tea.NewProgram(initialModel(items, chats))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func initialModel(chatListItems []list.Item, chats []*client.Chat) model {
	chatList := list.New(chatListItems, list.NewDefaultDelegate(), 0, 0)
	chatList.Title = "Chats"

	msgList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	msgList.Title = "Messages"
	return model{
		chatListModel:    chatList,
		msgListModel:     msgList,
		chats:            chats,
		chatListItems:    chatListItems,
		currentChatIndex: 0,
		focus:            chatsFocus,
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
