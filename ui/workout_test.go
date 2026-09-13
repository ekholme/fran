package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ekholme/fran"
)

// fakeStore is an in-memory WorkoutStore for testing WorkoutModel.Update
// without a real database.
type fakeStore struct {
	workouts []*fran.Workout
	nextID   int

	getLatestErr error
	createErr    error
	updateErr    error
	deleteErr    error
	searchErr    error

	created []*fran.Workout
	updated []*fran.Workout
	deleted []int
}

func newFakeStore() *fakeStore {
	return &fakeStore{nextID: 1}
}

func (f *fakeStore) findByID(id int) *fran.Workout {
	for _, w := range f.workouts {
		if w.ID == id {
			return w
		}
	}
	return nil
}

func (f *fakeStore) GetLatestWorkouts(ctx context.Context, n int) ([]*fran.Workout, error) {
	if f.getLatestErr != nil {
		return nil, f.getLatestErr
	}
	return f.workouts, nil
}

func (f *fakeStore) GetWorkoutByID(ctx context.Context, id int) (*fran.Workout, error) {
	if w := f.findByID(id); w != nil {
		return w, nil
	}
	return nil, errors.New("workout not found")
}

func (f *fakeStore) GetWorkoutByDate(ctx context.Context, date string) (*fran.Workout, error) {
	for _, w := range f.workouts {
		if w.WorkoutDate == date {
			return w, nil
		}
	}
	return nil, errors.New("workout not found")
}

func (f *fakeStore) GetWorkoutsByShortDesc(ctx context.Context, desc string) ([]*fran.Workout, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	var matches []*fran.Workout
	for _, w := range f.workouts {
		if strings.Contains(w.ShortDescription, desc) {
			matches = append(matches, w)
		}
	}
	return matches, nil
}

func (f *fakeStore) CreateWorkout(ctx context.Context, w *fran.Workout) (int, error) {
	if f.createErr != nil {
		return 0, f.createErr
	}
	cp := *w
	cp.ID = f.nextID
	f.nextID++
	f.workouts = append(f.workouts, &cp)
	f.created = append(f.created, &cp)
	return cp.ID, nil
}

func (f *fakeStore) UpdateWorkout(ctx context.Context, w *fran.Workout) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	for i, existing := range f.workouts {
		if existing.ID == w.ID {
			f.workouts[i] = w
		}
	}
	f.updated = append(f.updated, w)
	return nil
}

func (f *fakeStore) DeleteWorkout(ctx context.Context, id int) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for i, w := range f.workouts {
		if w.ID == id {
			f.workouts = append(f.workouts[:i], f.workouts[i+1:]...)
			break
		}
	}
	f.deleted = append(f.deleted, id)
	return nil
}

// keyMsg builds the tea.KeyMsg that produces the given msg.String(), for
// every key this package actually binds.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+t":
		return tea.KeyMsg{Type: tea.KeyCtrlT}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// isDomainMsg reports whether msg is one of this app's own async-result
// message types, as opposed to a bubbles-internal command (e.g. the
// textinput cursor's blink ticker, which is intentionally unbounded and
// must never be chased).
func isDomainMsg(msg tea.Msg) bool {
	switch msg.(type) {
	case workoutsLoadedMsg, workoutsSearchedMsg, workoutCreatedMsg, workoutUpdatedMsg, workoutDeletedMsg, workoutLoadedMsg, error:
		return true
	default:
		return false
	}
}

