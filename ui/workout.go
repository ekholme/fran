// package ui defines the tui used by the application
package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

type WorkoutUpdater interface {
	UpdateWorkout(ctx context.Context, w *fran.Workout) error
}

type WorkoutStore interface {
	WorkoutGetter
	WorkoutDeleter
	WorkoutCreator
	WorkoutUpdater
}

// detailFieldKind determines how a detail field's value is validated.
type detailFieldKind int

const (
	fieldText detailFieldKind = iota
	fieldNumeric
	fieldUnit
)

// detailField is one type-specific input in the create/edit form (e.g.
// Cardio's "Distance" field). The set of detail fields shown is rebuilt
// whenever the selected WorkoutType changes.
type detailField struct {
	label   string
	input   textinput.Model
	kind    detailFieldKind
	allowed []string // used when kind == fieldUnit
}

func newTextField(placeholder string) detailField {
	ti := textinput.New()
	ti.Placeholder = placeholder
	return detailField{label: placeholder, input: ti, kind: fieldText}
}

func newNumericField(placeholder string) detailField {
	ti := textinput.New()
	ti.Placeholder = placeholder
	return detailField{label: placeholder, input: ti, kind: fieldNumeric}
}

func newUnitField(placeholder string, allowed []string) detailField {
	ti := textinput.New()
	ti.Placeholder = placeholder
	return detailField{label: placeholder, input: ti, kind: fieldUnit, allowed: allowed}
}

// strengthFieldsPerExercise is the number of detailFields making up one
// exercise in a Strength workout's DetailFields (name, sets, reps, weight,
// weight unit).
const strengthFieldsPerExercise = 5

// strengthExerciseGroup returns one blank exercise's worth of fields, in
// the fixed order strengthFieldsPerExercise assumes.
func strengthExerciseGroup() []detailField {
	return []detailField{
		newTextField("Exercise Name"),
		newNumericField("Sets"),
		newNumericField("Reps"),
		newNumericField("Weight"),
		newUnitField("Weight Unit (lbs/kg)", []string{"lbs", "kg"}),
	}
}

// newDetailFields returns the empty, type-specific input fields for t.
// strengthCount controls how many exercise groups a Strength workout
// starts with (ignored for every other type); callers should pass at
// least 1.
func newDetailFields(t fran.WorkoutType, strengthCount int) []detailField {
	switch t {
	case fran.WorkoutTypeAMRAP:
		return []detailField{
			newNumericField("Time Cap (min)"),
			newNumericField("Rounds Completed"),
			newNumericField("Extra Reps"),
		}
	case fran.WorkoutTypeEMOM:
		return []detailField{
			newNumericField("Interval (sec)"),
			newNumericField("Duration (min)"),
		}
	case fran.WorkoutTypeCardio:
		return []detailField{
			newTextField("Modality (run/bike/row/...)"),
			newNumericField("Duration (min)"),
			newNumericField("Distance"),
			newUnitField("Distance Unit (mi/km)", []string{"mi", "km"}),
		}
	case fran.WorkoutTypeStrength:
		if strengthCount < 1 {
			strengthCount = 1
		}
		fields := make([]detailField, 0, strengthCount*strengthFieldsPerExercise)
		for i := 0; i < strengthCount; i++ {
			fields = append(fields, strengthExerciseGroup()...)
		}
		return fields
	default: // fran.WorkoutTypeOther
		return nil
	}
}

