package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/warriorguo/memory_flow/backend/internal/model"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, username, passwordHash string, displayName *string, role string) (*model.User, error) {
	query := `
		INSERT INTO users (username, password_hash, display_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, password_hash, display_name, role, created_at`

	row := r.pool.QueryRow(ctx, query, username, passwordHash, displayName, role)

	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `SELECT id, username, password_hash, display_name, role, created_at FROM users WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)

	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	query := `SELECT id, username, password_hash, display_name, role, created_at FROM users WHERE username = $1`
	row := r.pool.QueryRow(ctx, query, username)

	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &u, nil
}
