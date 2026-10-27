// internal/service/profile_service.go
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/pkg/budgetcycle"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ─── Sentinel errors (bisa ditangkap di handler dengan errors.Is) ─────────────
var (
	ErrEmailAlreadyUsed  = errors.New("email sudah digunakan akun lain")
	ErrWrongPassword     = errors.New("password lama tidak sesuai")
	ErrUserNotFound      = errors.New("user tidak ditemukan")
	ErrInvalidFileType   = errors.New("format tidak didukung, gunakan JPG PNG atau WebP")
	ErrFileTooLarge      = errors.New("ukuran file maksimal 2 MB")
	ErrDailyCategory     = errors.New("kategori jatah harian harus kategori pengeluaran milikmu")
	ErrDeletePassword    = errors.New("password salah, akun tidak dihapus")
	ErrAdminCannotDelete = errors.New("akun admin tidak bisa dihapus dari profil")
)

type ProfileService interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req *dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req *dto.ChangePasswordRequest) error
	UploadAvatar(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader) (*dto.AvatarResponse, error)
	DeleteAccount(ctx context.Context, userID uuid.UUID, password string) error
	UpdateBudgetCycle(ctx context.Context, userID uuid.UUID, req *dto.UpdateBudgetCycleRequest) (*dto.ProfileResponse, error)
	UpdateIncome(ctx context.Context, userID uuid.UUID, req *dto.UpdateIncomeRequest) (*dto.ProfileResponse, error)
	UpdateDailyCategories(ctx context.Context, userID uuid.UUID, req *dto.UpdateDailyCategoriesRequest) (*dto.ProfileResponse, error)
}

type profileService struct {
	profileRepo repository.ProfileRepository
	userRepo    repository.UserRepository
}

func NewProfileService(
	profileRepo repository.ProfileRepository,
	userRepo repository.UserRepository,
) ProfileService {
	return &profileService{
		profileRepo: profileRepo,
		userRepo:    userRepo,
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *profileService) buildResponse(ctx context.Context, user *model.User) (*dto.ProfileResponse, error) {
	balance, err := s.profileRepo.SumWalletBalance(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	txCount, err := s.profileRepo.CountTransactions(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	walletCount, err := s.profileRepo.CountWallets(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &dto.ProfileResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
		CreatedAt: user.CreatedAt,

		BudgetStartDay:   budgetcycle.Normalize(user.BudgetStartDay),
		IncomeType:       incomeTypeOf(user),
		IncomeSetupDone:  user.IncomeSetupAt != nil,
		DailyCategoryIDs: dailyCategoryIDs(user),
		LastLoginAt:      user.LastLoginAt,
		LastLoginDevice:  user.LastLoginDevice,
		Role:             user.Role,
		Status:           user.Status,
		TotalBalance:     balance,
		TotalTx:          txCount,
		TotalWallet:      walletCount,
	}, nil
}

// ─── GetProfile ───────────────────────────────────────────────────────────────

func (s *profileService) GetProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	return s.buildResponse(ctx, user)
}

// ─── UpdateProfile ────────────────────────────────────────────────────────────

func (s *profileService) UpdateProfile(ctx context.Context, userID uuid.UUID, req *dto.UpdateProfileRequest) (*dto.ProfileResponse, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Email = normalizeEmail(req.Email)

	exists, err := s.profileRepo.EmailExistsExcept(ctx, req.Email, userID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyUsed
	}

	user, err := s.profileRepo.UpdateProfile(ctx, userID, req.Name, req.Email)
	if err != nil {
		return nil, err
	}
	return s.buildResponse(ctx, user)
}

// ─── ChangePassword ───────────────────────────────────────────────────────────

func (s *profileService) ChangePassword(ctx context.Context, userID uuid.UUID, req *dto.ChangePasswordRequest) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return ErrUserNotFound
	}

	// Verifikasi password lama
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return ErrWrongPassword
	}

	// Hash password baru (cost 12 adalah standar yang aman)
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
	if err != nil {
		return fmt.Errorf("gagal memproses password: %w", err)
	}

	return s.profileRepo.UpdatePassword(ctx, userID, string(hashed))
}

// ─── UploadAvatar ─────────────────────────────────────────────────────────────

const (
	maxAvatarBytes = 2 * 1024 * 1024
	maxAvatarSide  = 6000
)

var allowedAvatarTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func (s *profileService) UploadAvatar(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader) (*dto.AvatarResponse, error) {
	if file.Size > maxAvatarBytes {
		return nil, ErrFileTooLarge
	}
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("gagal membuka file: %w", err)
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, maxAvatarBytes+1))
	if err != nil {
		return nil, fmt.Errorf("gagal membaca file: %w", err)
	}
	if len(data) > maxAvatarBytes {
		return nil, ErrFileTooLarge
	}

	// Tipe ditentukan dari isi file, bukan dari header yang dikirim browser.
	ext, ok := allowedAvatarTypes[http.DetectContentType(data)]
	if !ok {
		return nil, ErrInvalidFileType
	}
	if ext != ".webp" {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width > maxAvatarSide || cfg.Height > maxAvatarSide {
			return nil, ErrInvalidFileType
		}
	}

	name := make([]byte, 16)
	if _, err := rand.Read(name); err != nil {
		return nil, err
	}
	filename := "avatar_" + hex.EncodeToString(name) + ext

	uploadDir := "./uploads/avatars"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("gagal menyiapkan direktori upload: %w", err)
	}
	savePath := filepath.Join(uploadDir, filename)
	if err := os.WriteFile(savePath, data, 0644); err != nil {
		return nil, fmt.Errorf("gagal menyimpan file: %w", err)
	}

	avatarURL := "/uploads/avatars/" + filename
	if _, err := s.profileRepo.UpdateAvatar(ctx, userID, avatarURL); err != nil {
		os.Remove(savePath)
		return nil, err
	}
	return &dto.AvatarResponse{AvatarURL: avatarURL}, nil
}

// ─── DeleteAccount ────────────────────────────────────────────────────────────

func (s *profileService) DeleteAccount(ctx context.Context, userID uuid.UUID, password string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return ErrUserNotFound
	}
	if user.Role == model.RoleAdmin {
		return ErrAdminCannotDelete
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return ErrDeletePassword
	}
	if err := s.profileRepo.DeleteAccount(ctx, userID); err != nil {
		return err
	}
	removeAvatar(user.AvatarURL)
	return nil
}

func removeAvatar(url string) {
	if url != "" {
		os.Remove("." + url)
	}
}

func incomeTypeOf(user *model.User) string {
	if user.IncomeType == model.IncomeIrregular {
		return model.IncomeIrregular
	}
	return model.IncomeSalary
}

// ─── UpdateIncome ─────────────────────────────────────────────────────────────

func (s *profileService) UpdateIncome(ctx context.Context, userID uuid.UUID, req *dto.UpdateIncomeRequest) (*dto.ProfileResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	day := req.StartDay
	switch {
	case req.IncomeType == model.IncomeIrregular:
		day = 1
	case day == 0:
		day = budgetcycle.Normalize(user.BudgetStartDay)
	}
	if err := s.profileRepo.UpdateIncome(ctx, userID, req.IncomeType, day); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, userID)
}

// ─── UpdateDailyCategories ────────────────────────────────────────────────────

func dailyCategoryIDs(user *model.User) []uuid.UUID {
	if user.DailyCategoryIDs == nil {
		return []uuid.UUID{}
	}
	return user.DailyCategoryIDs
}

func (s *profileService) UpdateDailyCategories(ctx context.Context, userID uuid.UUID, req *dto.UpdateDailyCategoriesRequest) (*dto.ProfileResponse, error) {
	ids := make([]uuid.UUID, 0, len(req.CategoryIDs))
	seen := map[uuid.UUID]bool{}
	for _, id := range req.CategoryIDs {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		n, err := s.profileRepo.CountExpenseCategories(ctx, userID, ids)
		if err != nil {
			return nil, err
		}
		if n != int64(len(ids)) {
			return nil, ErrDailyCategory
		}
	}
	if err := s.profileRepo.UpdateDailyCategories(ctx, userID, ids); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, userID)
}

// ─── UpdateBudgetCycle ────────────────────────────────────────────────────────

func (s *profileService) UpdateBudgetCycle(ctx context.Context, userID uuid.UUID, req *dto.UpdateBudgetCycleRequest) (*dto.ProfileResponse, error) {
	if err := s.profileRepo.UpdateBudgetStartDay(ctx, userID, req.StartDay); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, userID)
}
