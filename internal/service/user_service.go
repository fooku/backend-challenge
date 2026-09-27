package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"backend-challenge/internal/model"
	"backend-challenge/internal/repository"
)

var (
	ErrUserNotFound       = repository.ErrNotFound
	ErrEmailExists        = repository.ErrDuplicateEmail
	ErrInvalidID          = repository.ErrInvalidID
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type CreateUserInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UpdateUserInput only allows name and email. Nil fields are left unchanged.
type UpdateUserInput struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

type UserService struct {
	userRepo  repository.UserInterfaceRepository
	jwtSecret string
	tokenTTL  time.Duration
}

func NewUserService(repo repository.UserInterfaceRepository, jwtSecret string, tokenTTL time.Duration) *UserService {
	return &UserService{userRepo: repo, jwtSecret: jwtSecret, tokenTTL: tokenTTL}
}

func (s *UserService) Register(ctx context.Context, in CreateUserInput) (*model.User, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = normalizeEmail(in.Email)

	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	if in.Email == "" {
		return nil, errors.New("email is required")
	}
	if in.Password == "" {
		return nil, errors.New("password is required")
	}

	_, err := s.userRepo.GetByEmail(ctx, in.Email)
	if err == nil {
		return nil, ErrEmailExists
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("password is too long")
	}

	user := &model.User{
		Name:      in.Name,
		Email:     in.Email,
		Password:  string(hash),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login checks the credentials and returns a signed JWT.
func (s *UserService) Login(ctx context.Context, email, password string) (string, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return "", errors.New("email and password are required")
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return "", ErrInvalidCredentials
	}

	return GenerateToken(s.jwtSecret, user.ID.Hex(), s.tokenTTL)
}

func (s *UserService) GetUser(ctx context.Context, id string) (*model.User, error) {
	return s.userRepo.GetByID(ctx, id)
}

func (s *UserService) ListUsers(ctx context.Context) ([]model.User, error) {
	return s.userRepo.List(ctx)
}

func (s *UserService) UpdateUser(ctx context.Context, id string, in UpdateUserInput) (*model.User, error) {
	if in.Name == nil && in.Email == nil {
		return nil, errors.New("name or email is required")
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, errors.New("name is required")
		}
		user.Name = name
	}

	if in.Email != nil {
		email := normalizeEmail(*in.Email)
		if email == "" {
			return nil, errors.New("email is required")
		}
		if email != user.Email {
			existing, err := s.userRepo.GetByEmail(ctx, email)
			if err == nil && existing.ID != user.ID {
				return nil, ErrEmailExists
			}
			if err != nil && !errors.Is(err, repository.ErrNotFound) {
				return nil, err
			}
		}
		user.Email = email
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	return s.userRepo.Delete(ctx, id)
}

func (s *UserService) CountUsers(ctx context.Context) (int64, error) {
	return s.userRepo.Count(ctx)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
