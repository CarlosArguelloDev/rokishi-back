package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"rokishi-back/internal/models"
	"rokishi-back/internal/repository"
)

var (
	ErrUnauthorized     = errors.New("credenciales invalidas")
	ErrForbidden        = errors.New("operacion no autorizada")
	ErrSetupComplete    = errors.New("configuracion inicial completada")
	ErrInvalidSetupCode = errors.New("codigo de configuracion invalido")
)

const (
	passwordMemory      = 19 * 1024
	passwordIterations  = 2
	passwordParallelism = 1
	passwordKeyLength   = 32
	passwordSaltLength  = 16
)

type CreateUserInput struct {
	Name     string
	Email    string
	Password string
	Role     string
}

type UpdateUserInput struct {
	Name     Field[string]
	Email    Field[string]
	Password Field[string]
	Role     Field[string]
	Active   Field[bool]
}

type AuditInput struct {
	UserID     int64
	UserName   string
	Action     string
	Resource   string
	HTTPStatus int
	IPAddress  string
	UserAgent  string
}

type securityRepository interface {
	SetupRequired(context.Context) (bool, error)
	CreateInitialAdmin(context.Context, models.User, string) (models.User, error)
	CreateUser(context.Context, models.User, string) (models.User, error)
	ListUsers(context.Context) ([]models.User, error)
	GetUser(context.Context, int64) (models.User, error)
	GetCredentialByEmail(context.Context, string) (repository.UserCredential, error)
	UpdateUser(context.Context, models.User, *string) (models.User, error)
	CountActiveAdmins(context.Context) (int, error)
	CreateSession(context.Context, string, int64, time.Time) error
	Authenticate(context.Context, string) (models.User, error)
	DeleteSession(context.Context, string) error
	DeleteUserSessions(context.Context, int64) error
	RecordAudit(context.Context, models.AuditEntry) error
	ListAudit(context.Context, int) ([]models.AuditEntry, error)
}

type SecurityService struct {
	repository      securityRepository
	setupCodeHash   [32]byte
	sessionDuration time.Duration
	dummyHash       string
	now             func() time.Time
}

func NewSecurityService(repository securityRepository, setupCode string, sessionDuration time.Duration) (*SecurityService, error) {
	dummyHash, err := hashPassword("rokishi-dummy-password")
	if err != nil {
		return nil, err
	}
	return &SecurityService{
		repository:      repository,
		setupCodeHash:   sha256.Sum256([]byte(setupCode)),
		sessionDuration: sessionDuration,
		dummyHash:       dummyHash,
		now:             time.Now,
	}, nil
}

func GenerateSetupCode() (string, error) {
	return randomToken(24)
}

func (s *SecurityService) SetupRequired(ctx context.Context) (bool, error) {
	return s.repository.SetupRequired(ctx)
}

func (s *SecurityService) Bootstrap(ctx context.Context, setupCode string, input CreateUserInput) (models.User, string, time.Time, error) {
	provided := sha256.Sum256([]byte(setupCode))
	if subtle.ConstantTimeCompare(provided[:], s.setupCodeHash[:]) != 1 {
		return models.User{}, "", time.Time{}, ErrInvalidSetupCode
	}
	user, passwordHash, err := prepareUser(input, models.RoleAdmin)
	if err != nil {
		return models.User{}, "", time.Time{}, err
	}
	created, err := s.repository.CreateInitialAdmin(ctx, user, passwordHash)
	if errors.Is(err, repository.ErrSetupComplete) {
		return models.User{}, "", time.Time{}, ErrSetupComplete
	}
	if err != nil {
		return models.User{}, "", time.Time{}, mapRepositoryError(err)
	}
	token, expiresAt, err := s.newSession(ctx, created.ID)
	return created, token, expiresAt, err
}

func (s *SecurityService) Login(ctx context.Context, email, password string) (models.User, string, time.Time, error) {
	credential, err := s.repository.GetCredentialByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			_ = verifyPassword(s.dummyHash, password)
			return models.User{}, "", time.Time{}, ErrUnauthorized
		}
		return models.User{}, "", time.Time{}, err
	}
	if !credential.User.Active || !verifyPassword(credential.PasswordHash, password) {
		return models.User{}, "", time.Time{}, ErrUnauthorized
	}
	token, expiresAt, err := s.newSession(ctx, credential.User.ID)
	return credential.User, token, expiresAt, err
}

func (s *SecurityService) Authenticate(ctx context.Context, token string) (models.User, error) {
	if token == "" {
		return models.User{}, ErrUnauthorized
	}
	user, err := s.repository.Authenticate(ctx, hashToken(token))
	if errors.Is(err, repository.ErrNotFound) {
		return models.User{}, ErrUnauthorized
	}
	return user, err
}

func (s *SecurityService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repository.DeleteSession(ctx, hashToken(token))
}

func (s *SecurityService) ListUsers(ctx context.Context) ([]models.User, error) {
	return s.repository.ListUsers(ctx)
}

func (s *SecurityService) CreateUser(ctx context.Context, input CreateUserInput) (models.User, error) {
	user, passwordHash, err := prepareUser(input, input.Role)
	if err != nil {
		return models.User{}, err
	}
	created, err := s.repository.CreateUser(ctx, user, passwordHash)
	return created, mapRepositoryError(err)
}

