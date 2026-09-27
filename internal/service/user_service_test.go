package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"backend-challenge/internal/model"
	"backend-challenge/internal/repository"
)

type mockUserRepository struct {
	users map[primitive.ObjectID]model.User
}

func newFakeRepo() *mockUserRepository {
	return &mockUserRepository{users: map[primitive.ObjectID]model.User{}}
}

func (f *mockUserRepository) Create(_ context.Context, user *model.User) error {
	for _, u := range f.users {
		if u.Email == user.Email {
			return repository.ErrDuplicateEmail
		}
	}
	user.ID = primitive.NewObjectID()
	f.users[user.ID] = *user
	return nil
}

func (f *mockUserRepository) GetByID(_ context.Context, id string) (*model.User, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, repository.ErrInvalidID
	}
	u, ok := f.users[objectID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &u, nil
}

func (f *mockUserRepository) GetByEmail(_ context.Context, email string) (*model.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return &u, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *mockUserRepository) List(_ context.Context) ([]model.User, error) {
	users := []model.User{}
	for _, u := range f.users {
		users = append(users, u)
	}
	return users, nil
}

func (f *mockUserRepository) Update(_ context.Context, user *model.User) error {
	if _, ok := f.users[user.ID]; !ok {
		return repository.ErrNotFound
	}
	f.users[user.ID] = *user
	return nil
}

func (f *mockUserRepository) Delete(_ context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return repository.ErrInvalidID
	}
	if _, ok := f.users[objectID]; !ok {
		return repository.ErrNotFound
	}
	delete(f.users, objectID)
	return nil
}

func (f *mockUserRepository) Count(_ context.Context) (int64, error) {
	return int64(len(f.users)), nil
}

const testSecret = "test-secret"

func newTestService() (*UserService, *mockUserRepository) {
	repo := newFakeRepo()
	return NewUserService(repo, testSecret, time.Hour), repo
}

func TestRegister_Success(t *testing.T) {
	s, _ := newTestService()

	user, err := s.Register(context.Background(), CreateUserInput{
		Name:     "MOCK",
		Email:    "mock@example.com",
		Password: "mock-pass",
	})
	if err != nil {
		t.Fatalf("register: unexpected error: %v", err)
	}

	if user.ID.IsZero() {
		t.Error("expected user ID to be set")
	}
	if user.Password == "password123" {
		t.Error("expected password to be hashed")
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	s, _ := newTestService()

	s.Register(context.Background(), CreateUserInput{
		Name:     "MOCK",
		Email:    "mock@example.com",
		Password: "mock-pass",
	})

	_, err := s.Register(context.Background(), CreateUserInput{
		Name:     "MOCK",
		Email:    "mock@example.com",
		Password: "mock-pass",
	})

	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	s, _ := newTestService()

	user, err := s.Register(context.Background(), CreateUserInput{
		Name:     "MOCK",
		Email:    "mock@example.com",
		Password: "mock-pass",
	})

	token, err := s.Login(context.Background(), "mock@example.com", "mock-pass")
	if err != nil {
		t.Fatalf("login: unexpected error: %v", err)
	}

	userID, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if userID != user.ID.Hex() {
		t.Errorf("expected user ID %s in token, got %s", user.ID.Hex(), userID)
	}
}

func TestGetUser_Success(t *testing.T) {
	s, _ := newTestService()
	created, _ := s.Register(context.Background(), CreateUserInput{
		Name:     "MOCK",
		Email:    "mock@example.com",
		Password: "mock-pass",
	})

	user, err := s.GetUser(context.Background(), created.ID.Hex())
	if err != nil {
		t.Fatalf("get user: unexpected error: %v", err)
	}
	if user.Email != "mock@example.com" || user.Name != "MOCK" {
		t.Errorf("unexpected user: %+v", user)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	s, _ := newTestService()

	_, err := s.GetUser(context.Background(), primitive.NewObjectID().Hex())

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetUser_InvalidID(t *testing.T) {
	s, _ := newTestService()

	_, err := s.GetUser(context.Background(), "not-an-object-id")

	if !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

func TestUpdateUser_NameAndEmail(t *testing.T) {
	s, _ := newTestService()
	created, _ := s.Register(context.Background(), CreateUserInput{
		Name:     "MOCK",
		Email:    "mock@example.com",
		Password: "mock-pass",
	})

	name, email := "MOCK Updated", "mock.new@example.com"
	user, err := s.UpdateUser(context.Background(), created.ID.Hex(), UpdateUserInput{Name: &name, Email: &email})
	if err != nil {
		t.Fatalf("update: unexpected error: %v", err)
	}
	if user.Name != name || user.Email != email {
		t.Errorf("unexpected user after update: %+v", user)
	}
	if user.Password != created.Password {
		t.Error("password must not change on update")
	}
}

func TestDeleteUser(t *testing.T) {
	s, _ := newTestService()

	created, _ := s.Register(context.Background(), CreateUserInput{
		Name:     "MOCK",
		Email:    "mock@example.com",
		Password: "mock-pass",
	})

	err := s.DeleteUser(context.Background(), created.ID.Hex())

	if errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected delete success, got %v", err)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	s, _ := newTestService()

	err := s.DeleteUser(context.Background(), primitive.NewObjectID().Hex())

	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
