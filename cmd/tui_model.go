package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	notesList        list.Model
	confirmingDelete bool
	deleteIndex      int
	addStage         int
	editStage        int
	editItem         NoteItem
	textInput        textinput.Model
	fileList         list.Model
	newMsg           string
	selectedFile     string
	newTags          []string
	width            int
	height           int
	searchMode       bool
	searchInput      textinput.Model
	allItems         []NoteItem
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
		loc += " "
		if i.Line > 0 {
			loc += fmt.Sprintf("%s:%d", i.File, i.Line)
		} else {
			loc += i.File
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
	all := make([]NoteItem, len(notes))
	for i, n := range notes {
		ni := NoteItem{
			ID:        n.ID,
			Message:   n.Message,
			File:      n.File,
			Line:      n.Line,
			CreatedAt: n.CreatedAt,
			Tags:      n.Tags,
		}
		items[i] = ni
		all[i] = ni
	}

	const listWidth = 20
	const listHeight = 10

	delegate := list.NewDefaultDelegate()

	l := list.New(items, delegate, listWidth, listHeight)
	l.Title = "Notes (press ↑/↓ to scroll, Delete to delete, q to quit)"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	ti := textinput.New()
	ti.Placeholder = ""
	ti.CharLimit = 256
	ti.Width = listWidth - 2

	si := textinput.New()
	si.Placeholder = "Search (use # to search by tag)"
	si.CharLimit = 256
	si.Width = listWidth - 2

	emptyList := list.New([]list.Item{}, list.NewDefaultDelegate(), listWidth, listHeight)
	emptyList.Title = "Select a file..."

	return model{
		notesList:        l,
		confirmingDelete: false,
		deleteIndex:      -1,
		addStage:         0,
		editStage:        0,
		editItem:         NoteItem{},
		textInput:        ti,
		newMsg:           "",
		fileList:         emptyList,
		selectedFile:     "",
		newTags:          []string{},
		width:            0,
		height:           0,
		searchMode:       false,
		searchInput:      si,
		allItems:         all,
	}, nil
}

