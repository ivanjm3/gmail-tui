package main

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"google.golang.org/api/gmail/v1"
)

type state int

const (
	stateInbox state = iota
	stateViewing
	stateLoading
	stateComposing
	stateReplying
	stateSearching
	stateManagingLabels
)

// keyMap defines all keyboard shortcuts
type keyMap struct {
	Back               key.Binding
	Reply              key.Binding
	Compose            key.Binding
	Delete             key.Binding
	Search             key.Binding
	Labels             key.Binding
	ToggleRead         key.Binding
	Quit               key.Binding
	Send               key.Binding
	NextInput          key.Binding
	PrevInput          key.Binding
	ShowHelp           key.Binding
	CloseHelp          key.Binding
	Select             key.Binding
	AddAttachment      key.Binding
	RemoveAttachment   key.Binding
	DownloadAttachment key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.ShowHelp, k.Compose, k.Search, k.Labels, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Compose, k.Reply, k.Search, k.Labels},
		{k.Delete, k.ToggleRead, k.Back, k.Quit},
		{k.Send, k.NextInput, k.PrevInput},
		{k.AddAttachment, k.RemoveAttachment, k.DownloadAttachment},
	}
}

var keys = keyMap{
	Back:               key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b/esc", "back")),
	Reply:              key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reply")),
	Compose:            key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "compose")),
	Delete:             key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Search:             key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Labels:             key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "labels")),
	ToggleRead:         key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mark read/unread")),
	Quit:               key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Send:               key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send")),
	NextInput:          key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
	PrevInput:          key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field")),
	ShowHelp:           key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	CloseHelp:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "close help")),
	Select:             key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	AddAttachment:      key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "add attachment")),
	RemoveAttachment:   key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("ctrl+x", "remove attachment")),
	DownloadAttachment: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "download attachment")),
}

// emailItem represents an email in the list or detail view
type emailItem struct {
	id          string
	threadId    string
	subject     string
	from        string
	snippet     string
	date        string
	labels      []string
	isUnread    bool
	body        string
	recipient   string
	cc          string
	bcc         string
	attachments []*gmail.MessagePart
}

func (e emailItem) Title() string {
	if e.isUnread {
		return "● " + e.subject
	}
	return "  " + e.subject
}

func (e emailItem) Description() string {
	snippet := e.snippet
	if len(snippet) > 80 {
		snippet = snippet[:77] + "..."
	}
	return e.from + " - " + snippet
}

func (e emailItem) FilterValue() string {
	return e.subject + " " + e.from
}

// labelItem represents a Gmail label
type labelItem struct {
	label *gmail.Label
}

func (l labelItem) Title() string       { return l.label.Name }
func (l labelItem) Description() string { return "ID: " + l.label.Id }
func (l labelItem) FilterValue() string { return l.label.Name }

// model is the main application state
type model struct {
	state                 state
	list                  list.Model
	srv                   *gmail.Service
	fullEmail             string
	loading               spinner.Model
	viewport              viewport.Model
	width                 int
	height                int
	err                   string
	help                  help.Model
	showHelp              bool
	composeFrom           textinput.Model
	composeTo             textinput.Model
	composeCc             textinput.Model
	composeBcc            textinput.Model
	composeSubj           textinput.Model
	composeBody           textarea.Model
	replyBody             textarea.Model
	searchInput           textinput.Model
	labels                []*gmail.Label
	labelsList            list.Model
	currentMsg            *emailItem
	replyToMsg            *emailItem
	focused               int
	searchQuery           string
	composeAttachments    []string
	replyAttachments      []string
	attachmentInput       textinput.Model
	addingAttachment      bool
	attachmentDownloading bool
	downloadingIndex      int
}

// Messages for tea.Cmd communication
type (
	emailLoadedMsg          struct{ content string }
	emailSentMsg            struct{}
	emailLoadErrorMsg       struct{ err error }
	labelsLoadedMsg         struct{ labels []*gmail.Label }
	searchResultMsg         struct{ messages []*gmail.Message }
	attachmentDownloadedMsg struct{ filename string }
	notificationMsg         struct{ message string }
)
