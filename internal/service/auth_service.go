package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/pkg/config"
	"dompetku/pkg/crypto"
	"dompetku/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const minPasswordLength = 8

var (
	ErrAccountPending     = errors.New("akun kamu menunggu persetujuan admin")
	ErrAccountRejected    = errors.New("pendaftaran akun ini ditolak admin")
	ErrAccountSuspended   = errors.New("akun ini dinonaktifkan admin")
	ErrInviteInvalid      = errors.New("kode undangan tidak valid, sudah dipakai, atau sudah kedaluwarsa")
	ErrResetTokenInvalid  = errors.New("link atur ulang password tidak valid atau sudah kedaluwarsa")
	ErrInvalidCredentials = errors.New("email atau password salah")
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest, client dto.ClientInfo) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req dto.LoginRequest, client dto.ClientInfo) (*dto.AuthResponse, error)
	CheckInvite(code string) (*dto.InviteCheckResponse, error)
	ResetPassword(req dto.ResetPasswordRequest) error
}

type authService struct {
	db       *gorm.DB
	userRepo repository.UserRepository
	notif    NotificationService
	cfg      *config.AppConf
}

func NewAuthService(db *gorm.DB, userRepo repository.UserRepository, notif NotificationService, cfg *config.AppConf) AuthService {
	return &authService{db: db, userRepo: userRepo, notif: notif, cfg: cfg}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeInvite(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func validatePassword(p string) error {
	if len(p) < minPasswordLength || len(p) > 72 {
		return errors.New("password harus 8 sampai 72 karakter")
	}
	return nil
}

// StatusError memetakan status akun non-aktif ke error yang bisa ditampilkan.
func StatusError(status string) error {
	switch status {
	case model.UserPending:
		return ErrAccountPending
	case model.UserRejected:
		return ErrAccountRejected
	case model.UserSuspended:
		return ErrAccountSuspended
	}
	return nil
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest, client dto.ClientInfo) (*dto.RegisterResponse, error) {
	name := strings.TrimSpace(req.Name)
	email := normalizeEmail(req.Email)

	if name == "" || email == "" || req.Password == "" {
		return nil, errors.New("nama, email, dan password wajib diisi")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, errors.New("format email tidak valid")
	}
	if err := validatePassword(req.Password); err != nil {
		return nil, err
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

	user := &model.User{Name: name, Email: email, Password: hashedPassword, Role: model.RoleUser, Status: model.UserPending}
	code := normalizeInvite(req.InviteCode)
	now := time.Now()

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch {
		case s.cfg.IsAdminEmail(email):
			user.Role, user.Status, user.ApprovedAt = model.RoleAdmin, model.UserActive, &now
		case code != "":
			var invite model.Invite
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&invite).Error
			if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !invite.Usable(now)) {
				return ErrInviteInvalid
			}
			if err != nil {
				return err
			}
			if err := tx.Model(&invite).UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error; err != nil {
				return err
			}
			user.Status, user.ApprovedAt, user.ApprovedBy, user.InviteID = model.UserActive, &now, &invite.CreatedBy, &invite.ID
		}
		return tx.Create(user).Error
	})
	if err != nil {
		return nil, err
	}

	resp := &dto.RegisterResponse{Status: user.Status, User: userResponse(user)}
	if user.Status != model.UserActive {
		s.notifyAdmins(user)
		return resp, nil
	}
	token, err := s.startSession(ctx, user, client)
	if err != nil {
		return nil, err
	}
	resp.Token = token
	return resp, nil
}

func (s *authService) notifyAdmins(user *model.User) {
	var admins []model.User
	if err := s.db.Where("role = ? AND status = ?", model.RoleAdmin, model.UserActive).Find(&admins).Error; err != nil {
		log.Printf("[auth] gagal mengambil admin: %v", err)
		return
	}
	for _, a := range admins {
		msg := fmt.Sprintf("%s (%s) mendaftar dan menunggu persetujuan.", user.Name, user.Email)
		if err := s.notif.CreateSystem(a.ID, "Pendaftar baru", msg); err != nil {
			log.Printf("[auth] gagal notifikasi admin: %v", err)
		}
	}
}

// startSession menaikkan versi token sehingga sesi di perangkat lain otomatis
// berakhir: satu akun hanya aktif di satu perangkat.
func (s *authService) startSession(ctx context.Context, user *model.User, client dto.ClientInfo) (string, error) {
	var version int
	err := s.db.WithContext(ctx).Raw(`UPDATE users
		SET token_version = token_version + 1, session_reason = ?, last_login_at = ?, last_login_device = ?, last_login_ip = ?
		WHERE id = ? RETURNING token_version`,
		model.SessionLogin, time.Now(), truncate(utils.DescribeDevice(client.UserAgent), 120), truncate(client.IP, 64), user.ID).
		Scan(&version).Error
	if err != nil {
		return "", err
	}
	user.TokenVersion = version
	return utils.GenerateToken(user.ID, user.Email, s.cfg.JWTSecret, version)
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest, client dto.ClientInfo) (*dto.AuthResponse, error) {
	email := normalizeEmail(req.Email)
	if email == "" || req.Password == "" {
		return nil, errors.New("email dan password wajib diisi")
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil || user == nil {
		return nil, ErrInvalidCredentials
	}
	if !crypto.CheckPasswordHash(req.Password, user.Password) {
		return nil, ErrInvalidCredentials
	}
	if err := StatusError(user.Status); err != nil {
		return nil, err
	}

	token, err := s.startSession(ctx, user, client)
	if err != nil {
		return nil, err
	}
	return &dto.AuthResponse{Token: token, User: userResponse(user)}, nil
}

func userResponse(user *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        user.ID.String(),
		Name:      user.Name,
		Email:     user.Email,
		Role:      user.Role,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (s *authService) CheckInvite(code string) (*dto.InviteCheckResponse, error) {
	var invite model.Invite
	err := s.db.Where("code = ?", normalizeInvite(code)).First(&invite).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &dto.InviteCheckResponse{Valid: false}, nil
	}
	if err != nil {
		return nil, err
	}
	if !invite.Usable(time.Now()) {
		return &dto.InviteCheckResponse{Valid: false}, nil
	}
	return &dto.InviteCheckResponse{Valid: true, ExpiresAt: invite.ExpiresAt.Format(time.RFC3339)}, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *authService) ResetPassword(req dto.ResetPasswordRequest) error {
	if err := validatePassword(req.Password); err != nil {
		return err
	}
	hashed, err := crypto.HashPassword(req.Password)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var t model.PasswordResetToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", hashToken(strings.TrimSpace(req.Token)), now).
			First(&t).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrResetTokenInvalid
		}
		if err != nil {
			return err
		}
		res := tx.Model(&model.User{}).Where("id = ?", t.UserID).Updates(map[string]any{
			"password":       hashed,
			"token_version":  gorm.Expr("token_version + 1"),
			"session_reason": model.SessionPassword,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrResetTokenInvalid
		}
		return tx.Model(&t).UpdateColumn("used_at", now).Error
	})
}