func (s *SecurityService) UpdateUser(ctx context.Context, actorID, id int64, input UpdateUserInput) (models.User, error) {
	if id < 1 || (!input.Name.Set && !input.Email.Set && !input.Password.Set && !input.Role.Set && !input.Active.Set) {
		return models.User{}, &ValidationError{Message: "Debes proporcionar un usuario valido y al menos un campo"}
	}
	user, err := s.repository.GetUser(ctx, id)
	if err != nil {
		return models.User{}, mapRepositoryError(err)
	}
	originalRole, originalActive := user.Role, user.Active
	if input.Name.Set {
		if input.Name.Value == nil {
			return models.User{}, requiredField("nombre")
		}
		user.Name = strings.TrimSpace(*input.Name.Value)
	}
	if input.Email.Set {
		if input.Email.Value == nil {
			return models.User{}, requiredField("correo")
		}
		user.Email = normalizeEmail(*input.Email.Value)
	}
	if input.Role.Set {
		if input.Role.Value == nil {
			return models.User{}, requiredField("rol")
		}
		user.Role = strings.ToUpper(strings.TrimSpace(*input.Role.Value))
	}
	if input.Active.Set {
		if input.Active.Value == nil {
			return models.User{}, requiredField("activo")
		}
		user.Active = *input.Active.Value
	}
	if id == actorID && (!user.Active || user.Role != models.RoleAdmin) {
		return models.User{}, &ValidationError{Message: "No puedes desactivar tu usuario ni quitarte el rol de administrador"}
	}
	if originalRole == models.RoleAdmin && originalActive && (user.Role != models.RoleAdmin || !user.Active) {
		count, err := s.repository.CountActiveAdmins(ctx)
		if err != nil {
			return models.User{}, err
		}
		if count <= 1 {
			return models.User{}, &ValidationError{Message: "Debe permanecer al menos un administrador activo"}
		}
	}
	if err := validateUser(user); err != nil {
		return models.User{}, err
	}
	var passwordHash *string
	if input.Password.Set {
		if input.Password.Value == nil {
			return models.User{}, requiredField("password")
		}
		if err := validatePassword(*input.Password.Value); err != nil {
			return models.User{}, err
		}
		hashed, err := hashPassword(*input.Password.Value)
		if err != nil {
			return models.User{}, err
		}
		passwordHash = &hashed
	}
	updated, err := s.repository.UpdateUser(ctx, user, passwordHash)
	if err != nil {
		return models.User{}, mapRepositoryError(err)
	}
	if passwordHash != nil || !updated.Active || updated.Role != originalRole {
		if err := s.repository.DeleteUserSessions(ctx, id); err != nil {
			return models.User{}, err
		}
	}
	return updated, nil
}

func (s *SecurityService) RecordAudit(ctx context.Context, input AuditInput) error {
	entry := models.AuditEntry{
		UserID: &input.UserID, UserName: input.UserName, Action: input.Action,
		Resource: truncateRunes(input.Resource, 200), HTTPStatus: input.HTTPStatus,
		IPAddress: cleanNullableString(input.IPAddress, 100), UserAgent: cleanNullableString(input.UserAgent, 500),
	}
	return s.repository.RecordAudit(ctx, entry)
}

func (s *SecurityService) ListAudit(ctx context.Context, limit int) ([]models.AuditEntry, error) {
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 200 {
		return nil, &ValidationError{Message: "El limite debe estar entre 1 y 200"}
	}
	return s.repository.ListAudit(ctx, limit)
}

func (s *SecurityService) newSession(ctx context.Context, userID int64) (string, time.Time, error) {
	token, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := s.now().Add(s.sessionDuration)
	if err := s.repository.CreateSession(ctx, hashToken(token), userID, expiresAt); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

func prepareUser(input CreateUserInput, role string) (models.User, string, error) {
	user := models.User{Name: strings.TrimSpace(input.Name), Email: normalizeEmail(input.Email), Role: strings.ToUpper(strings.TrimSpace(role)), Active: true}
	if err := validateUser(user); err != nil {
		return models.User{}, "", err
	}
	if err := validatePassword(input.Password); err != nil {
		return models.User{}, "", err
	}
	passwordHash, err := hashPassword(input.Password)
	return user, passwordHash, err
}

func validateUser(user models.User) error {
	if user.Name == "" {
		return requiredField("nombre")
	}
	if utf8.RuneCountInString(user.Name) > 100 {
		return maxLength("nombre", 100)
	}
	if user.Email == "" {
		return requiredField("correo")
	}
	if utf8.RuneCountInString(user.Email) > 254 {
		return maxLength("correo", 254)
	}
	parsed, err := mail.ParseAddress(user.Email)
	if err != nil || normalizeEmail(parsed.Address) != user.Email {
		return &ValidationError{Message: "El campo correo no tiene un formato valido"}
	}
	if user.Role != models.RoleAdmin && user.Role != models.RoleOperator {
		return &ValidationError{Message: "El rol debe ser ADMIN u OPERADOR"}
	}
	return nil
}

func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < 12 || length > 128 {
		return &ValidationError{Message: "La contrasena debe contener entre 12 y 128 caracteres"}
	}
	return nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func cleanNullableString(value string, maximum int) *string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return nil
	}
	cleaned = truncateRunes(cleaned, maximum)
	return &cleaned
}

func truncateRunes(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemory, passwordParallelism, passwordKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemory, passwordIterations,
		passwordParallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	if memory < 8*1024 || memory > 256*1024 || iterations < 1 || iterations > 10 || parallelism < 1 || parallelism > 8 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func ParseAuditLimit(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 100, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return 0, &ValidationError{Message: "El limite debe ser un entero"}
	}
	return limit, nil
}