// execCmd runs cmd and reports its result, but only if it resolves
// quickly. This app's own commands (create/update/delete/fetch) call the
// in-memory fakeStore synchronously and return almost instantly; bubbles'
// internal commands (notably the textinput cursor's blink ticker) block
// on a real timer for hundreds of milliseconds by design and must never
// be waited on. ok is false if cmd is nil or doesn't resolve in time.
func execCmd(cmd tea.Cmd) (msg tea.Msg, ok bool) {
	if cmd == nil {
		return nil, false
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	select {
	case msg := <-done:
		return msg, true
	case <-time.After(20 * time.Millisecond):
		return nil, false
	}
}

// send feeds msg through m.Update, then keeps feeding the result through
// Update again as long as the returned tea.Cmd resolves quickly to one of
// this app's own domain messages (see isDomainMsg and execCmd) — that's
// the only kind of chaining this app's commands ever do, and it always
// terminates. Any other command (notably a cursor blink tick) is left to
// resolve on its own time and simply not chased.
func send(t *testing.T, m WorkoutModel, msg tea.Msg) WorkoutModel {
	t.Helper()

	for i := 0; i < 10; i++ {
		newModel, cmd := m.Update(msg)
		m = newModel.(WorkoutModel)

		next, ok := execCmd(cmd)
		if !ok || !isDomainMsg(next) {
			return m
		}
		msg = next
	}

	t.Fatalf("send: exceeded max domain-message chain depth (possible infinite loop) for msg %#v", msg)
	return m
}

func sendKey(t *testing.T, m WorkoutModel, key string) WorkoutModel {
	t.Helper()
	return send(t, m, keyMsg(key))
}

func typeText(t *testing.T, m WorkoutModel, s string) WorkoutModel {
	t.Helper()
	return send(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

// newReadyModel returns a WorkoutModel that has already loaded its
// workout list from store, mirroring the state the real app is in once
// Init's command resolves.
func newReadyModel(t *testing.T, store WorkoutStore) WorkoutModel {
	t.Helper()

	m := NewWorkoutModel(store, 50)
	cmd := m.Init()
	return send(t, m, cmd())
}

func TestNewWorkoutModel_Defaults(t *testing.T) {
	m := NewWorkoutModel(newFakeStore(), 50)

	if m.state != stateWorkoutList {
		t.Errorf("state = %v, want stateWorkoutList", m.state)
	}
	if m.WorkoutType != fran.WorkoutTypeOther {
		t.Errorf("WorkoutType = %v, want WorkoutTypeOther", m.WorkoutType)
	}
	if m.strengthExerciseCount != 1 {
		t.Errorf("strengthExerciseCount = %d, want 1", m.strengthExerciseCount)
	}
}

func TestCreateWorkout_HappyPath(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	if m.state != stateCreateWorkout {
		t.Fatalf("state after 'n' = %v, want stateCreateWorkout", m.state)
	}

	m = typeText(t, m, "KB ABC")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "20 min AMRAP notes")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "2026-09-12")

	m = sendKey(t, m, "ctrl+s")

	if m.err != nil {
		t.Fatalf("unexpected error after save: %v", m.err)
	}
	if m.state != stateWorkoutList {
		t.Fatalf("state after save = %v, want stateWorkoutList", m.state)
	}
	if len(store.created) != 1 {
		t.Fatalf("store.created has %d entries, want 1", len(store.created))
	}

	got := store.created[0]
	if got.ShortDescription != "KB ABC" || got.WorkoutDate != "2026-09-12" || got.WorkoutType != fran.WorkoutTypeOther {
		t.Errorf("created workout = %+v, want ShortDescription=KB ABC WorkoutDate=2026-09-12 WorkoutType=other", got)
	}
}

func TestCreateWorkout_RequiresShortDescription(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	m = sendKey(t, m, "ctrl+s")

	if m.err == nil {
		t.Fatal("expected an error when saving without a short description, got nil")
	}
	if m.state != stateCreateWorkout {
		t.Errorf("state = %v, want stateCreateWorkout (save should be blocked)", m.state)
	}
	if len(store.created) != 0 {
		t.Errorf("store.created has %d entries, want 0", len(store.created))
	}
}

func TestCreateWorkout_RequiresValidDate(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	m = typeText(t, m, "KB ABC")
	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "not-a-date")

	m = sendKey(t, m, "ctrl+s")

	if m.err == nil {
		t.Fatal("expected an error when saving with an invalid date, got nil")
	}
	if len(store.created) != 0 {
		t.Errorf("store.created has %d entries, want 0", len(store.created))
	}
}

func TestWorkoutTypeCycling_PreservesFixedFieldsAndRebuildsDetailFields(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	m = typeText(t, m, "KB ABC")

	wantOrder := []fran.WorkoutType{
		fran.WorkoutTypeAMRAP,
		fran.WorkoutTypeEMOM,
		fran.WorkoutTypeCardio,
		fran.WorkoutTypeStrength,
		fran.WorkoutTypeOther,
	}
	wantFieldCounts := map[fran.WorkoutType]int{
		fran.WorkoutTypeAMRAP:    3,
		fran.WorkoutTypeEMOM:     2,
		fran.WorkoutTypeCardio:   4,
		fran.WorkoutTypeStrength: 5,
		fran.WorkoutTypeOther:    0,
	}

	for _, want := range wantOrder {
		m = sendKey(t, m, "ctrl+t")
		if m.WorkoutType != want {
			t.Fatalf("WorkoutType = %v, want %v", m.WorkoutType, want)
		}
		if got := len(m.inputs.DetailFields); got != wantFieldCounts[want] {
			t.Errorf("type %v: len(DetailFields) = %d, want %d", want, got, wantFieldCounts[want])
		}
	}

	if m.inputs.ShortDescriptionInput.Value() != "KB ABC" {
		t.Errorf("ShortDescriptionInput = %q, want %q (should survive type cycling)", m.inputs.ShortDescriptionInput.Value(), "KB ABC")
	}
}

