// package ui defines the tui used by the application
package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ekholme/fran"
)

type sessionState int

const (
	stateWorkoutList sessionState = iota
	stateCreateWorkout
	stateViewWorkout
)

type WorkoutGetter interface {
	GetLatestWorkouts(ctx context.Context, n int) ([]*fran.Workout, error)
	GetWorkoutByID(ctx context.Context, id int) (*fran.Workout, error)
	GetWorkoutByDate(ctx context.Context, date string) (*fran.Workout, error)
	GetWorkoutsByShortDesc(ctx context.Context, desc string) ([]*fran.Workout, error)
}

type WorkoutCreator interface {
	CreateWorkout(ctx context.Context, w *fran.Workout) (int, error)
}

type WorkoutDeleter interface {
	DeleteWorkout(ctx context.Context, id int) error
}

type WorkoutStore interface {
	WorkoutGetter
	WorkoutDeleter
	WorkoutCreator
}

type WorkoutModelInputs struct {
	ShortDescriptionInput textinput.Model
	LongDescriptionInput  textarea.Model
	WorkoutDateInput      textinput.Model
}

type WorkoutModel struct {
	store           WorkoutStore
	list            list.Model
	inputs          WorkoutModelInputs
	inputFocusIndex int
	state           sessionState
	loading         bool
	listLength      int
	err             error
	selectedWorkout *fran.Workout
	allWorkouts     []*fran.Workout
}

func NewWorkoutModel(s WorkoutStore, listLength int) WorkoutModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)

	l.Title = "Select a Workout"
	l.SetFilteringEnabled(true)
	l.SetShowFilter(true)

	// add an entry in the help keybinds to create a new workout
	createWorkoutKey := key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new workout"),
	)

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			createWorkoutKey,
		}
	}

	// short description init
	sdi := textinput.New()
	sdi.Placeholder = "Short Workout Description (e.g. KB ABC)"
	sdi.Focus()

	// long description init
	ldi := textarea.New()
	ldi.Placeholder = "Long Description (e.g. 20 min AMRAP...)"

	// workout date init
	wdi := textinput.New()
	wdi.Placeholder = "Workout Date (YYYY-MM-DD)"
	wdi.CharLimit = 10

	wmi := WorkoutModelInputs{
		ShortDescriptionInput: sdi,
		LongDescriptionInput:  ldi,
		WorkoutDateInput:      wdi,
	}

	return WorkoutModel{
		store:      s,
		list:       l,
		inputs:     wmi,
		state:      stateWorkoutList,
		loading:    true,
		listLength: listLength,
	}
}

type workoutsLoadedMsg struct {
	workouts []*fran.Workout
}

type workoutCreatedMsg struct{}

type workoutLoadedMsg struct {
	workout *fran.Workout
}

type workoutItem struct {
	fran.Workout
}

func (i workoutItem) Title() string       { return i.ShortDescription }
func (i workoutItem) Description() string { return i.WorkoutDate }
func (i workoutItem) FilterValue() string { return i.LongDescription }

func fetchLatestWorkoutsCmd(s WorkoutStore, n int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		workouts, err := s.GetLatestWorkouts(ctx, n)
		if err != nil {
			return err
		}

		return workoutsLoadedMsg{workouts}
	}
}

func createWorkoutCmd(s WorkoutStore, w *fran.Workout) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := s.CreateWorkout(ctx, w)
		if err != nil {
			return err
		}
		return workoutCreatedMsg{}
	}
}

func fetchWorkoutCmd(s WorkoutStore, id int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		w, err := s.GetWorkoutByID(ctx, id)
		if err != nil {
			return err
		}
		return workoutLoadedMsg{w}
	}
}

func (m WorkoutModel) Init() tea.Cmd {
	return fetchLatestWorkoutsCmd(m.store, m.listLength)
}

