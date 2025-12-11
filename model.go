package main

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"google.golang.org/api/gmail/v1"
)

func initialModel(emails []*gmail.Message, srv *gmail.Service, labels []*gmail.Label) model {
	items := make([]list.Item, 0, len(emails))
	for _, msg := range emails {
		if item := createEmailItem(srv, msg.Id, false); item != nil {
			items = append(items, *item)
		}
	}

	delegate := createListDelegate()
	emailList := createEmailList(items, delegate)
	labelsList := createLabelsList()

	return model{
		state:              stateInbox,
		list:               emailList,
		srv:                srv,
		loading:            createSpinner(),
		viewport:           createViewport(),
		help:               createHelp(),
		composeFrom:        createTextInput("From", 100),
		composeTo:          createTextInput("To", 100),
		composeCc:          createTextInput("CC", 100),
		composeBcc:         createTextInput("BCC", 100),
		composeSubj:        createTextInput("Subject", 200),
		composeBody:        createTextArea("Compose your message here...", 80, 10),
		replyBody:          createTextArea("Type your reply here...", 80, 10),
		searchInput:        createTextInput("Search emails...", 200),
		attachmentInput:    createTextInput("Path to attachment...", 300),
		labels:             labels,
		labelsList:         labelsList,
		composeAttachments: []string{},
		replyAttachments:   []string{},
		focused:            0,
	}
}

func (m model) Init() tea.Cmd {
	return m.loading.Tick
}

// UI component factories
func createListDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		BorderForeground(lipgloss.Color("62")).
		Foreground(lipgloss.Color("62"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy().
		Foreground(lipgloss.Color("245"))
	return delegate
}

func createEmailList(items []list.Item, delegate list.DefaultDelegate) list.Model {
	l := list.New(items, delegate, 0, 0)
	l.Title = "Inbox"
	l.Styles.Title = lipgloss.NewStyle().MarginLeft(2)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("q"))
	return l
}

func createLabelsList() list.Model {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Labels"
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.KeyMap.Quit = key.NewBinding(key.WithKeys("q"))
	l.KeyMap.CursorUp = key.NewBinding(key.WithKeys("up", "k"))
	l.KeyMap.CursorDown = key.NewBinding(key.WithKeys("down", "j"))
	l.KeyMap.GoToStart = key.NewBinding(key.WithKeys("home", "g"))
	l.KeyMap.GoToEnd = key.NewBinding(key.WithKeys("end", "G"))
	return l
}

func createSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return s
}

func createViewport() viewport.Model {
	vp := viewport.New(20, 10)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	return vp
}

func createHelp() help.Model {
	h := help.New()
	h.ShowAll = false
	return h
}

func createTextInput(placeholder string, charLimit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = charLimit
	return ti
}

func createTextArea(placeholder string, width, height int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.CharLimit = 0
	ta.SetWidth(width)
	ta.SetHeight(height)
	return ta
}

// Focus management
func (m *model) focusComposeField() tea.Cmd {
	m.composeFrom.Blur()
	m.composeTo.Blur()
	m.composeCc.Blur()
	m.composeBcc.Blur()
	m.composeSubj.Blur()
	m.composeBody.Blur()

	switch m.focused {
	case 0:
		return m.composeFrom.Focus()
	case 1:
		return m.composeTo.Focus()
	case 2:
		return m.composeCc.Focus()
	case 3:
		return m.composeBcc.Focus()
	case 4:
		return m.composeSubj.Focus()
	case 5:
		return m.composeBody.Focus()
	}
	return nil
}