// newDetailFieldsFromWorkout returns w's type-specific input fields,
// pre-filled from its existing detail struct (for the edit flow), along
// with the number of Strength exercise groups it built (1 for every other
// type, or if w has no exercises yet).
func newDetailFieldsFromWorkout(w *fran.Workout) ([]detailField, int) {
	strengthCount := 1
	if w.WorkoutType == fran.WorkoutTypeStrength && w.Strength != nil && len(w.Strength.Exercises) > 0 {
		strengthCount = len(w.Strength.Exercises)
	}

	fields := newDetailFields(w.WorkoutType, strengthCount)

	switch w.WorkoutType {
	case fran.WorkoutTypeAMRAP:
		if w.AMRAP != nil {
			fields[0].input.SetValue(strconv.Itoa(w.AMRAP.TimeCapMinutes))
			fields[1].input.SetValue(strconv.Itoa(w.AMRAP.RoundsCompleted))
			fields[2].input.SetValue(strconv.Itoa(w.AMRAP.ExtraReps))
		}
	case fran.WorkoutTypeEMOM:
		if w.EMOM != nil {
			fields[0].input.SetValue(strconv.Itoa(w.EMOM.IntervalSeconds))
			fields[1].input.SetValue(strconv.Itoa(w.EMOM.DurationMinutes))
		}
	case fran.WorkoutTypeCardio:
		if w.Cardio != nil {
			fields[0].input.SetValue(w.Cardio.Modality)
			fields[1].input.SetValue(strconv.Itoa(w.Cardio.DurationMin))
			fields[2].input.SetValue(strconv.FormatFloat(w.Cardio.Distance, 'f', -1, 64))
			fields[3].input.SetValue(w.Cardio.DistanceUnit)
		}
	case fran.WorkoutTypeStrength:
		if w.Strength != nil {
			for i, ex := range w.Strength.Exercises {
				base := i * strengthFieldsPerExercise
				fields[base+0].input.SetValue(ex.Name)
				fields[base+1].input.SetValue(strconv.Itoa(ex.Sets))
				fields[base+2].input.SetValue(strconv.Itoa(ex.Reps))
				fields[base+3].input.SetValue(strconv.FormatFloat(ex.Weight, 'f', -1, 64))
				fields[base+4].input.SetValue(ex.WeightUnit)
			}
		}
	}

	return fields, strengthCount
}

// detailFieldValid reports whether f's current value is acceptable. An
// empty value is always valid (every detail field is optional).
func detailFieldValid(f detailField) bool {
	v := strings.TrimSpace(f.input.Value())
	if v == "" {
		return true
	}

	switch f.kind {
	case fieldNumeric:
		n, err := strconv.ParseFloat(v, 64)
		return err == nil && n >= 0
	case fieldUnit:
		for _, a := range f.allowed {
			if strings.EqualFold(v, a) {
				return true
			}
		}
		return false
	default: // fieldText
		return true
	}
}

func parseOptionalInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("must be a non-negative whole number")
	}
	return v, nil
}

func parseOptionalFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("must be a non-negative number")
	}
	return v, nil
}

func parseOptionalUnit(s string, allowed []string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, a := range allowed {
		if strings.EqualFold(s, a) {
			return a, nil
		}
	}
	return "", fmt.Errorf("must be one of %s", strings.Join(allowed, "/"))
}

