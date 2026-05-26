package ui

import (
	"os"

	"github.com/rdx40/gmail-tui/api"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	stateLoading viewState = iota
	stateInbox
	stateViewing
	stateComposing
	stateReplying
	stateSearching
	stateLabels
)

// ---------- list.Item adapters ----------

// emailItem wraps an api.Email for display in the inbox list.
type emailItem struct{ email api.Email }

func (e emailItem) Title() string {
	if e.email.IsUnread {
		return "● " + e.email.Subject
	}
	return "  " + e.email.Subject
}

// Description returns the sender and snippet for the list delegate.
func (e emailItem) Description() string { return e.email.From + " - " + e.email.Snippet }

// FilterValue returns the text used for list filtering.
func (e emailItem) FilterValue() string { return e.email.Subject + " " + e.email.From }

// labelItem wraps an api.Label for display in the labels list.
type labelItem struct{ label api.Label }

func (l labelItem) Title() string       { return l.label.Name }
func (l labelItem) Description() string { return "" }
func (l labelItem) FilterValue() string { return l.label.Name }

// ---------- compose form ----------

type composeForm struct {
	from         textinput.Model
	to           textinput.Model
	cc           textinput.Model
	bcc          textinput.Model
	subject      textinput.Model
	body         textarea.Model
	attachments  []string
	attachInput  textinput.Model
	addingAttach bool
	focused      int
}

func newComposeForm() composeForm {
	return composeForm{
		from:        newTextInput("From", 100),
		to:          newTextInput("To", 100),
		cc:          newTextInput("CC", 100),
		bcc:         newTextInput("BCC", 100),
		subject:     newTextInput("Subject", 200),
		body:        newTextArea("Compose your message here...", 80, 10),
		attachInput: newTextInput("Path to attachment...", 300),
	}
}

func (f *composeForm) reset(from string) {
	f.from.SetValue(from)
	f.to.SetValue("")
	f.cc.SetValue("")
	f.bcc.SetValue("")
	f.subject.SetValue("")
	f.body.Reset()
	f.attachments = nil
	f.addingAttach = false
	f.focused = 1 // start on To
}

func (f *composeForm) focusField() tea.Cmd {
	f.from.Blur()
	f.to.Blur()
	f.cc.Blur()
	f.bcc.Blur()
	f.subject.Blur()
	f.body.Blur()

	switch f.focused {
	case 0:
		return f.from.Focus()
	case 1:
		return f.to.Focus()
	case 2:
		return f.cc.Focus()
	case 3:
		return f.bcc.Focus()
	case 4:
		return f.subject.Focus()
	case 5:
		return f.body.Focus()
	}
	return nil
}

// ---------- model ----------

// Model is the top-level Bubble Tea model.
type Model struct {
	client api.ClientInterface
	config *api.Config
	state  viewState
	width  int
	height int

	// Inbox
	emailList list.Model

	// Pagination
	pageToken    string   // next page token from last inbox fetch
	pageHistory  []string // stack of previous page tokens
	currentPage  int      // 1-based current page number
	totalFetched int      // total emails fetched so far

	// Email viewing
	viewport     viewport.Model
	currentEmail *api.Email

	// Compose / Reply
	compose          composeForm
	isReply          bool
	replyTo          *api.Email
	replyBody        textarea.Model
	replyAttachments []string
	confirmDiscard   bool // true when waiting for discard confirmation
	confirmNoSubject bool // true when waiting for no-subject send confirmation

	// Search
	searchInput textinput.Model
	lastQuery   string // preserved last search query

	// Labels
	labelList list.Model

	// Shared
	spinner       spinner.Model
	help          help.Model
	showHelp      bool
	statusMessage string
	attachInput   textinput.Model
	addingAttach  bool
	userEmail     string // authenticated user's email address
	noStyle       bool   // true when TERM=dumb

	// Delete confirmation
	confirmDelete  bool
	deleteTargetID string
	deleteSubject  string

	// Attachment download
	downloading bool
}

// New creates an initial Model in loading state.
func New(client api.ClientInterface, cfg *api.Config) Model {
	noStyle := os.Getenv("TERM") == "dumb"

	delegate := list.NewDefaultDelegate()
	if !noStyle {
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
			BorderForeground(lipgloss.Color("62")).
			Foreground(lipgloss.Color("62"))
		delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.
			Foreground(lipgloss.Color("245"))
	}

	el := list.New([]list.Item{}, delegate, 0, 0)
	el.Title = "Inbox"
	el.Styles.Title = lipgloss.NewStyle().MarginLeft(2)
	el.SetShowStatusBar(true)
	el.SetFilteringEnabled(true)
	el.SetShowHelp(false)
	el.DisableQuitKeybindings()
	el.KeyMap.Quit = key.NewBinding(key.WithKeys("q"))

	ll := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	ll.Title = "Labels"
	ll.SetShowHelp(false)
	ll.DisableQuitKeybindings()
	ll.KeyMap.Quit = key.NewBinding(key.WithKeys("q"))
	ll.KeyMap.CursorUp = key.NewBinding(key.WithKeys("up", "k"))
	ll.KeyMap.CursorDown = key.NewBinding(key.WithKeys("down", "j"))
	ll.KeyMap.GoToStart = key.NewBinding(key.WithKeys("home", "g"))
	ll.KeyMap.GoToEnd = key.NewBinding(key.WithKeys("end", "G"))

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	vp := viewport.New(20, 10)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	h := help.New()
	h.ShowAll = false

	return Model{
		client:      client,
		config:      cfg,
		state:       stateLoading,
		emailList:   el,
		viewport:    vp,
		spinner:     s,
		help:        h,
		compose:     newComposeForm(),
		replyBody:   newTextArea("Type your reply here...", 80, 10),
		searchInput: newTextInput("from: is:unread subject: has:attachment", 200),
		labelList:   ll,
		attachInput: newTextInput("Path to attachment...", 300),
		currentPage: 1,
		noStyle:     noStyle,
	}
}

// Init starts the async inbox fetch and user profile fetch.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchInbox(m.client, int64(m.config.MaxResults)),
		fetchUserProfileCmd(m.client),
	)
}

// ---------- helpers ----------

func newTextInput(placeholder string, charLimit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = charLimit
	return ti
}

func newTextArea(placeholder string, width, height int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = placeholder
	ta.CharLimit = 0
	ta.SetWidth(width)
	ta.SetHeight(height)
	return ta
}

// emailsToItems converts a slice of emails to list items.
func emailsToItems(emails []api.Email) []list.Item {
	items := make([]list.Item, len(emails))
	for i, e := range emails {
		items[i] = emailItem{email: e}
	}
	return items
}

// labelsToItems converts a slice of labels to list items.
func labelsToItems(labels []api.Label) []list.Item {
	items := make([]list.Item, len(labels))
	for i, l := range labels {
		items[i] = labelItem{label: l}
	}
	return items
}
