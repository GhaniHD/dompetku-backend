package service

import (
	"context"
	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/pkg/config"
	"dompetku/pkg/crypto"
	"dompetku/pkg/utils"
	"errors"
	"net/mail"
	"strings"
)

const minPasswordLength = 8

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error)
}

type authService struct {
	userRepo repository.UserRepository
	cfg      *config.AppConf
}

func NewAuthService(userRepo repository.UserRepository, cfg *config.AppConf) AuthService {
	return &authService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	name := strings.TrimSpace(req.Name)
	email := normalizeEmail(req.Email)

	if name == "" || email == "" || req.Password == "" {
		return nil, errors.New("nama, email, dan password wajib diisi")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, errors.New("format email tidak valid")
	}
	if len(req.Password) < minPasswordLength || len(req.Password) > 72 {
		return nil, errors.New("password harus 8 sampai 72 karakter")
	}

	taken, err := s.userRepo.EmailTaken(ctx, email)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errors.New("email sudah terdaftar")
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Name:     name,
		Email:    email,
		Password: hashedPassword,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return s.buildAuthResponse(user)
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, error) {
	email := normalizeEmail(req.Email)
	if email == "" || req.Password == "" {
		return nil, errors.New("email dan password wajib diisi")
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, errors.New("email atau password salah")
	}

	if !crypto.CheckPasswordHash(req.Password, user.Password) {
		return nil, errors.New("email atau password salah")
	}

	return s.buildAuthResponse(user)
}

func (s *authService) buildAuthResponse(user *model.User) (*dto.AuthResponse, error) {
	token, err := utils.GenerateToken(user.ID, user.Email, s.cfg.JWTSecret)
	if err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		Token: token,
		User: dto.UserResponse{
			ID:        user.ID.String(),
			Name:      user.Name,
			Email:     user.Email,
			CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		},
	}, nil
}