// applyDetailFieldsToWorkout parses fields according to w.WorkoutType and
// attaches the resulting typed detail struct to w. Returns an error naming
// the offending field if any value fails validation.
func applyDetailFieldsToWorkout(w *fran.Workout, fields []detailField) error {
	switch w.WorkoutType {
	case fran.WorkoutTypeAMRAP:
		timeCap, err := parseOptionalInt(fields[0].input.Value())
		if err != nil {
			return fmt.Errorf("Time Cap: %w", err)
		}
		rounds, err := parseOptionalInt(fields[1].input.Value())
		if err != nil {
			return fmt.Errorf("Rounds Completed: %w", err)
		}
		extra, err := parseOptionalInt(fields[2].input.Value())
		if err != nil {
			return fmt.Errorf("Extra Reps: %w", err)
		}
		w.AMRAP = &fran.AMRAPDetails{TimeCapMinutes: timeCap, RoundsCompleted: rounds, ExtraReps: extra}

	case fran.WorkoutTypeEMOM:
		interval, err := parseOptionalInt(fields[0].input.Value())
		if err != nil {
			return fmt.Errorf("Interval: %w", err)
		}
		duration, err := parseOptionalInt(fields[1].input.Value())
		if err != nil {
			return fmt.Errorf("Duration: %w", err)
		}
		w.EMOM = &fran.EMOMDetails{IntervalSeconds: interval, DurationMinutes: duration}

	case fran.WorkoutTypeCardio:
		modality := strings.TrimSpace(fields[0].input.Value())
		duration, err := parseOptionalInt(fields[1].input.Value())
		if err != nil {
			return fmt.Errorf("Duration: %w", err)
		}
		distance, err := parseOptionalFloat(fields[2].input.Value())
		if err != nil {
			return fmt.Errorf("Distance: %w", err)
		}
		unit, err := parseOptionalUnit(fields[3].input.Value(), []string{"mi", "km"})
		if err != nil {
			return fmt.Errorf("Distance Unit: %w", err)
		}
		w.Cardio = &fran.CardioDetails{Modality: modality, DurationMin: duration, Distance: distance, DistanceUnit: unit}

	case fran.WorkoutTypeStrength:
		var exercises []fran.StrengthExercise
		for i := 0; i+strengthFieldsPerExercise <= len(fields); i += strengthFieldsPerExercise {
			group := fields[i : i+strengthFieldsPerExercise]
			exerciseNum := i/strengthFieldsPerExercise + 1

			name := strings.TrimSpace(group[0].input.Value())
			setsStr := strings.TrimSpace(group[1].input.Value())
			repsStr := strings.TrimSpace(group[2].input.Value())
			weightStr := strings.TrimSpace(group[3].input.Value())
			unitStr := strings.TrimSpace(group[4].input.Value())

			if name == "" && setsStr == "" && repsStr == "" && weightStr == "" && unitStr == "" {
				continue // skip a blank exercise slot
			}

			sets, err := parseOptionalInt(setsStr)
			if err != nil {
				return fmt.Errorf("Exercise %d Sets: %w", exerciseNum, err)
			}
			reps, err := parseOptionalInt(repsStr)
			if err != nil {
				return fmt.Errorf("Exercise %d Reps: %w", exerciseNum, err)
			}
			weight, err := parseOptionalFloat(weightStr)
			if err != nil {
				return fmt.Errorf("Exercise %d Weight: %w", exerciseNum, err)
			}
			unit, err := parseOptionalUnit(unitStr, []string{"lbs", "kg"})
			if err != nil {
				return fmt.Errorf("Exercise %d Weight Unit: %w", exerciseNum, err)
			}

			exercises = append(exercises, fran.StrengthExercise{
				Name:       name,
				Sets:       sets,
				Reps:       reps,
				Weight:     weight,
				WeightUnit: unit,
			})
		}
		w.Strength = &fran.StrengthDetails{Exercises: exercises}

	case fran.WorkoutTypeOther:
		// no structured fields
	}

	return nil
}

func nextWorkoutType(t fran.WorkoutType) fran.WorkoutType {
	for i, wt := range fran.WorkoutTypes {
		if wt == t {
			return fran.WorkoutTypes[(i+1)%len(fran.WorkoutTypes)]
		}
	}
	return fran.WorkoutTypes[0]
}

