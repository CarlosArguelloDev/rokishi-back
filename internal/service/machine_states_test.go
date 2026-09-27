package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

type fakeMachineStateRepository struct {
	exists        bool
	current       models.MachineStatePeriod
	history       []models.MachineStatePeriod
	lastFilters   repository.StateHistoryFilters
	changes       []models.MachineStateChange
	lastStartedAt time.Time
	lastNotes     *string
	err           error
}

func (f *fakeMachineStateRepository) ListStates(context.Context) ([]models.MachineState, error) {
	return []models.MachineState{{ID: 1, Code: "DISPONIBLE", Name: "Disponible"}}, f.err
}

func (f *fakeMachineStateRepository) MachineExists(context.Context, int64) (bool, error) {
	return f.exists, f.err
}

func (f *fakeMachineStateRepository) Current(context.Context, int64) (models.MachineStatePeriod, error) {
	return f.current, f.err
}

func (f *fakeMachineStateRepository) History(_ context.Context, _ int64, filters repository.StateHistoryFilters) ([]models.MachineStatePeriod, error) {
	f.lastFilters = filters
	return f.history, f.err
}

func (f *fakeMachineStateRepository) Change(_ context.Context, machineID, stateID int64, startedAt time.Time, notes *string) (models.MachineStateChange, error) {
	f.lastStartedAt = startedAt
	f.lastNotes = notes
	if f.err != nil {
		return models.MachineStateChange{}, f.err
	}
	var previous *models.MachineStatePeriod
	if len(f.changes) > 0 {
		prior := f.changes[len(f.changes)-1].Current
		prior.EndedAt = &startedAt
		previous = &prior
	}
	change := models.MachineStateChange{
		Previous: previous,
		Current: models.MachineStatePeriod{
			ID: int64(len(f.changes) + 1), MachineID: machineID, StateID: stateID, StartedAt: startedAt, Notes: notes,
		},
	}
	f.changes = append(f.changes, change)
	return change, nil
}

func TestMachineStateServiceConsecutiveChanges(t *testing.T) {
	repository := &fakeMachineStateRepository{exists: true}
	service := NewMachineStateService(repository)
	firstTime := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	secondTime := firstTime.Add(time.Hour)
	firstValue := firstTime.Format(time.RFC3339)
	secondValue := secondTime.Format(time.RFC3339)
	notes := " Inicio de turno "

	first, err := service.Change(context.Background(), 4, ChangeMachineStateInput{StateID: 2, StartedAt: &firstValue, Notes: &notes})
	if err != nil {
		t.Fatal(err)
	}
	if first.Previous != nil || first.Current.StateID != 2 || first.Current.Notes == nil || *first.Current.Notes != "Inicio de turno" {
		t.Fatalf("unexpected first change: %+v", first)
	}

	second, err := service.Change(context.Background(), 4, ChangeMachineStateInput{StateID: 1, StartedAt: &secondValue})
	if err != nil {
		t.Fatal(err)
	}
	if second.Previous == nil || second.Previous.EndedAt == nil || !second.Previous.EndedAt.Equal(secondTime) || second.Current.StateID != 1 {
		t.Fatalf("unexpected consecutive change: %+v", second)
	}
}

func TestMachineStateServiceUsesClockAndValidatesInput(t *testing.T) {
	repository := &fakeMachineStateRepository{exists: true}
	service := NewMachineStateService(repository)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if _, err := service.Change(context.Background(), 1, ChangeMachineStateInput{StateID: 1}); err != nil {
		t.Fatal(err)
	}
	if !repository.lastStartedAt.Equal(now) {
		t.Fatalf("started at = %v, want %v", repository.lastStartedAt, now)
	}
	if _, err := service.Change(context.Background(), 1, ChangeMachineStateInput{}); !isValidationError(err) {
		t.Fatalf("expected state validation error, got %v", err)
	}
	invalid := "26/09/2026"
	if _, err := service.Change(context.Background(), 1, ChangeMachineStateInput{StateID: 1, StartedAt: &invalid}); !isValidationError(err) {
		t.Fatalf("expected date validation error, got %v", err)
	}
}

func TestMachineStateServiceHistoryFilters(t *testing.T) {
	repository := &fakeMachineStateRepository{exists: true}
	service := NewMachineStateService(repository)
	from := "2026-09-01T00:00:00Z"
	to := "2026-10-01T00:00:00Z"
	if _, err := service.History(context.Background(), 1, StateHistoryFilters{From: &from, To: &to}); err != nil {
		t.Fatal(err)
	}
	if repository.lastFilters.From == nil || repository.lastFilters.To == nil {
		t.Fatal("expected parsed history filters")
	}
	reversedFrom := "2026-10-01T00:00:00Z"
	if _, err := service.History(context.Background(), 1, StateHistoryFilters{From: &reversedFrom, To: &to}); !isValidationError(err) {
		t.Fatalf("expected range validation error, got %v", err)
	}
}

func TestMachineStateServiceMapsErrors(t *testing.T) {
	missing := NewMachineStateService(&fakeMachineStateRepository{exists: false})
	if _, err := missing.Current(context.Background(), 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	unchanged := NewMachineStateService(&fakeMachineStateRepository{exists: true, err: repository.ErrStateUnchanged})
	if _, err := unchanged.Change(context.Background(), 1, ChangeMachineStateInput{StateID: 2}); !errors.Is(err, ErrStateUnchanged) {
		t.Fatalf("expected unchanged state, got %v", err)
	}
	invalidTime := NewMachineStateService(&fakeMachineStateRepository{exists: true, err: repository.ErrInvalidStateTime})
	if _, err := invalidTime.Change(context.Background(), 1, ChangeMachineStateInput{StateID: 2}); !isValidationError(err) {
		t.Fatalf("expected transition date validation error, got %v", err)
	}
}
