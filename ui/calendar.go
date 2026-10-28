package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ekholme/fran"
)

const dateLayout = "2006-01-02"

// now is the calendar's clock, overridable in tests.
var now = time.Now

// calendarCellWidth is the rendered width of one day in the month grid:
// a 2-digit day number, a space, and up to 3 workout-type badges.
const calendarCellWidth = 7

// calendarWeekdays are the grid's column headers; weeks start on Monday.
var calendarWeekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

type calendarLoadedMsg struct {
	month    time.Time
	workouts []*fran.Workout
}

// firstOfMonth returns midnight on the first day of t's month.
func firstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
}

// mondayIndex returns t's weekday as a 0-based column index with Monday
// first (Mon=0 ... Sun=6).
func mondayIndex(t time.Time) int {
	return (int(t.Weekday()) + 6) % 7
}

// workoutTypeBadge returns the single-letter marker shown in a calendar
// cell for a workout of type t.
func workoutTypeBadge(t fran.WorkoutType) string {
	switch t {
	case fran.WorkoutTypeAMRAP:
		return "A"
	case fran.WorkoutTypeEMOM:
		return "E"
	case fran.WorkoutTypeCardio:
		return "C"
	case fran.WorkoutTypeStrength:
		return "S"
	default:
		return "O"
	}
}

func fetchCalendarMonthCmd(s WorkoutStore, month time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		start := month.Format(dateLayout)
		end := month.AddDate(0, 1, -1).Format(dateLayout)
		workouts, err := s.GetWorkoutsByDateRange(ctx, start, end)
		if err != nil {
			return err
		}
		return calendarLoadedMsg{month: month, workouts: workouts}
	}
}

// openCalendar switches to the calendar screen with the cursor on day and
// fetches day's month.
func (m WorkoutModel) openCalendar(day time.Time) (WorkoutModel, tea.Cmd) {
	m.state = stateCalendar
	m.calendarCursor = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	m.calendarMonth = firstOfMonth(day)
	m.calendarWorkouts = nil
	m.calendarDayIndex = 0
	return m, fetchCalendarMonthCmd(m.store, m.calendarMonth)
}

// moveCalendarCursor moves the selected day to day, refetching only if
// that crosses into a different month.
func (m WorkoutModel) moveCalendarCursor(day time.Time) (WorkoutModel, tea.Cmd) {
	if !firstOfMonth(day).Equal(m.calendarMonth) {
		return m.openCalendar(day)
	}
	m.calendarCursor = day
	m.calendarDayIndex = 0
	return m, nil
}

// calendarWorkoutsOn returns the loaded workouts dated on day.
func (m WorkoutModel) calendarWorkoutsOn(day time.Time) []*fran.Workout {
	date := day.Format(dateLayout)
	var out []*fran.Workout
	for _, w := range m.calendarWorkouts {
		if w.WorkoutDate == date {
			out = append(out, w)
		}
	}
	return out
}

// browseWorkouts returns the workouts the view screen's previous/next
// navigation steps through: the loaded month when the workout was opened
// from the calendar, otherwise the list's loaded page.
func (m WorkoutModel) browseWorkouts() []*fran.Workout {
	if m.returnState == stateCalendar {
		return m.calendarWorkouts
	}
	return m.allWorkouts
}