func TestCreateWorkout_AMRAPDetails(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	m = typeText(t, m, "20 min AMRAP")
	m = sendKey(t, m, "ctrl+t") // -> AMRAP

	m = sendKey(t, m, "tab") // short desc -> long desc
	m = sendKey(t, m, "tab") // long desc -> date
	m = sendKey(t, m, "tab") // date -> detail field 0 (Time Cap)
	m = typeText(t, m, "20")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "5")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "12")

	m = sendKey(t, m, "ctrl+s")

	if m.err != nil {
		t.Fatalf("unexpected error: %v", m.err)
	}
	if len(store.created) != 1 {
		t.Fatalf("store.created has %d entries, want 1", len(store.created))
	}

	got := store.created[0]
	if got.WorkoutType != fran.WorkoutTypeAMRAP {
		t.Fatalf("WorkoutType = %v, want AMRAP", got.WorkoutType)
	}
	want := &fran.AMRAPDetails{TimeCapMinutes: 20, RoundsCompleted: 5, ExtraReps: 12}
	if got.AMRAP == nil || *got.AMRAP != *want {
		t.Errorf("AMRAP = %+v, want %+v", got.AMRAP, want)
	}
}

func TestCreateWorkout_InvalidNumericDetailFieldBlocksSave(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	m = typeText(t, m, "20 min AMRAP")
	m = sendKey(t, m, "ctrl+t") // -> AMRAP

	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "tab")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "not-a-number")

	m = sendKey(t, m, "ctrl+s")

	if m.err == nil {
		t.Fatal("expected an error when a numeric detail field has non-numeric input, got nil")
	}
	if len(store.created) != 0 {
		t.Errorf("store.created has %d entries, want 0", len(store.created))
	}
}

func TestStrengthExercise_AddAndRemove(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	m = sendKey(t, m, "ctrl+t") // AMRAP
	m = sendKey(t, m, "ctrl+t") // EMOM
	m = sendKey(t, m, "ctrl+t") // Cardio
	m = sendKey(t, m, "ctrl+t") // Strength

	if len(m.inputs.DetailFields) != 5 || m.strengthExerciseCount != 1 {
		t.Fatalf("after switching to Strength: len(DetailFields)=%d strengthExerciseCount=%d, want 5 and 1", len(m.inputs.DetailFields), m.strengthExerciseCount)
	}

	// ctrl+a/ctrl+r should no-op for non-Strength types.
	m2 := sendKey(t, m, "ctrl+t") // -> Other
	m2 = sendKey(t, m2, "ctrl+a")
	if len(m2.inputs.DetailFields) != 0 {
		t.Errorf("ctrl+a on WorkoutTypeOther: len(DetailFields) = %d, want 0 (no-op)", len(m2.inputs.DetailFields))
	}

	m = sendKey(t, m, "ctrl+a")
	if len(m.inputs.DetailFields) != 10 || m.strengthExerciseCount != 2 {
		t.Fatalf("after ctrl+a: len(DetailFields)=%d strengthExerciseCount=%d, want 10 and 2", len(m.inputs.DetailFields), m.strengthExerciseCount)
	}

	m = sendKey(t, m, "ctrl+r")
	if len(m.inputs.DetailFields) != 5 || m.strengthExerciseCount != 1 {
		t.Fatalf("after ctrl+r: len(DetailFields)=%d strengthExerciseCount=%d, want 5 and 1", len(m.inputs.DetailFields), m.strengthExerciseCount)
	}

	// Can't remove below 1 exercise.
	m = sendKey(t, m, "ctrl+r")
	if len(m.inputs.DetailFields) != 5 || m.strengthExerciseCount != 1 {
		t.Errorf("ctrl+r below 1 exercise: len(DetailFields)=%d strengthExerciseCount=%d, want unchanged 5 and 1", len(m.inputs.DetailFields), m.strengthExerciseCount)
	}
}

