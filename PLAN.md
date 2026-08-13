# Improvement Plan for Fran

This document outlines a roadmap for improving and extending the Fran workout tracker. The goal is to maintain its minimalist nature while enhancing usability and functionality.

## Phase 1: Core Enhancements (UX & Robustness)

### 1.1 Input Validation & Feedback
- Implement validation for the `WorkoutDate` field (ensure it follows YYYY-MM-DD format).
- Add real-time validation feedback in the "Create Workout" view (e.g., highlight fields in red if invalid).
- Show specific error messages if a database operation fails instead of just "Error: ...".

### 1.2 Enhanced Navigation & Filtering
- Integrate the `GetWorkoutsByShortDesc` functionality into the main list view.
- Add a search bar or a toggleable filter at the top of the list to quickly find specific workouts.
- Add "Previous" and "Next" navigation between workouts in the "View Workout" state.

### 1.3 Polished UI/UX
- Use `lipgloss` to add borders and consistent spacing to the "Create Workout" form.
- Improve the "View Workout" layout to better handle long descriptions (e.g., using a scrollable area if it exceeds terminal height).
- Add a "loading" spinner or progress indicator for database fetches.

### 1.4 Configuration & Defaults
- Support a configuration file (e.g., `~/.config/fran/config.toml`) to manage settings like `FRAN_DB_PATH`, default list size, and theme preferences.

## Phase 2: Data Model & Functionality Extensions

### 2.1 Richer Workout Data
- **Categories/Tags:** Add a tagging system to categorize workouts (e.g., #strength, #cardio, #mobility).
- **Sets and Reps:** Introduce a structured way to record sets, reps, and weight (e.g., `Bench Press: 3x10 @ 135lbs`).
- **Duration Tracking:** Add a field to track how long a workout lasted.

### 2.2 Statistics & Analytics
- **Summary View:** A new state to view aggregate statistics (e.g., workouts per week, total volume).
- **Progress Tracking:** Simple visual indicators of progress over time for specific exercises.

### 2.3 Data Portability
- **Export/Import:** Add commands to export workouts to JSON or CSV files for backup or sharing.

## Phase 3: Advanced Features (Optional)

### 3.1 Notifications
- Integration with system notifications (e.g., `notify-send` or similar) for workout reminders (if a timer is added).

### 3.2 Multiple Profiles
- Support for different profiles (e.g., "Powerlifting", "Yoga") to keep workout histories separate.