func (m WorkoutModel) updateCalendar(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "c":
		m.state = stateWorkoutList
		m.returnState = stateWorkoutList
		m.loading = true
		return m, fetchLatestWorkoutsCmd(m.store, m.listLength)
	case "left", "h":
		return m.moveCalendarCursor(m.calendarCursor.AddDate(0, 0, -1))
	case "right", "l":
		return m.moveCalendarCursor(m.calendarCursor.AddDate(0, 0, 1))
	case "up", "k":
		return m.moveCalendarCursor(m.calendarCursor.AddDate(0, 0, -7))
	case "down", "j":
		return m.moveCalendarCursor(m.calendarCursor.AddDate(0, 0, 7))
	case "[":
		return m.openCalendar(m.calendarCursor.AddDate(0, -1, 0))
	case "]":
		return m.openCalendar(m.calendarCursor.AddDate(0, 1, 0))
	case "t":
		return m.moveCalendarCursor(now())
	case "tab":
		if n := len(m.calendarWorkoutsOn(m.calendarCursor)); n > 0 {
			m.calendarDayIndex = (m.calendarDayIndex + 1) % n
		}
		return m, nil
	case "enter":
		day := m.calendarWorkoutsOn(m.calendarCursor)
		if len(day) == 0 {
			return m, nil
		}
		m.returnState = stateCalendar
		m.selectedWorkout = day[m.calendarDayIndex]
		m.state = stateViewWorkout
		m.loading = true
		return m, fetchWorkoutCmd(m.store, m.selectedWorkout.ID)
	case "n":
		m = m.resetWorkoutForm()
		m.returnState = stateCalendar
		m.inputs.WorkoutDateInput.SetValue(m.calendarCursor.Format(dateLayout))
		m.state = stateCreateWorkout
		return m, nil
	}

	return m, nil
}

func (m WorkoutModel) viewCalendar() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	hasWorkoutStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	todayStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	selectedStyle := lipgloss.NewStyle().Reverse(true)
	cellStyle := lipgloss.NewStyle().Width(calendarCellWidth)

	byDate := make(map[string][]*fran.Workout)
	for _, w := range m.calendarWorkouts {
		byDate[w.WorkoutDate] = append(byDate[w.WorkoutDate], w)
	}

	headers := make([]string, len(calendarWeekdays))
	for i, d := range calendarWeekdays {
		headers[i] = cellStyle.Render(headerStyle.Render(d))
	}
	rows := []string{lipgloss.JoinHorizontal(lipgloss.Top, headers...)}

	today := now().Format(dateLayout)
	selected := m.calendarCursor.Format(dateLayout)

	// Each row is one Monday-to-Sunday week; leading blanks pad the
	// first week out to the month's starting weekday.
	cells := make([]string, mondayIndex(m.calendarMonth))
	for i := range cells {
		cells[i] = cellStyle.Render("")
	}
	for day := m.calendarMonth; day.Month() == m.calendarMonth.Month(); day = day.AddDate(0, 0, 1) {
		date := day.Format(dateLayout)

		num := fmt.Sprintf("%2d", day.Day())
		if date == today {
			num = todayStyle.Render(num)
		}

		var badges strings.Builder
		for i, w := range byDate[date] {
			if i == 3 {
				badges.WriteString("+")
				break
			}
			badges.WriteString(workoutTypeBadge(w.WorkoutType))
		}

		content := num
		if badges.Len() > 0 {
			content += " " + hasWorkoutStyle.Render(badges.String())
		}
		if date == selected {
			content = selectedStyle.Render(content)
		}
		cells = append(cells, cellStyle.Render(content))

		if len(cells) == 7 {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
			cells = nil
		}
	}
	if len(cells) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}

	dayLines := []string{"", titleStyle.Render(m.calendarCursor.Format("Monday, January 2, 2006"))}
	dayWorkouts := byDate[selected]
	if len(dayWorkouts) == 0 {
		dayLines = append(dayLines, headerStyle.Render("No workouts"))
	}
	for i, w := range dayWorkouts {
		prefix := "  "
		if i == m.calendarDayIndex {
			prefix = "> "
		}
		dayLines = append(dayLines, prefix+w.ShortDescription+headerStyle.Render(" · "+w.WorkoutType.Label()))
	}

	help := headerStyle.Render("\nhjkl/arrows: move  [/]: prev/next month  t: today  tab: next on day\nenter: view  n: new on this day  esc/c: back to list  q: quit\nA=AMRAP E=EMOM C=Cardio S=Strength O=Other")

	lines := []string{titleStyle.Render(m.calendarMonth.Format("January 2006")), ""}
	lines = append(lines, rows...)
	lines = append(lines, dayLines...)
	lines = append(lines, help)

	return lipgloss.NewStyle().Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