func TestCreateWorkout_MultipleStrengthExercises_SkipsBlankTrailingSlot(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "n")
	m = typeText(t, m, "Leg day")
	m = sendKey(t, m, "ctrl+t")
	m = sendKey(t, m, "ctrl+t")
	m = sendKey(t, m, "ctrl+t")
	m = sendKey(t, m, "ctrl+t") // -> Strength

	m = sendKey(t, m, "ctrl+a") // 2 exercise slots, second left blank

	// Fill in exercise 1 (fields index 3..7): Name, Sets, Reps, Weight, Unit
	m = sendKey(t, m, "tab") // short desc -> long desc
	m = sendKey(t, m, "tab") // long desc -> date
	m = sendKey(t, m, "tab") // date -> exercise 1 name
	m = typeText(t, m, "Squat")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "5")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "5")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "225")
	m = sendKey(t, m, "tab")
	m = typeText(t, m, "lbs")

	m = sendKey(t, m, "ctrl+s")

	if m.err != nil {
		t.Fatalf("unexpected error: %v", m.err)
	}
	if len(store.created) != 1 {
		t.Fatalf("store.created has %d entries, want 1", len(store.created))
	}

	got := store.created[0]
	if got.Strength == nil || len(got.Strength.Exercises) != 1 {
		t.Fatalf("Strength.Exercises = %+v, want exactly 1 (blank trailing slot skipped)", got.Strength)
	}
	want := fran.StrengthExercise{Name: "Squat", Sets: 5, Reps: 5, Weight: 225, WeightUnit: "lbs"}
	if got.Strength.Exercises[0] != want {
		t.Errorf("Exercises[0] = %+v, want %+v", got.Strength.Exercises[0], want)
	}
}

func TestEditFlow_PrefillsTypeAndDetailFields(t *testing.T) {
	store := newFakeStore()
	store.workouts = []*fran.Workout{
		{
			ID:               1,
			ShortDescription: "Morning run",
			LongDescription:  "Easy pace",
			WorkoutDate:      "2026-09-12",
			WorkoutType:      fran.WorkoutTypeCardio,
			Cardio: &fran.CardioDetails{
				Modality:     "run",
				DurationMin:  32,
				Distance:     4.2,
				DistanceUnit: "mi",
			},
		},
	}

	m := newReadyModel(t, store)
	m.selectedWorkout = store.workouts[0]
	m.state = stateViewWorkout

	m = sendKey(t, m, "e")

	if m.state != stateCreateWorkout {
		t.Fatalf("state after 'e' = %v, want stateCreateWorkout", m.state)
	}
	if !m.isEditing || m.editingID != 1 {
		t.Fatalf("isEditing=%v editingID=%d, want true and 1", m.isEditing, m.editingID)
	}
	if m.WorkoutType != fran.WorkoutTypeCardio {
		t.Fatalf("WorkoutType = %v, want Cardio", m.WorkoutType)
	}
	if m.inputs.ShortDescriptionInput.Value() != "Morning run" {
		t.Errorf("ShortDescriptionInput = %q, want %q", m.inputs.ShortDescriptionInput.Value(), "Morning run")
	}
	if len(m.inputs.DetailFields) != 4 {
		t.Fatalf("len(DetailFields) = %d, want 4 for Cardio", len(m.inputs.DetailFields))
	}
	if got := m.inputs.DetailFields[0].input.Value(); got != "run" {
		t.Errorf("DetailFields[0] (Modality) = %q, want %q", got, "run")
	}
	if got := m.inputs.DetailFields[2].input.Value(); got != "4.2" {
		t.Errorf("DetailFields[2] (Distance) = %q, want %q", got, "4.2")
	}

	// Saving without changes should round-trip back through UpdateWorkout.
	m = sendKey(t, m, "ctrl+s")
	if len(store.updated) != 1 {
		t.Fatalf("store.updated has %d entries, want 1", len(store.updated))
	}
	if store.updated[0].ID != 1 {
		t.Errorf("updated workout ID = %d, want 1", store.updated[0].ID)
	}
}

func TestDeleteFlow_ConfirmationAndCancel(t *testing.T) {
	store := newFakeStore()
	w := &fran.Workout{ID: 1, ShortDescription: "to delete", WorkoutDate: "2026-09-12", WorkoutType: fran.WorkoutTypeOther}
	store.workouts = []*fran.Workout{w}

	m := newReadyModel(t, store)
	m.selectedWorkout = w
	m.state = stateViewWorkout

	m = sendKey(t, m, "d")
	if !m.confirmDelete {
		t.Fatal("confirmDelete = false after 'd', want true")
	}

	m = sendKey(t, m, "n")
	if m.confirmDelete {
		t.Error("confirmDelete = true after cancelling with 'n', want false")
	}
	if len(store.deleted) != 0 {
		t.Fatalf("store.deleted has %d entries after cancel, want 0", len(store.deleted))
	}

	m = sendKey(t, m, "d")
	m = sendKey(t, m, "y")

	if len(store.deleted) != 1 || store.deleted[0] != 1 {
		t.Fatalf("store.deleted = %v, want [1]", store.deleted)
	}
	if m.state != stateWorkoutList {
		t.Errorf("state after confirmed delete = %v, want stateWorkoutList", m.state)
	}
}

