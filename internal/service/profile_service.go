// internal/service/profile_service.go
package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ─── Sentinel errors (bisa ditangkap di handler dengan errors.Is) ─────────────
var (
	ErrEmailAlreadyUsed    = errors.New("email sudah digunakan akun lain")
	ErrWrongPassword       = errors.New("password lama tidak sesuai")
	ErrUserNotFound        = errors.New("user tidak ditemukan")
	ErrInvalidFileType     = errors.New("format tidak didukung, gunakan JPG PNG atau WebP")
	ErrFileTooLarge        = errors.New("ukuran file maksimal 2 MB")
)

type ProfileService interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req *dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req *dto.ChangePasswordRequest) error
	UploadAvatar(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader) (*dto.AvatarResponse, error)
	DeleteAccount(ctx context.Context, userID uuid.UUID) error
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
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		AvatarURL:    user.AvatarURL,
		CreatedAt:    user.CreatedAt,
		TotalBalance: balance,
		TotalTx:      txCount,
		TotalWallet:  walletCount,
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
	// Cek duplikat email
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

var allowedAvatarTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func (s *profileService) UploadAvatar(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader) (*dto.AvatarResponse, error) {
	// Validasi tipe MIME
	contentType := file.Header.Get("Content-Type")
	ext, ok := allowedAvatarTypes[contentType]
	if !ok {
		return nil, ErrInvalidFileType
	}

	// Validasi ukuran (maks 2 MB)
	if file.Size > 2*1024*1024 {
		return nil, ErrFileTooLarge
	}

	// Buat nama file unik agar tidak bertabrakan
	filename := fmt.Sprintf("avatar_%s_%s%s",
		userID.String(),
		time.Now().Format("20060102150405"),
		ext,
	)

	// Pastikan direktori tersedia
	uploadDir := "./uploads/avatars"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("gagal menyiapkan direktori upload: %w", err)
	}

	// Simpan file — NOTE: SaveUploadedFile ada di *gin.Context, di sini kita
	// buka manual karena service tidak boleh bergantung pada gin.
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("gagal membuka file: %w", err)
	}
	defer src.Close()

	savePath := filepath.Join(uploadDir, filename)
	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan file: %w", err)
	}
	defer dst.Close()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return nil, fmt.Errorf("gagal menulis file: %w", writeErr)
			}
		}
		if readErr != nil {
			break
		}
	}

	avatarURL := "/uploads/avatars/" + filename

	// Update database & hapus file lama
	user, err := s.profileRepo.UpdateAvatar(ctx, userID, avatarURL)
	if err != nil {
		// Rollback: hapus file yang baru saja disimpan
		os.Remove(savePath)
		return nil, err
	}
	_ = user

	return &dto.AvatarResponse{AvatarURL: avatarURL}, nil
}

// ─── DeleteAccount ────────────────────────────────────────────────────────────

func (s *profileService) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return ErrUserNotFound
	}

	// Hapus avatar dari disk jika ada
	if user.AvatarURL != "" {
		os.Remove("." + user.AvatarURL)
	}

	return s.profileRepo.DeleteAccount(ctx, userID)
}