func (m WorkoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case error:
		m.loading = false
		m.err = msg
		return m, nil

	case workoutsLoadedMsg:
		m.loading = false
		m.allWorkouts = msg.workouts
		items := make([]list.Item, len(msg.workouts))
		for i, w := range msg.workouts {
			items[i] = workoutItem{*w}
		}
		cmd = m.list.SetItems(items)
		return m, cmd

	case workoutCreatedMsg:
		// reset form and go back to workout list state
		m.state = stateWorkoutList
		m.loading = true
		m.inputs.ShortDescriptionInput.Reset()
		m.inputs.LongDescriptionInput.Reset()
		m.inputs.WorkoutDateInput.Reset()
		return m, fetchLatestWorkoutsCmd(m.store, m.listLength)

	case workoutLoadedMsg:
		m.loading = false
		m.selectedWorkout = msg.workout

	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		switch m.state {
		case stateWorkoutList:
			return m.updateWorkoutList(msg)
		case stateCreateWorkout:
			return m.updateWorkoutForm(msg)
		}
	default:
		switch m.state {
		case stateWorkoutList:
			return m.updateWorkoutList(msg)
		case stateCreateWorkout:
			return m.updateWorkoutForm(msg)
		case stateViewWorkout:
			return m.updateViewWorkout(msg)
		}
	}
	return m, cmd
}

func (m WorkoutModel) View() string {
	if m.err != nil {
		errMsg := m.err.Error()
		return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(errMsg + "\n\nPress q to quit.")
	}

	if m.loading && m.state != stateWorkoutList {
		return "Loading..."
	}

	switch m.state {
	case stateWorkoutList:
		return m.list.View()
	case stateCreateWorkout:
		return m.viewCreateWorkout()
	case stateViewWorkout:
		return m.viewWorkout()
	}
	return ""
}

func isDateValid(date string) bool {
	if date == "" {
		return false
	}
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

func (m WorkoutModel) viewCreateWorkout() string {
	shortDesc := m.inputs.ShortDescriptionInput.Value()
	date := m.inputs.WorkoutDateInput.Value()

	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	validStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

	var shortDescView string
	if shortDesc == "" {
		shortDescView = errorStyle.Render(m.inputs.ShortDescriptionInput.View())
	} else {
		shortDescView = validStyle.Render(m.inputs.ShortDescriptionInput.View())
	}

	var dateView string
	if date != "" && !isDateValid(date) {
		dateView = errorStyle.Render(m.inputs.WorkoutDateInput.View())
	} else if date == "" {
		dateView = m.inputs.WorkoutDateInput.View()
	} else {
		dateView = validStyle.Render(m.inputs.WorkoutDateInput.View())
	}

	formBorder := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("69")).
		MarginBottom(1)

	validationMsg := ""
	if date != "" && !isDateValid(date) {
		validationMsg = errorStyle.Render("\nInvalid Date Format (Expected YYYY-MM-DD)")
	} else if shortDesc == "" {
		validationMsg = errorStyle.Render("\nShort description is required")
	}

	formContent := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Create New Workout"),
		shortDescView,
		m.inputs.LongDescriptionInput.View(),
		dateView,
		validationMsg,
		"\nCtrl+S to save, Esc to cancel",
	)

	return formBorder.Render(formContent)
}

func (m WorkoutModel) viewWorkout() string {
	if m.selectedWorkout == nil {
		return "No workout selected."
	}

	prevNext := ""
	for i, w := range m.allWorkouts {
		if w.ID == m.selectedWorkout.ID && len(m.allWorkouts) > 1 {
			if i > 0 {
				prevNext += "\n← Previous: " + m.allWorkouts[i-1].ShortDescription
			}
			if i < len(m.allWorkouts)-1 {
				prevNext += "\n→ Next: " + m.allWorkouts[i+1].ShortDescription
			}
			break
		}
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"\nWorkout Details\n",
		"Short Desc: "+m.selectedWorkout.ShortDescription,
		"Date: "+m.selectedWorkout.WorkoutDate,
		"\nDescription:",
		m.selectedWorkout.LongDescription,
		prevNext,
		"\n\nPress Esc or q to return to list",
	)
}