// renderWorkoutTypeDetails renders the type-specific fields of w for the
// read-only detail view. Returns a visible placeholder rather than
// panicking if w's WorkoutType claims structured data that isn't present.
func renderWorkoutTypeDetails(w *fran.Workout) string {
	const missing = "(missing structured data)"

	switch w.WorkoutType {
	case fran.WorkoutTypeAMRAP:
		if w.AMRAP == nil {
			return missing
		}
		return fmt.Sprintf("Time Cap: %d min | Rounds: %d | Extra Reps: %d", w.AMRAP.TimeCapMinutes, w.AMRAP.RoundsCompleted, w.AMRAP.ExtraReps)
	case fran.WorkoutTypeEMOM:
		if w.EMOM == nil {
			return missing
		}
		return fmt.Sprintf("Interval: %ds | Duration: %d min", w.EMOM.IntervalSeconds, w.EMOM.DurationMinutes)
	case fran.WorkoutTypeCardio:
		if w.Cardio == nil {
			return missing
		}
		return fmt.Sprintf("%s | Duration: %d min | Distance: %s %s", w.Cardio.Modality, w.Cardio.DurationMin, strconv.FormatFloat(w.Cardio.Distance, 'f', -1, 64), w.Cardio.DistanceUnit)
	case fran.WorkoutTypeStrength:
		if w.Strength == nil || len(w.Strength.Exercises) == 0 {
			return missing
		}
		lines := make([]string, len(w.Strength.Exercises))
		for i, ex := range w.Strength.Exercises {
			lines[i] = fmt.Sprintf("Exercise %d: %s — %d x %d @ %s %s", i+1, ex.Name, ex.Sets, ex.Reps, strconv.FormatFloat(ex.Weight, 'f', -1, 64), ex.WeightUnit)
		}
		return strings.Join(lines, "\n")
	default: // fran.WorkoutTypeOther
		return ""
	}
}

type WorkoutModelInputs struct {
	ShortDescriptionInput textinput.Model
	LongDescriptionInput  textarea.Model
	WorkoutDateInput      textinput.Model
	DetailFields          []detailField
}

type WorkoutModel struct {
	store                 WorkoutStore
	list                  list.Model
	inputs                WorkoutModelInputs
	inputFocusIndex       int
	state                 sessionState
	loading               bool
	listLength            int
	err                   error
	selectedWorkout       *fran.Workout
	allWorkouts           []*fran.Workout
	isEditing             bool
	editingID             int
	confirmDelete         bool
	WorkoutType           fran.WorkoutType
	strengthExerciseCount int
	searchInput           textinput.Model
	searching             bool
	searchActive          bool
}

func NewWorkoutModel(s WorkoutStore, listLength int) WorkoutModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)

	l.Title = workoutListTitle
	// The list's own built-in filtering is unused — search is handled by
	// searchInput below and backed by a real database query
	// (GetWorkoutsByShortDesc) rather than filtering the already-loaded page.
	l.SetFilteringEnabled(false)

	// add entries in the help keybinds for actions not obvious from the list itself
	createWorkoutKey := key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "new workout"),
	)
	searchKey := key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	)

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			createWorkoutKey,
			searchKey,
		}
	}

	// search input init
	si := textinput.New()
	si.Prompt = "Search: "
	si.Placeholder = "short description..."

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
		store:                 s,
		list:                  l,
		inputs:                wmi,
		state:                 stateWorkoutList,
		loading:               true,
		listLength:            listLength,
		WorkoutType:           fran.WorkoutTypeOther,
		strengthExerciseCount: 1,
		searchInput:           si,
	}
}

const workoutListTitle = "Select a Workout"

type workoutsLoadedMsg struct {
	workouts []*fran.Workout
}

type workoutsSearchedMsg struct {
	workouts []*fran.Workout
	query    string
}

type workoutCreatedMsg struct{}

type workoutUpdatedMsg struct{}

type workoutDeletedMsg struct{}

type workoutLoadedMsg struct {
	workout *fran.Workout
}

type workoutItem struct {
	fran.Workout
}

func (i workoutItem) Title() string       { return i.ShortDescription }
func (i workoutItem) Description() string { return i.WorkoutType.Label() + " · " + i.WorkoutDate }

// FilterValue satisfies list.Item; unused since the list's own built-in
// filtering is disabled in favor of the database-backed search (see
// searchInput/searchWorkoutsCmd).
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

func searchWorkoutsCmd(s WorkoutStore, query string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		workouts, err := s.GetWorkoutsByShortDesc(ctx, query)
		if err != nil {
			return err
		}

		return workoutsSearchedMsg{workouts: workouts, query: query}
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

func updateWorkoutCmd(s WorkoutStore, w *fran.Workout) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := s.UpdateWorkout(ctx, w); err != nil {
			return err
		}
		return workoutUpdatedMsg{}
	}
}

