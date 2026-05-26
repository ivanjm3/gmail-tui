package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

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
	NextPage           key.Binding
	PrevPage           key.Binding
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
		{k.NextPage, k.PrevPage},
	}
}

var _ help.KeyMap = keyMap{}

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
	NextPage:           key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next page")),
	PrevPage:           key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev page")),
}