func filterItems(all []NoteItem, rawQuery string) []list.Item {
	raw := strings.TrimSpace(rawQuery)
	if raw == "" {
		out := make([]list.Item, len(all))
		for i, n := range all {
			out[i] = n
		}
		return out
	}

	terms := strings.Fields(raw)
	var tagFilters []string
	var textFilters []string
	for _, t := range terms {
		if strings.HasPrefix(t, "#") && len(t) > 1 {
			tagFilters = append(tagFilters, strings.ToLower(t[1:]))
		} else {
			textFilters = append(textFilters, strings.ToLower(t))
		}
	}

	var filtered []list.Item
	for _, ni := range all {
		matches := true
		lowMsg := strings.ToLower(ni.Message)
		lowFile := strings.ToLower(ni.File)

		for _, tf := range textFilters {
			if !strings.Contains(lowMsg, tf) && !strings.Contains(lowFile, tf) {
				matches = false
				break
			}
		}

		if !matches {
			continue
		}

		for _, tg := range tagFilters {
			foundTag := false
			for _, noteTag := range ni.Tags {
				if strings.ToLower(noteTag) == tg {
					foundTag = true
					break
				}
			}
			if !foundTag {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}

		filtered = append(filtered, ni)
	}

	return filtered
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

func DeleteNoteByID(id string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	notesPath := filepath.Join(root, ".notes", "notes.json")
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		return fmt.Errorf("notes file does not exist")
	}

	data, err := os.ReadFile(notesPath)
	if err != nil {
		return err
	}

	var notes []Note
	if err := json.Unmarshal(data, &notes); err != nil {
		return err
	}

	var updated []Note
	found := false
	for _, n := range notes {
		short := n.ID
		if len(short) > 8 {
			short = short[:8]
		}
		if short == id || n.ID == id {
			found = true
			continue
		}
		updated = append(updated, n)
	}

	if !found {
		return fmt.Errorf("no note found with id %s", id)
	}

	out, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(notesPath, out, 0644)
}

var (
	modalBorder = lipgloss.RoundedBorder()
	modalStyle  = lipgloss.NewStyle().
			Border(modalBorder, true).
			BorderForeground(lipgloss.Color("#FF5F87")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#333333")).
			Padding(1, 2).
			Render
)

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		w := max(1, m.width-2)
		h := max(1, m.height-4)
		m.notesList.SetSize(w, h)
		m.textInput.Width = max(1, m.width-6)
		if m.addStage == 2 {
			m.fileList.SetSize(w, h)
		}
		if m.editStage == 2 {
			m.fileList.SetSize(w, h)
		}
		if m.searchMode {
			m.searchInput.Width = max(1, m.width-2)
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		if m.searchMode {
			switch key {
			case "esc", "ctrl+c":
				m.searchMode = false
				items := make([]list.Item, len(m.allItems))
				for i, ni := range m.allItems {
					items[i] = ni
				}
				m.notesList.SetItems(items)
				return m, nil
			}

			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			query := m.searchInput.Value()
			items := filterItems(m.allItems, query)
			m.notesList.SetItems(items)
			return m, cmd
		}

		if m.addStage > 0 {
			switch key {
			case "enter":
				value := strings.TrimSpace(m.textInput.Value())
				switch m.addStage {
				case 1:
					m.newMsg = value
					m.addStage = 2
					var items []list.Item
					root, _ := os.Getwd()
					filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
							return filepath.SkipDir
						}
						if !info.IsDir() {
							rel, _ := filepath.Rel(root, path)
							items = append(items, NoteItem{Message: rel})
						}
						return nil
					})

					items = append([]list.Item{NoteItem{Message: "(No File)"}}, items...)
					w := max(1, m.width-2)
					h := max(1, m.height-4)
					m.fileList = list.New(items, list.NewDefaultDelegate(), w, h)
					m.fileList.Title = "Step 2/3: Select file (Enter to choose, Esc to cancel)"

					return m, nil
				case 2:
					selected := m.fileList.SelectedItem().(NoteItem).Message
					if selected == "(No File)" {
						m.selectedFile = ""
					} else {
						m.selectedFile = selected
					}
					m.addStage = 3
					m.textInput.SetValue("")
					m.textInput.Placeholder = "Tags (comma separated, leave blank for none)"
					m.textInput.Focus()
					return m, nil
				case 3:
					rawTags := strings.TrimSpace(m.textInput.Value())
					if rawTags != "" {
						parts := strings.Split(rawTags, ",")
						for i := range parts {
							parts[i] = strings.TrimSpace(parts[i])
						}
						m.newTags = parts
					}
					if err := SaveNote(m.newMsg, m.selectedFile, 0, m.newTags); err != nil {
						return m, tea.Printf("failed to save note: %v", err)
					}
					newModel, _ := initialModel()
					newModel.width, newModel.height = m.width, m.height
					w := max(1, m.width-2)
					h := max(1, m.height-4)
					newModel.notesList.SetSize(w, h)
					newModel.addStage = 0
					return newModel, nil
				}

			case "esc", "ctrl+c":
				m.addStage = 0
				m.textInput.Blur()
				return m, nil
			}

			if m.addStage == 2 {
				updatedList, cmd := m.fileList.Update(msg)
				m.fileList = updatedList
				return m, cmd
			}

			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd

		}

		if m.editStage > 0 {
			switch key {
			case "enter":
				value := strings.TrimSpace(m.textInput.Value())
				switch m.editStage {
				case 1:
					m.editItem.Message = value
					m.editStage = 2

					var items []list.Item
					root, _ := os.Getwd()
					filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
							return filepath.SkipDir
						}
						if !info.IsDir() {
							rel, _ := filepath.Rel(root, path)
							items = append(items, NoteItem{Message: rel})
						}
						return nil
					})

					items = append([]list.Item{NoteItem{Message: "(No File)"}}, items...)
					w := max(1, m.width-2)
					h := max(1, m.height-4)
					m.fileList = list.New(items, list.NewDefaultDelegate(), w, h)
					m.fileList.Title = "Step 2/3: Select file (Enter to choose, Esc to skip)"

					if m.editItem.File == "" {
						m.fileList.Select(0)
					} else {
						for i := 1; i < len(items); i++ {
							if items[i].(NoteItem).Message == m.editItem.File {
								m.fileList.Select(i)
								break
							}
						}
					}

					return m, nil

				case 2:
					sel := m.fileList.SelectedItem().(NoteItem).Message
					if sel == "(No File)" {
						m.editItem.File = ""
					} else {
						m.editItem.File = sel
					}
					m.editStage = 3
					m.textInput.SetValue(strings.Join(m.editItem.Tags, ", "))
					m.textInput.Placeholder = "Edit tags (comma-separated, Esc to skip)"
					m.textInput.Focus()
					return m, nil

				case 3:
					rawTags := strings.TrimSpace(m.textInput.Value())
					if rawTags != "" {
						parts := strings.Split(rawTags, ",")
						for i := range parts {
							parts[i] = strings.TrimSpace(parts[i])
						}
						m.editItem.Tags = parts
					} else {
						m.editItem.Tags = []string{}
					}

					root, _ := os.Getwd()
					notesPath := filepath.Join(root, ".notes", "notes.json")
					bytes, _ := os.ReadFile(notesPath)
					var allNotes []Note
					_ = json.Unmarshal(bytes, &allNotes)

					for i := range allNotes {
						if allNotes[i].ID == m.editItem.ID {
							allNotes[i].Message = m.editItem.Message
							allNotes[i].File = m.editItem.File
							allNotes[i].Tags = m.editItem.Tags
							allNotes[i].CreatedAt = m.editItem.CreatedAt
							break
						}
					}

					out, _ := json.MarshalIndent(allNotes, "", "  ")
					_ = os.WriteFile(notesPath, out, 0644)

					newModel, _ := initialModel()
					newModel.width, newModel.height = m.width, m.height
					w := max(1, m.width-2)
					h := max(1, m.height-4)
					newModel.notesList.SetSize(w, h)
					newModel.editStage = 0
					return newModel, nil
				}

			case "esc", "ctrl+c":
				m.editStage = 0
				m.textInput.Blur()
				return m, nil
			}

			if m.editStage == 2 {
				updatedList, cmd := m.fileList.Update(msg)
				m.fileList = updatedList
				return m, cmd
			}

			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}

		if m.confirmingDelete {
			switch key {
			case "y", "yes", "Y", "enter":
				item := m.notesList.Items()[m.deleteIndex].(NoteItem)
				if err := DeleteNoteByID(item.ID); err != nil {
					fmt.Fprintf(os.Stderr, "Error deleting note: %v\n", err)
				}

				newModel, _ := initialModel()
				newModel.width, newModel.height = m.width, m.height
				w := max(1, m.width-2)
				h := max(1, m.height-4)
				newModel.notesList.SetSize(w, h)
				return newModel, nil

			default:
				// Make any other key cancel the deletion
				m.confirmingDelete = false
				m.deleteIndex = -1
				return m, nil
			}
		}

		switch key {
		case "q", "esc", "ctrl+c", "ctrl+q":
			return m, tea.Quit

		case "ctrl+a":
			m.addStage = 1
			m.newMsg = ""
			m.selectedFile = ""
			m.newTags = []string{}
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Note message"
			m.textInput.Focus()
			m.textInput.Width = max(1, m.width-6)
			return m, nil

		case "ctrl+e":
			idx := m.notesList.Index()
			if idx >= 0 && idx < len(m.notesList.Items()) {
				selected := m.notesList.Items()[idx].(NoteItem)
				m.editItem = selected
				m.editStage = 1
				m.textInput.SetValue(selected.Message)
				m.textInput.Placeholder = "Edit message (Enter to continue, Esc to cancel)"
				m.textInput.Focus()
				m.textInput.Width = max(1, m.width-6)
			}
			return m, nil

		case "delete", "ctrl+d":
			idx := m.notesList.Index()
			if idx >= 0 && idx < len(m.notesList.Items()) {
				m.confirmingDelete = true
				m.deleteIndex = idx
			}

		case "/", "ctrl+f":
			m.searchMode = true
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			m.searchInput.Width = max(1, m.width-2)
			return m, nil
		}
	}
	updatedList, cmd := m.notesList.Update(msg)
	m.notesList = updatedList
	return m, cmd
}