func TestQuitKeys(t *testing.T) {
	store := newFakeStore()
	m := newReadyModel(t, store)

	for _, key := range []string{"q", "ctrl+c"} {
		_, cmd := m.Update(keyMsg(key))
		if cmd == nil {
			t.Fatalf("key %q: expected a tea.Quit command, got nil", key)
		}
	}
}

func searchTestWorkouts() []*fran.Workout {
	return []*fran.Workout{
		{ID: 1, ShortDescription: "KB ABC", WorkoutDate: "2026-09-01", WorkoutType: fran.WorkoutTypeOther},
		{ID: 2, ShortDescription: "Cardio - run", WorkoutDate: "2026-09-02", WorkoutType: fran.WorkoutTypeOther},
		{ID: 3, ShortDescription: "KB DEF", WorkoutDate: "2026-09-03", WorkoutType: fran.WorkoutTypeOther},
	}
}

func TestSearch_QueriesDatabaseAndDisplaysResults(t *testing.T) {
	store := newFakeStore()
	store.workouts = searchTestWorkouts()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "/")
	if !m.searching {
		t.Fatal("searching = false after '/', want true")
	}

	m = typeText(t, m, "KB")
	m = sendKey(t, m, "enter")

	if m.searching {
		t.Error("searching = true after enter, want false")
	}
	if !m.searchActive {
		t.Error("searchActive = false after a non-empty search, want true")
	}
	if len(m.allWorkouts) != 2 {
		t.Fatalf("allWorkouts has %d entries after searching \"KB\", want 2", len(m.allWorkouts))
	}
}

func TestSearch_EmptyQueryShowsAllWorkouts(t *testing.T) {
	store := newFakeStore()
	store.workouts = searchTestWorkouts()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "/")
	m = sendKey(t, m, "enter")

	if m.searching {
		t.Error("searching = true after enter on an empty query, want false")
	}
	if m.searchActive {
		t.Error("searchActive = true after an empty-query enter, want false (should show all)")
	}
	if len(m.allWorkouts) != len(store.workouts) {
		t.Fatalf("allWorkouts has %d entries, want %d (all workouts)", len(m.allWorkouts), len(store.workouts))
	}
}

func TestSearch_EscWhileTypingCancelsWithoutSearching(t *testing.T) {
	store := newFakeStore()
	store.workouts = searchTestWorkouts()
	m := newReadyModel(t, store)
	originalCount := len(m.allWorkouts)

	m = sendKey(t, m, "/")
	m = typeText(t, m, "nonexistent query")
	m = sendKey(t, m, "esc")

	if m.searching {
		t.Error("searching = true after esc, want false")
	}
	if m.searchActive {
		t.Error("searchActive = true after cancelling with esc, want false")
	}
	if len(m.allWorkouts) != originalCount {
		t.Errorf("allWorkouts changed after cancelling search: got %d, want %d", len(m.allWorkouts), originalCount)
	}
}

func TestSearch_EscAfterActiveSearchResetsToLatest(t *testing.T) {
	store := newFakeStore()
	store.workouts = searchTestWorkouts()
	m := newReadyModel(t, store)

	m = sendKey(t, m, "/")
	m = typeText(t, m, "KB")
	m = sendKey(t, m, "enter")

	if len(m.allWorkouts) != 2 {
		t.Fatalf("allWorkouts has %d entries after search, want 2", len(m.allWorkouts))
	}

	m = sendKey(t, m, "esc")

	if m.searchActive {
		t.Error("searchActive = true after esc, want false")
	}
	if len(m.allWorkouts) != len(store.workouts) {
		t.Errorf("allWorkouts has %d entries after clearing search, want %d (all workouts)", len(m.allWorkouts), len(store.workouts))
	}
}

func TestSearch_ErrorSurfacesToModel(t *testing.T) {
	store := newFakeStore()
	store.workouts = searchTestWorkouts()
	store.searchErr = errors.New("boom")
	m := newReadyModel(t, store)

	m = sendKey(t, m, "/")
	m = typeText(t, m, "KB")
	m = sendKey(t, m, "enter")

	if m.err == nil {
		t.Fatal("expected m.err to be set after a failing search, got nil")
	}
}
