package service

import (
	"context"
	"errors"
	"net/mail"
	"time"

	"github.com/theWebPartyTime/server/internal/models"
	"github.com/theWebPartyTime/server/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo repository.UserRepository
}

func NewAuthService(userRepo repository.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.User, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if user == nil {
		return nil, err
	}
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !CheckPasswordHash(req.Password, user.PasswordHash) {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.User, error) {
	_, err := mail.ParseAddress(req.Email)
	if err != nil {
		return nil, errors.New("invalid email")
	}
	email := req.Email
	password, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	user := &models.User{
		Email:        email,
		PasswordHash: password,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = s.userRepo.CreateUser(ctx, user)
	if err != nil {
		return nil, errors.New("email already in use")
	}
	return user, nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