func (m model) View() string {

	if m.searchMode {
		bar := "Search: " + m.searchInput.View()
		return fmt.Sprintf("%s\n\n%s\n\n(Enter to filter, Esc to clear)", bar, m.notesList.View())
	}

	if m.addStage > 0 || m.editStage > 0 {
		if m.addStage > 0 {
			switch m.addStage {
			case 1:
				prompt := "Step 1/3: Enter note message (Enter to continue, Esc to cancel)\n\n"
				raw := prompt + m.textInput.View()
				wrap := lipgloss.NewStyle().MaxWidth(m.width - 6).Render(raw)
				box := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder(), true).
					BorderForeground(lipgloss.Color("#5DAFF4")).
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#333333")).
					Padding(1, 2).
					Render(wrap)
				return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
			case 2:
				return "\n" + m.fileList.View() + "\n\n(Use ↑/↓, Enter to pick, Esc to cancel)"
			case 3:
				prompt := "Step 3/3: Enter tags (comma-separated, leave blank) (Enter to save, Esc to cancel)\n\n"
				raw := prompt + m.textInput.View()
				wrap := lipgloss.NewStyle().MaxWidth(m.width - 6).Render(raw)
				box := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder(), true).
					BorderForeground(lipgloss.Color("#5DAFF4")).
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#333333")).
					Padding(1, 2).
					Render(wrap)
				return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
			}
		}

		if m.editStage > 0 {
			switch m.editStage {
			case 1:
				prompt := "Edit message (Enter to continue, Esc to cancel)\n\n"
				raw := prompt + m.textInput.View()
				wrap := lipgloss.NewStyle().MaxWidth(m.width - 6).Render(raw)
				box := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder(), true).
					BorderForeground(lipgloss.Color("#5DAFF4")).
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#333333")).
					Padding(1, 2).
					Render(wrap)
				return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
			case 2:
				return "\n" + m.fileList.View() + "\n\n(Use ↑/↓, Enter to choose, Esc to cancel)"
			case 3:
				prompt := "Edit tags (comma-separated, leave blank) (Enter to save, Esc to cancel)\n\n"
				raw := prompt + m.textInput.View()
				wrap := lipgloss.NewStyle().MaxWidth(m.width - 6).Render(raw)
				box := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder(), true).
					BorderForeground(lipgloss.Color("#5DAFF4")).
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#333333")).
					Padding(1, 2).
					Render(wrap)
				return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
			}
		}
	}

	if m.confirmingDelete {
		item := m.notesList.Items()[m.deleteIndex].(NoteItem)

		var lines []string
		lines = append(lines, "Delete note:", "")
		lines = append(lines, fmt.Sprintf("\"%s\"", item.Message), "")

		if item.File != "" {
			lines = append(lines, fmt.Sprintf("File: %s:%d", item.File, item.Line))
		}

		if len(item.Tags) > 0 {
			lines = append(lines, fmt.Sprintf("Tags: %s", strings.Join(item.Tags, ", ")))
		}

		lines = append(lines, "", "Press Y/Enter to confirm, any other key to cancel")

		content := strings.Join(lines, "\n")

		modal := modalStyle(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	}

	return "\n" + m.notesList.View() + "\n\n(Use Ctrl+D to remove, Ctrl+E to edit, Ctrl+A to add, Ctrl+F to search, Ctrl+Q to quit)"
}
