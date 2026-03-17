package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/warriorguo/memory_flow/backend/internal/model"
	"github.com/warriorguo/memory_flow/backend/internal/repository/mocks"
	"github.com/warriorguo/memory_flow/backend/internal/service"
)

func hashTestPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

func newTestAuthHandler() (*AuthHandler, *mocks.MockUserRepo) {
	userRepo := &mocks.MockUserRepo{}
	svc := service.NewAuthService(userRepo, "test-secret")
	h := NewAuthHandler(svc)
	return h, userRepo
}

func TestLogin_Handler_Success(t *testing.T) {
	h, userRepo := newTestAuthHandler()

	passwordHash := hashTestPassword(t, "correct-password")

	userRepo.GetByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
		return &model.User{
			ID:           uuid.New(),
			Username:     username,
			PasswordHash: passwordHash,
			Role:         "admin",
			CreatedAt:    time.Now(),
		}, nil
	}

	body, _ := json.Marshal(model.LoginRequest{
		Username: "testuser",
		Password: "correct-password",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data model.LoginResponse `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.Token == "" {
		t.Error("expected non-empty token")
	}
}

func TestLogin_Handler_InvalidCredentials(t *testing.T) {
	h, userRepo := newTestAuthHandler()

	userRepo.GetByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
		return nil, nil
	}

	body, _ := json.Marshal(model.LoginRequest{
		Username: "nonexistent",
		Password: "password",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d; body: %s", w.Code, w.Body.String())
	}
}
