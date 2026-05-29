package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/warriorguo/memory_flow/backend/internal/database"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

type UserRepo struct {
	db database.DB
}

func NewUserRepo(db database.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, username, passwordHash string, displayName *string, role string) (*model.User, error) {
	query := `
		INSERT INTO users (id, username, password_hash, display_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, password_hash, display_name, role, created_at`

	row := r.db.QueryRow(ctx, query, uuid.New(), username, passwordHash, displayName, role)

	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `SELECT id, username, password_hash, display_name, role, created_at FROM users WHERE id = $1`
	row := r.db.QueryRow(ctx, query, id)

	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	query := `SELECT id, username, password_hash, display_name, role, created_at FROM users WHERE username = $1`
	row := r.db.QueryRow(ctx, query, username)

	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &u, nil
}