func (m WorkoutModel) updateWorkoutList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "n":
			m.state = stateCreateWorkout
			m.inputFocusIndex = 0
			m.inputs.ShortDescriptionInput.Focus()
			return m, nil
		case "/":
			m.list.FilterInput.Focus()
			return m, nil
		case "esc":
			if m.list.FilteringEnabled() && m.list.FilterInput.Focused() {
				m.list.ResetFilter()
				m.list.SetFilterText("")
				return m, fetchLatestWorkoutsCmd(m.store, m.listLength)
			}
		case "enter":
			selected := m.list.SelectedItem().(workoutItem)
			m.selectedWorkout = &selected.Workout
			m.state = stateViewWorkout
			m.loading = true
			return m, fetchWorkoutCmd(m.store, m.selectedWorkout.ID)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m WorkoutModel) updateWorkoutForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = stateWorkoutList
			return m, nil
		case "tab":
			m.inputFocusIndex = (m.inputFocusIndex + 1) % 3
			m.inputs.ShortDescriptionInput.Blur()
			m.inputs.LongDescriptionInput.Blur()
			m.inputs.WorkoutDateInput.Blur()
			switch m.inputFocusIndex {
			case 0:
				m.inputs.ShortDescriptionInput.Focus()
			case 1:
				m.inputs.LongDescriptionInput.Focus()
			case 2:
				m.inputs.WorkoutDateInput.Focus()
			}
		case "ctrl+s":
			shortDesc := m.inputs.ShortDescriptionInput.Value()
			if shortDesc == "" {
				m.err = fmt.Errorf("short description is required")
				return m, nil
			}

			date := m.inputs.WorkoutDateInput.Value()
			if date != "" && !isDateValid(date) {
				m.err = fmt.Errorf("invalid date format")
				return m, nil
			}

			w := &fran.Workout{
				ShortDescription: shortDesc,
				LongDescription:  m.inputs.LongDescriptionInput.Value(),
				WorkoutDate:      date,
			}
			return m, createWorkoutCmd(m.store, w)
		}

	case tea.WindowSizeMsg:
		m.inputs.LongDescriptionInput.SetWidth(msg.Width)
		m.inputs.LongDescriptionInput.SetHeight(msg.Height - 10)
	}

	// Update the focused input
	switch m.inputFocusIndex {
	case 0:
		var newTextinput textinput.Model
		newTextinput = m.inputs.ShortDescriptionInput
		newTextinput, cmd = newTextinput.Update(msg)
		m.inputs.ShortDescriptionInput = newTextinput
	case 1:
		var newTextarea textarea.Model
		newTextarea = m.inputs.LongDescriptionInput
		newTextarea, cmd = newTextarea.Update(msg)
		m.inputs.LongDescriptionInput = newTextarea
	case 2:
		var newTextinput textinput.Model
		newTextinput = m.inputs.WorkoutDateInput
		newTextinput, cmd = newTextinput.Update(msg)
		m.inputs.WorkoutDateInput = newTextinput
	}

	return m, cmd
}

func (m WorkoutModel) updateViewWorkout(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.state = stateWorkoutList
			m.loading = false
			return m, nil
		case "left", "h":
			for i, w := range m.allWorkouts {
				if w.ID == m.selectedWorkout.ID && i > 0 {
					m.selectedWorkout = m.allWorkouts[i-1]
					return m, fetchWorkoutCmd(m.store, m.selectedWorkout.ID)
				}
			}
		case "right", "l":
			for i, w := range m.allWorkouts {
				if w.ID == m.selectedWorkout.ID && i < len(m.allWorkouts)-1 {
					m.selectedWorkout = m.allWorkouts[i+1]
					return m, fetchWorkoutCmd(m.store, m.selectedWorkout.ID)
				}
			}
		}
	}
	return m, nil
}
