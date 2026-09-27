package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

var ErrStateUnchanged = errors.New("la maquina ya tiene ese estado")

type ChangeMachineStateInput struct {
	StateID   int64
	StartedAt *string
	Notes     *string
}

type StateHistoryFilters struct {
	From *string
	To   *string
}

type machineStateRepository interface {
	ListStates(context.Context) ([]models.MachineState, error)
	MachineExists(context.Context, int64) (bool, error)
	Current(context.Context, int64) (models.MachineStatePeriod, error)
	History(context.Context, int64, repository.StateHistoryFilters) ([]models.MachineStatePeriod, error)
	Change(context.Context, int64, int64, time.Time, *string) (models.MachineStateChange, error)
}

type MachineStateService struct {
	repository machineStateRepository
	now        func() time.Time
}

func NewMachineStateService(repository machineStateRepository) *MachineStateService {
	return &MachineStateService{repository: repository, now: time.Now}
}

func (s *MachineStateService) List(ctx context.Context) ([]models.MachineState, error) {
	return s.repository.ListStates(ctx)
}

func (s *MachineStateService) Current(ctx context.Context, machineID int64) (models.MachineStatePeriod, error) {
	if err := s.requireMachine(ctx, machineID); err != nil {
		return models.MachineStatePeriod{}, err
	}
	period, err := s.repository.Current(ctx, machineID)
	return period, mapRepositoryError(err)
}

func (s *MachineStateService) History(ctx context.Context, machineID int64, filters StateHistoryFilters) ([]models.MachineStatePeriod, error) {
	if err := s.requireMachine(ctx, machineID); err != nil {
		return nil, err
	}
	from, err := parseOptionalTimestamp(filters.From, "desde")
	if err != nil {
		return nil, err
	}
	to, err := parseOptionalTimestamp(filters.To, "hasta")
	if err != nil {
		return nil, err
	}
	if from != nil && to != nil && !to.After(*from) {
		return nil, &ValidationError{Message: "El filtro hasta debe ser posterior a desde"}
	}
	return s.repository.History(ctx, machineID, repository.StateHistoryFilters{From: from, To: to})
}

func (s *MachineStateService) Change(ctx context.Context, machineID int64, input ChangeMachineStateInput) (models.MachineStateChange, error) {
	if input.StateID < 1 {
		return models.MachineStateChange{}, &ValidationError{Message: "El campo estado_maquina_id debe ser un entero positivo"}
	}
	startedAt := s.now()
	if input.StartedAt != nil {
		parsed, err := parseTimestamp(*input.StartedAt, "fecha_inicio")
		if err != nil {
			return models.MachineStateChange{}, err
		}
		startedAt = parsed
	}
	change, err := s.repository.Change(ctx, machineID, input.StateID, startedAt, cleanNullable(input.Notes))
	switch {
	case errors.Is(err, repository.ErrStateUnchanged):
		return models.MachineStateChange{}, ErrStateUnchanged
	case errors.Is(err, repository.ErrInvalidStateTime):
		return models.MachineStateChange{}, &ValidationError{Message: "La fecha del cambio debe ser posterior al inicio del estado actual"}
	default:
		return change, mapMachineRepositoryError(err)
	}
}

func (s *MachineStateService) requireMachine(ctx context.Context, machineID int64) error {
	exists, err := s.repository.MachineExists(ctx, machineID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func parseOptionalTimestamp(value *string, field string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := parseTimestamp(*value, field)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseTimestamp(value, field string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, &ValidationError{Message: "El campo " + field + " debe tener formato RFC3339"}
	}
	return parsed, nil
}