func deleteWorkoutCmd(s WorkoutStore, id int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := s.DeleteWorkout(ctx, id); err != nil {
			return err
		}
		return workoutDeletedMsg{}
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
		m.searchActive = false
		m.list.Title = workoutListTitle
		m.allWorkouts = msg.workouts
		items := make([]list.Item, len(msg.workouts))
		for i, w := range msg.workouts {
			items[i] = workoutItem{*w}
		}
		cmd = m.list.SetItems(items)
		return m, cmd

	case workoutsSearchedMsg:
		m.loading = false
		m.searchActive = true
		m.list.Title = fmt.Sprintf("Search: %q (%d result(s), esc to clear)", msg.query, len(msg.workouts))
		m.allWorkouts = msg.workouts
		items := make([]list.Item, len(msg.workouts))
		for i, w := range msg.workouts {
			items[i] = workoutItem{*w}
		}
		cmd = m.list.SetItems(items)
		return m, cmd

	case workoutCreatedMsg, workoutUpdatedMsg:
		// reset form and go back to workout list state
		m.state = stateWorkoutList
		m.loading = true
		m.isEditing = false
		m.editingID = 0
		m.WorkoutType = fran.WorkoutTypeOther
		m.strengthExerciseCount = 1
		m.inputs.DetailFields = nil
		m.inputs.ShortDescriptionInput.Reset()
		m.inputs.LongDescriptionInput.Reset()
		m.inputs.WorkoutDateInput.Reset()
		return m, fetchLatestWorkoutsCmd(m.store, m.listLength)

	case workoutDeletedMsg:
		m.state = stateWorkoutList
		m.loading = true
		m.selectedWorkout = nil
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
		if m.searching {
			return lipgloss.JoinVertical(lipgloss.Left, m.searchInput.View(), m.list.View())
		}
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
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true)

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

	hasDetailError := false
	detailViews := make([]string, 0, len(m.inputs.DetailFields))
	for i, f := range m.inputs.DetailFields {
		if m.WorkoutType == fran.WorkoutTypeStrength && i%strengthFieldsPerExercise == 0 {
			detailViews = append(detailViews, typeStyle.Render(fmt.Sprintf("Exercise %d:", i/strengthFieldsPerExercise+1)))
		}
		view := f.input.View()
		switch {
		case strings.TrimSpace(f.input.Value()) == "":
			detailViews = append(detailViews, view)
		case detailFieldValid(f):
			detailViews = append(detailViews, validStyle.Render(view))
		default:
			detailViews = append(detailViews, errorStyle.Render(view))
			hasDetailError = true
		}
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
	} else if hasDetailError {
		validationMsg = errorStyle.Render("\nOne or more fields below are invalid")
	}

	title := "Create New Workout"
	if m.isEditing {
		title = "Edit Workout"
	}

	lines := []string{
		titleStyle.Render(title),
		typeStyle.Render("Type: "+m.WorkoutType.Label()) + "  (ctrl+t to change)",
		shortDescView,
		m.inputs.LongDescriptionInput.View(),
		dateView,
	}
	lines = append(lines, detailViews...)

	saveHelp := "\nCtrl+S to save, Esc to cancel"
	if m.WorkoutType == fran.WorkoutTypeStrength {
		saveHelp = "\nCtrl+A: add exercise, Ctrl+R: remove last  |  " + strings.TrimPrefix(saveHelp, "\n")
	}
	lines = append(lines, validationMsg, saveHelp)

	formContent := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return formBorder.Render(formContent)
}

