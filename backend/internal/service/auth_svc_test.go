package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
)

const testJWTSecret = "test-secret"

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

func TestLogin_Success(t *testing.T) {
	userID := uuid.New()
	passwordHash := hashPassword(t, "correct-password")

	userRepo := &mocks.MockUserRepo{
		GetByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
			return &model.User{
				ID:           userID,
				Username:     username,
				PasswordHash: passwordHash,
				Role:         "admin",
				CreatedAt:    time.Now(),
			}, nil
		},
	}

	svc := NewAuthService(userRepo, testJWTSecret)
	resp, err := svc.Login(context.Background(), model.LoginRequest{
		Username: "testuser",
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", resp.User.Username)
	}
	if resp.User.ID != userID {
		t.Errorf("expected user ID %v, got %v", userID, resp.User.ID)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	userRepo := &mocks.MockUserRepo{
		GetByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
			return nil, nil
		},
	}

	svc := NewAuthService(userRepo, testJWTSecret)
	_, err := svc.Login(context.Background(), model.LoginRequest{
		Username: "nonexistent",
		Password: "password",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	passwordHash := hashPassword(t, "correct-password")

	userRepo := &mocks.MockUserRepo{
		GetByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
			return &model.User{
				ID:           uuid.New(),
				Username:     username,
				PasswordHash: passwordHash,
				Role:         "admin",
				CreatedAt:    time.Now(),
			}, nil
		},
	}

	svc := NewAuthService(userRepo, testJWTSecret)
	_, err := svc.Login(context.Background(), model.LoginRequest{
		Username: "testuser",
		Password: "wrong-password",
	})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateToken_Success(t *testing.T) {
	userRepo := &mocks.MockUserRepo{
		GetByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
			return &model.User{
				ID:           uuid.New(),
				Username:     username,
				PasswordHash: hashPassword(t, "password"),
				Role:         "admin",
				CreatedAt:    time.Now(),
			}, nil
		},
	}

	svc := NewAuthService(userRepo, testJWTSecret)

	// First login to get a valid token
	resp, err := svc.Login(context.Background(), model.LoginRequest{
		Username: "testuser",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Validate the token
	claims, err := svc.ValidateToken(resp.Token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	username, ok := claims["username"].(string)
	if !ok || username != "testuser" {
		t.Errorf("expected username testuser in claims, got %v", claims["username"])
	}

	role, ok := claims["role"].(string)
	if !ok || role != "admin" {
		t.Errorf("expected role admin in claims, got %v", claims["role"])
	}
}

func TestValidateToken_InvalidToken(t *testing.T) {
	userRepo := &mocks.MockUserRepo{}
	svc := NewAuthService(userRepo, testJWTSecret)

	_, err := svc.ValidateToken("invalid.token.string")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	userRepo := &mocks.MockUserRepo{}
	svc := NewAuthService(userRepo, testJWTSecret)

	// Create an expired token manually
	claims := jwt.MapClaims{
		"sub":      uuid.New().String(),
		"username": "testuser",
		"role":     "admin",
		"exp":      time.Now().Add(-1 * time.Hour).Unix(), // expired 1 hour ago
		"iat":      time.Now().Add(-2 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	_, err = svc.ValidateToken(tokenString)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}
