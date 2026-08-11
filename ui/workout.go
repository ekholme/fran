// package ui defines the tui used by the application
package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
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
	GetWorkoutbyID(ctx context.Context, id int) (*fran.Workout, error)
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
}

func NewWorkoutModel(s WorkoutStore, listLength int) WorkoutModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)

	l.Title = "Select a Workout"

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
		w, err := s.GetWorkoutbyID(ctx, id)
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
		items := make([]list.Item, len(msg.workouts))
		for i, w := range msg.workouts {
			items[i] = workoutItem{*w}
		}
		m.list.SetItems(items)

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
		return lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("Error: " + m.err.Error() + "\n\nPress q to quit.")
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

func (m WorkoutModel) viewCreateWorkout() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		"\nCreate New Workout\n",
		m.inputs.ShortDescriptionInput.View(),
		m.inputs.LongDescriptionInput.View(),
		m.inputs.WorkoutDateInput.View(),
		"\nCtrl+S to save, Esc to cancel",
	)
}

func (m WorkoutModel) viewWorkout() string {
	if m.selectedWorkout == nil {
		return "No workout selected."
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		"\nWorkout Details\n",
		"Short Desc: "+m.selectedWorkout.ShortDescription,
		"Date: "+m.selectedWorkout.WorkoutDate,
		"\nDescription:",
		m.selectedWorkout.LongDescription,
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
			w := &fran.Workout{
				ShortDescription: m.inputs.ShortDescriptionInput.Value(),
				LongDescription:  m.inputs.LongDescriptionInput.Value(),
				WorkoutDate:      m.inputs.WorkoutDateInput.Value(),
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
		}
	}
	return m, nil
}