func (m WorkoutModel) viewWorkout() string {
	if m.selectedWorkout == nil {
		return "No workout selected."
	}
	w := m.selectedWorkout

	prevNext := ""
	for i, ww := range m.allWorkouts {
		if ww.ID == w.ID && len(m.allWorkouts) > 1 {
			if i > 0 {
				prevNext += "\n← Previous: " + m.allWorkouts[i-1].ShortDescription
			}
			if i < len(m.allWorkouts)-1 {
				prevNext += "\n→ Next: " + m.allWorkouts[i+1].ShortDescription
			}
			break
		}
	}

	help := "\n\nPress e to edit, d to delete, Esc or q to return to list"
	if m.confirmDelete {
		help = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("\n\nDelete this workout? (y/n)")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"\nWorkout Details\n",
		"Short Desc: "+w.ShortDescription,
		"Type: "+w.WorkoutType.Label(),
		"Date: "+w.WorkoutDate,
		renderWorkoutTypeDetails(w),
		"\nDescription:",
		w.LongDescription,
		prevNext,
		help,
	)
}

func (m WorkoutModel) updateWorkoutList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.updateSearchInput(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "n":
			m.state = stateCreateWorkout
			m.isEditing = false
			m.editingID = 0
			m.WorkoutType = fran.WorkoutTypeOther
			m.strengthExerciseCount = 1
			m.inputs.DetailFields = nil
			m.inputFocusIndex = 0
			m.inputs.ShortDescriptionInput.Reset()
			m.inputs.LongDescriptionInput.Reset()
			m.inputs.WorkoutDateInput.Reset()
			m.inputs.ShortDescriptionInput.Focus()
			return m, nil
		case "/":
			m.searching = true
			m.searchInput.Reset()
			m.searchInput.Focus()
			return m, nil
		case "esc":
			if m.searchActive {
				m.searchActive = false
				m.loading = true
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

// updateSearchInput handles key input while the search box (opened with
// "/") is focused, keeping it entirely separate from the list's own
// (unused) built-in filtering. Enter runs a real GetWorkoutsByShortDesc
// query against the database rather than filtering the already-loaded
// page; an empty query on Enter is treated as "show all" instead.
func (m WorkoutModel) updateSearchInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.searching = false
			m.searchInput.Blur()
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.searchInput.Value())
			m.searching = false
			m.searchInput.Blur()
			m.loading = true
			if query == "" {
				m.searchActive = false
				return m, fetchLatestWorkoutsCmd(m.store, m.listLength)
			}
			return m, searchWorkoutsCmd(m.store, query)
		}
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m WorkoutModel) updateWorkoutForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.state = stateWorkoutList
			m.isEditing = false
			m.editingID = 0
			m.WorkoutType = fran.WorkoutTypeOther
			m.strengthExerciseCount = 1
			m.inputs.DetailFields = nil
			m.inputs.ShortDescriptionInput.Reset()
			m.inputs.LongDescriptionInput.Reset()
			m.inputs.WorkoutDateInput.Reset()
			return m, nil
		case "ctrl+t":
			m.WorkoutType = nextWorkoutType(m.WorkoutType)
			m.strengthExerciseCount = 1
			m.inputs.DetailFields = newDetailFields(m.WorkoutType, m.strengthExerciseCount)
			m.inputFocusIndex = 0
			m.inputs.ShortDescriptionInput.Focus()
			m.inputs.LongDescriptionInput.Blur()
			m.inputs.WorkoutDateInput.Blur()
			return m, nil
		case "ctrl+a":
			if m.WorkoutType != fran.WorkoutTypeStrength {
				return m, nil
			}
			m.inputs.DetailFields = append(m.inputs.DetailFields, strengthExerciseGroup()...)
			m.strengthExerciseCount++
			return m, nil
		case "ctrl+r":
			if m.WorkoutType != fran.WorkoutTypeStrength || m.strengthExerciseCount <= 1 {
				return m, nil
			}
			m.inputs.DetailFields = m.inputs.DetailFields[:len(m.inputs.DetailFields)-strengthFieldsPerExercise]
			m.strengthExerciseCount--
			if m.inputFocusIndex >= 3+len(m.inputs.DetailFields) {
				m.inputFocusIndex = 0
				m.inputs.ShortDescriptionInput.Focus()
				m.inputs.LongDescriptionInput.Blur()
				m.inputs.WorkoutDateInput.Blur()
			}
			return m, nil
		case "tab":
			total := 3 + len(m.inputs.DetailFields)
			m.inputFocusIndex = (m.inputFocusIndex + 1) % total
			m.inputs.ShortDescriptionInput.Blur()
			m.inputs.LongDescriptionInput.Blur()
			m.inputs.WorkoutDateInput.Blur()
			for i := range m.inputs.DetailFields {
				m.inputs.DetailFields[i].input.Blur()
			}
			switch {
			case m.inputFocusIndex == 0:
				m.inputs.ShortDescriptionInput.Focus()
			case m.inputFocusIndex == 1:
				m.inputs.LongDescriptionInput.Focus()
			case m.inputFocusIndex == 2:
				m.inputs.WorkoutDateInput.Focus()
			default:
				m.inputs.DetailFields[m.inputFocusIndex-3].input.Focus()
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
				WorkoutType:      m.WorkoutType,
			}

			if err := applyDetailFieldsToWorkout(w, m.inputs.DetailFields); err != nil {
				m.err = err
				return m, nil
			}

			if m.isEditing {
				w.ID = m.editingID
				return m, updateWorkoutCmd(m.store, w)
			}
			return m, createWorkoutCmd(m.store, w)
		}

	case tea.WindowSizeMsg:
		m.inputs.LongDescriptionInput.SetWidth(msg.Width)
		m.inputs.LongDescriptionInput.SetHeight(msg.Height - 10)
	}

	// Update the focused input
	switch {
	case m.inputFocusIndex == 0:
		var newTextinput textinput.Model
		newTextinput = m.inputs.ShortDescriptionInput
		newTextinput, cmd = newTextinput.Update(msg)
		m.inputs.ShortDescriptionInput = newTextinput
	case m.inputFocusIndex == 1:
		var newTextarea textarea.Model
		newTextarea = m.inputs.LongDescriptionInput
		newTextarea, cmd = newTextarea.Update(msg)
		m.inputs.LongDescriptionInput = newTextarea
	case m.inputFocusIndex == 2:
		var newTextinput textinput.Model
		newTextinput = m.inputs.WorkoutDateInput
		newTextinput, cmd = newTextinput.Update(msg)
		m.inputs.WorkoutDateInput = newTextinput
	case m.inputFocusIndex-3 < len(m.inputs.DetailFields):
		idx := m.inputFocusIndex - 3
		var newTextinput textinput.Model
		newTextinput = m.inputs.DetailFields[idx].input
		newTextinput, cmd = newTextinput.Update(msg)
		m.inputs.DetailFields[idx].input = newTextinput
	}

	return m, cmd
}

func (m WorkoutModel) updateViewWorkout(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmDelete {
			switch msg.String() {
			case "y", "Y":
				m.confirmDelete = false
				return m, deleteWorkoutCmd(m.store, m.selectedWorkout.ID)
			case "n", "N", "esc":
				m.confirmDelete = false
			}
			return m, nil
		}

		switch msg.String() {
		case "esc", "q":
			m.state = stateWorkoutList
			m.loading = false
			return m, nil
		case "e":
			w := m.selectedWorkout
			m.isEditing = true
			m.editingID = w.ID
			m.WorkoutType = w.WorkoutType
			m.inputs.ShortDescriptionInput.SetValue(w.ShortDescription)
			m.inputs.LongDescriptionInput.SetValue(w.LongDescription)
			m.inputs.WorkoutDateInput.SetValue(w.WorkoutDate)
			m.inputs.DetailFields, m.strengthExerciseCount = newDetailFieldsFromWorkout(w)
			m.inputFocusIndex = 0
			m.inputs.ShortDescriptionInput.Focus()
			m.inputs.LongDescriptionInput.Blur()
			m.inputs.WorkoutDateInput.Blur()
			m.state = stateCreateWorkout
			return m, nil
		case "d":
			m.confirmDelete = true
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
