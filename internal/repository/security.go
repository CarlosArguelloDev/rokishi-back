package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"rokishi-back/internal/models"
)

type securityDB interface {
	DB
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type UserCredential struct {
	User         models.User
	PasswordHash string
}

type SecurityRepository struct {
	db securityDB
}

func NewSecurityRepository(db securityDB) *SecurityRepository {
	return &SecurityRepository{db: db}
}

func (r *SecurityRepository) SetupRequired(ctx context.Context) (bool, error) {
	var required bool
	err := r.db.QueryRow(ctx, `SELECT NOT EXISTS (SELECT 1 FROM usuarios)`).Scan(&required)
	return required, err
}

func (r *SecurityRepository) CreateInitialAdmin(ctx context.Context, user models.User, passwordHash string) (models.User, error) {
	created, err := scanUser(r.db.QueryRow(ctx, `
		INSERT INTO usuarios (nombre, correo, password_hash, rol, activo, es_usuario_inicial)
		SELECT $1, $2, $3, 'ADMIN', TRUE, TRUE
		WHERE NOT EXISTS (SELECT 1 FROM usuarios)
		RETURNING id, nombre, correo, rol, activo, fecha_creacion, fecha_actualizacion`,
		user.Name, user.Email, passwordHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrSetupComplete
	}
	err = translateError(err)
	if errors.Is(err, ErrConflict) {
		return models.User{}, ErrSetupComplete
	}
	return created, err
}

func (r *SecurityRepository) CreateUser(ctx context.Context, user models.User, passwordHash string) (models.User, error) {
	created, err := scanUser(r.db.QueryRow(ctx, `
		INSERT INTO usuarios (nombre, correo, password_hash, rol, activo)
		VALUES ($1, $2, $3, $4, TRUE)
		RETURNING id, nombre, correo, rol, activo, fecha_creacion, fecha_actualizacion`,
		user.Name, user.Email, passwordHash, user.Role))
	return created, translateError(err)
}

func (r *SecurityRepository) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, nombre, correo, rol, activo, fecha_creacion, fecha_actualizacion
		FROM usuarios ORDER BY nombre, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]models.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *SecurityRepository) GetUser(ctx context.Context, id int64) (models.User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `
		SELECT id, nombre, correo, rol, activo, fecha_creacion, fecha_actualizacion
		FROM usuarios WHERE id = $1`, id))
	return user, translateError(err)
}

func (r *SecurityRepository) GetCredentialByEmail(ctx context.Context, email string) (UserCredential, error) {
	var credential UserCredential
	err := r.db.QueryRow(ctx, `
		SELECT id, nombre, correo, rol, activo, fecha_creacion, fecha_actualizacion, password_hash
		FROM usuarios WHERE lower(correo) = lower($1)`, email).Scan(
		&credential.User.ID, &credential.User.Name, &credential.User.Email, &credential.User.Role,
		&credential.User.Active, &credential.User.CreationDate, &credential.User.LastUpdatedAt,
		&credential.PasswordHash,
	)
	return credential, translateError(err)
}

func (r *SecurityRepository) UpdateUser(ctx context.Context, user models.User, passwordHash *string) (models.User, error) {
	updated, err := scanUser(r.db.QueryRow(ctx, `
		UPDATE usuarios
		SET nombre = $2, correo = $3, rol = $4, activo = $5,
			password_hash = COALESCE($6, password_hash), fecha_actualizacion = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING id, nombre, correo, rol, activo, fecha_creacion, fecha_actualizacion`,
		user.ID, user.Name, user.Email, user.Role, user.Active, passwordHash))
	return updated, translateError(err)
}

func (r *SecurityRepository) CountActiveAdmins(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM usuarios WHERE rol = 'ADMIN' AND activo`).Scan(&count)
	return count, err
}

func (r *SecurityRepository) CreateSession(ctx context.Context, tokenHash string, userID int64, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO sesiones (token_hash, usuario_id, fecha_expiracion)
		VALUES ($1, $2, $3)`, tokenHash, userID, expiresAt)
	return err
}

func (r *SecurityRepository) Authenticate(ctx context.Context, tokenHash string) (models.User, error) {
	user, err := scanUser(r.db.QueryRow(ctx, `
		WITH sesion AS (
			UPDATE sesiones
			SET ultima_actividad = CURRENT_TIMESTAMP
			WHERE token_hash = $1 AND fecha_expiracion > CURRENT_TIMESTAMP
			RETURNING usuario_id
		)
		SELECT u.id, u.nombre, u.correo, u.rol, u.activo, u.fecha_creacion, u.fecha_actualizacion
		FROM usuarios u JOIN sesion s ON s.usuario_id = u.id
		WHERE u.activo`, tokenHash))
	return user, translateError(err)
}

func (r *SecurityRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sesiones WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *SecurityRepository) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sesiones WHERE usuario_id = $1`, userID)
	return err
}

func (r *SecurityRepository) RecordAudit(ctx context.Context, entry models.AuditEntry) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auditoria (usuario_id, usuario_nombre, accion, recurso, estado_http, direccion_ip, agente_usuario)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.UserID, entry.UserName, entry.Action, entry.Resource, entry.HTTPStatus, entry.IPAddress, entry.UserAgent)
	return err
}

func (r *SecurityRepository) ListAudit(ctx context.Context, limit int) ([]models.AuditEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, usuario_id, usuario_nombre, accion, recurso, estado_http, direccion_ip, agente_usuario, fecha
		FROM auditoria ORDER BY fecha DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]models.AuditEntry, 0)
	for rows.Next() {
		var entry models.AuditEntry
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.UserName, &entry.Action, &entry.Resource,
			&entry.HTTPStatus, &entry.IPAddress, &entry.UserAgent, &entry.Date); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func scanUser(row scanner) (models.User, error) {
	var user models.User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.Role, &user.Active, &user.CreationDate, &user.LastUpdatedAt)
	return user, err
}
