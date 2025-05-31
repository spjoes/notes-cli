package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	notesList list.Model
}

type NoteItem struct {
	ID        string
	Message   string
	File      string
	Line      int
	CreatedAt time.Time
	Tags      []string
}

var _ list.Item = (*NoteItem)(nil)

func (i NoteItem) Title() string {
	if len(i.Message) > 40 {
		return i.Message[:37] + "..."
	}
	return i.Message
}

func (i NoteItem) Description() string {
	loc := ""
	if i.File != "" {
		if i.Line > 0 {
			loc = fmt.Sprintf("%s:%d", i.File, i.Line)
		} else {
			loc = i.File
		}
	}
	if len(i.Tags) > 0 {
		loc += " [" + strings.Join(i.Tags, ", ") + "]"
	}
	return loc
}

func (i NoteItem) FilterValue() string {
	return i.Message
}

func initialModel() (model, error) {
	notes, err := LoadAllNotes()
	if err != nil {
		return model{}, err
	}

	items := make([]list.Item, len(notes))
	for idx, n := range notes {
		items[idx] = NoteItem{
			ID:        n.ID,
			Message:   n.Message,
			File:      n.File,
			Line:      n.Line,
			CreatedAt: n.CreatedAt,
			Tags:      n.Tags,
		}
	}

	const listWidth = 60
	const listHeight = 15

	delegate := list.NewDefaultDelegate()

	l := list.New(items, delegate, listWidth, listHeight)
	l.Title = "Notes (press ↑/↓ to scroll, q to quit)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = l.Styles.Title.Blink(true)

	return model{notesList: l}, nil
}

func LoadAllNotes() ([]Note, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	notesPath := filepath.Join(root, ".notes", "notes.json")
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		return []Note{}, nil
	}

	data, err := os.ReadFile(notesPath)
	if err != nil {
		return nil, err
	}

	var notes []Note
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, err
	}

	return notes, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch key := msg.String(); key {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}

	newList, cmd := m.notesList.Update(msg)
	m.notesList = newList
	return m, cmd
}

func (m model) View() string {
	return "\n" + m.notesList.View() + "\n\nPress q to quit."
}
