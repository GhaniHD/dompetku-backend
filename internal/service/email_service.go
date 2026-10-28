package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/pkg/config"
	"dompetku/pkg/crypto"
	"dompetku/pkg/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	codeTTL          = 15 * time.Minute
	codeMaxAttempts  = 5
	codeCooldown     = 60 * time.Second
	codeHourlyMax    = 5
	verifyGrace      = 7 * 24 * time.Hour
	challengeTTL     = 30 * time.Minute
	selfResetTTL     = 30 * time.Minute
	selfResetHourly  = 3
	scheduledHourWIB = 7
)

var (
	ErrMailDisabled       = errors.New("fitur email belum aktif")
	ErrCodeWrong          = errors.New("kode verifikasi salah")
	ErrCodeExpired        = errors.New("kode sudah kedaluwarsa atau sudah diganti, minta kode baru")
	ErrCodeLocked         = errors.New("terlalu banyak salah memasukkan kode, minta kode baru")
	ErrChallengeInvalid   = errors.New("sesi verifikasi berakhir, silakan masuk lagi")
	ErrAlreadyVerified    = errors.New("email sudah terverifikasi")
	ErrEmailSame          = errors.New("email baru sama dengan email sekarang")
	ErrNoPendingEmail     = errors.New("tidak ada email baru yang menunggu verifikasi")
	ErrEmailReserved      = errors.New("email ini tidak bisa dipakai")
	ErrEmailPassword      = errors.New("password salah, email tidak diganti")
	ErrUnsubscribeInvalid = errors.New("link berhenti langganan tidak valid")
	ErrWebhookSecret      = errors.New("webhook tidak dikenali")
)

// CooldownError: permintaan kode terlalu cepat atau terlalu sering.
type CooldownError struct {
	RetryAfter int
	msg        string
}

func (e *CooldownError) Error() string { return e.msg }

// VerificationRequiredError: password benar, tetapi email harus diverifikasi
// dulu sebelum sesi dibuat.
type VerificationRequiredError struct {
	VerifyToken string
	Email       string
	ResendIn    int
}

func (e *VerificationRequiredError) Error() string {
	return "masukkan kode verifikasi yang kami kirim ke emailmu"
}

type EmailService interface {
	Enabled() bool
	Status(u *model.User) dto.EmailStatus
	RequiresVerification(u *model.User) bool
	EnsureDue(u *model.User) time.Time
	Challenge(u *model.User) *VerificationRequiredError
	ChallengeUser(token string) (uuid.UUID, error)
	SendCode(userID uuid.UUID, purpose string) (*dto.CodeSentResponse, error)
	ConfirmCode(userID uuid.UUID, purpose, code string, client dto.ClientInfo) (*model.User, error)
	SendApproved(userID uuid.UUID)
	NotifyNewSignup(user *model.User, admins []model.User)
	LoginAlert(u *model.User, prevDevice string, client dto.ClientInfo)
	ForgotPassword(email string) error
	StartEmailChange(ctx context.Context, userID uuid.UUID, req dto.ChangeEmailRequest, client dto.ClientInfo) (*dto.CodeSentResponse, error)
	CancelEmailChange(userID uuid.UUID) error
	UpdatePrefs(userID uuid.UUID, prefs dto.EmailPrefs) error
	Unsubscribe(token string) (*dto.UnsubscribeResponse, error)
	HandleBrevo(secret string, events []dto.BrevoEvent) error
	MarkVerified(adminID, userID uuid.UUID) error
	Stats() (*dto.EmailStatsResponse, error)
	RunScheduled(now time.Time)
}

type emailService struct {
	db       *gorm.DB
	mailer   Mailer
	outbox   Outbox
	cfg      *config.AppConf
	activity Activity
	budgets  BudgetService
}

func NewEmailService(db *gorm.DB, mailer Mailer, outbox Outbox, cfg *config.AppConf, activity Activity, budgets BudgetService) EmailService {
	return &emailService{db: db, mailer: mailer, outbox: outbox, cfg: cfg, activity: activity, budgets: budgets}
}

func (s *emailService) Enabled() bool { return s.mailer.Enabled() }

// ─── Status ──────────────────────────────────────────────────────────────────

func emailStatus(u *model.User, enabled bool) dto.EmailStatus {
	return dto.EmailStatus{
		MailEnabled:  enabled,
		Verified:     u.EmailVerifiedAt != nil,
		VerifiedAt:   u.EmailVerifiedAt,
		Required:     enabled && u.Role != model.RoleAdmin && u.EmailVerifiedAt == nil,
		DueAt:        u.EmailVerifyDueAt,
		PendingEmail: u.PendingEmail,
		Bounced:      u.EmailBouncedAt != nil,
		Prefs: dto.EmailPrefs{
			Weekly:     !u.EmailPrefs.WeeklyOff,
			LoginAlert: !u.EmailPrefs.LoginAlertOff,
			Reminders:  !u.EmailPrefs.RemindersOff,
		},
	}
}

func (s *emailService) Status(u *model.User) dto.EmailStatus { return emailStatus(u, s.Enabled()) }

func (s *emailService) RequiresVerification(u *model.User) bool {
	return s.Enabled() && u.Role != model.RoleAdmin && u.EmailVerifiedAt == nil
}

// EnsureDue memberi akun lama masa tenggang 7 hari sejak login pertama
// setelah fitur ini aktif.
func (s *emailService) EnsureDue(u *model.User) time.Time {
	if u.EmailVerifyDueAt != nil {
		return *u.EmailVerifyDueAt
	}
	due := time.Now().Add(verifyGrace)
	if err := s.db.Model(&model.User{}).Where("id = ? AND email_verify_due_at IS NULL", u.ID).Update("email_verify_due_at", due).Error; err != nil {
		log.Printf("[mail] gagal set batas verifikasi: %v", err)
	}
	u.EmailVerifyDueAt = &due
	return due
}

// ─── Token bertanda tangan ───────────────────────────────────────────────────

func (s *emailService) sign(msg string) string {
	m := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))[:40]
}

func (s *emailService) signed(scope, payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + s.sign(scope+"|"+payload)
}

func (s *emailService) unsign(scope, token string) (string, bool) {
	raw, sig, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok {
		return "", false
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", false
	}
	payload := string(b)
	if !hmac.Equal([]byte(sig), []byte(s.sign(scope+"|"+payload))) {
		return "", false
	}
	return payload, true
}

func (s *emailService) Challenge(u *model.User) *VerificationRequiredError {
	exp := time.Now().Add(challengeTTL).Unix()
	out := &VerificationRequiredError{
		VerifyToken: s.signed("challenge", u.ID.String()+"|"+strconv.FormatInt(exp, 10)),
		Email:       maskEmail(u.Email),
	}
	sent, err := s.SendCode(u.ID, model.CodeVerify)
	var cd *CooldownError
	switch {
	case err == nil:
		out.ResendIn = sent.ResendIn
	case errors.As(err, &cd):
		out.ResendIn = cd.RetryAfter
	default:
		log.Printf("[mail] gagal kirim kode login: %v", err)
	}
	return out
}

func (s *emailService) ChallengeUser(token string) (uuid.UUID, error) {
	payload, ok := s.unsign("challenge", token)
	if !ok {
		return uuid.Nil, ErrChallengeInvalid
	}
	idPart, expPart, _ := strings.Cut(payload, "|")
	exp, err := strconv.ParseInt(expPart, 10, 64)
	id, idErr := uuid.Parse(idPart)
	if err != nil || idErr != nil || time.Now().Unix() > exp {
		return uuid.Nil, ErrChallengeInvalid
	}
	return id, nil
}

// ─── Kode OTP ────────────────────────────────────────────────────────────────

func sixDigits() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func (s *emailService) codeHash(userID uuid.UUID, purpose, email, code string) string {
	return s.sign("otp|" + userID.String() + "|" + purpose + "|" + strings.ToLower(email) + "|" + code)
}

func codeTarget(u *model.User, purpose string) (string, string, error) {
	if purpose == "" {
		purpose = model.CodeVerify
		if u.PendingEmail != "" {
			purpose = model.CodeChange
		}
	}
	switch purpose {
	case model.CodeChange:
		if u.PendingEmail == "" {
			return "", "", ErrNoPendingEmail
		}
		return purpose, u.PendingEmail, nil
	default:
		if u.EmailVerifiedAt != nil {
			return "", "", ErrAlreadyVerified
		}
		return model.CodeVerify, u.Email, nil
	}
}

// issueCode membuat kode baru (kode lama otomatis tidak berlaku) setelah
// memeriksa jeda kirim ulang dan batas per jam.
func (s *emailService) issueCode(tx *gorm.DB, userID uuid.UUID, purpose, email string) (string, error) {
	now := time.Now()
	var recent []model.EmailCode
	if err := tx.Where("user_id = ? AND created_at > ?", userID, now.Add(-time.Hour)).Order("created_at DESC").Find(&recent).Error; err != nil {
		return "", err
	}
	for _, c := range recent {
		if !strings.EqualFold(c.Email, email) {
			continue
		}
		if wait := codeCooldown - now.Sub(c.CreatedAt); wait > 0 {
			sec := int(math.Ceil(wait.Seconds()))
			return "", &CooldownError{RetryAfter: sec, msg: fmt.Sprintf("tunggu %d detik sebelum minta kode lagi", sec)}
		}
		break
	}
	if len(recent) >= codeHourlyMax {
		wait := recent[len(recent)-1].CreatedAt.Add(time.Hour).Sub(now)
		min := int(math.Ceil(wait.Minutes()))
		return "", &CooldownError{RetryAfter: int(math.Ceil(wait.Seconds())), msg: fmt.Sprintf("batas kirim kode tercapai, coba lagi dalam %d menit", min)}
	}
	if err := tx.Model(&model.EmailCode{}).Where("user_id = ? AND used_at IS NULL", userID).Update("used_at", now).Error; err != nil {
		return "", err
	}
	code := sixDigits()
	err := tx.Create(&model.EmailCode{
		UserID: userID, Email: email, Purpose: purpose,
		CodeHash: s.codeHash(userID, purpose, email, code), ExpiresAt: now.Add(codeTTL),
	}).Error
	return code, err
}

func (s *emailService) lockUser(tx *gorm.DB, userID uuid.UUID) (*model.User, error) {
	var u model.User
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func (s *emailService) SendCode(userID uuid.UUID, purpose string) (*dto.CodeSentResponse, error) {
	if !s.Enabled() {
		return nil, ErrMailDisabled
	}
	var user *model.User
	var code, to string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		u, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		user = u
		purpose, to, err = codeTarget(u, purpose)
		if err != nil {
			return err
		}
		code, err = s.issueCode(tx, userID, purpose, to)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := s.outbox.Enqueue(&userID, to, model.MailKindCode, 0, s.codeEmail(user, purpose, code)); err != nil {
		return nil, err
	}
	return &dto.CodeSentResponse{Email: maskEmail(to), ResendIn: int(codeCooldown.Seconds()), ExpiresIn: int(codeTTL.Seconds()), Sent: true}, nil
}

func cleanCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, code)
}

// ConfirmCode mencocokkan kode. Percobaan salah tetap tercatat walau
// hasilnya error, jadi hitungan disimpan di luar rollback.
func (s *emailService) ConfirmCode(userID uuid.UUID, purpose, code string, client dto.ClientInfo) (*model.User, error) {
	if !s.Enabled() {
		return nil, ErrMailDisabled
	}
	code = cleanCode(code)
	now := time.Now()
	var result error
	var user *model.User
	var oldEmail string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		u, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		purpose, target, err := codeTarget(u, purpose)
		if err != nil {
			return err
		}
		var c model.EmailCode
		err = tx.Where("user_id = ? AND purpose = ? AND email = ? AND used_at IS NULL", userID, purpose, target).
			Order("created_at DESC").First(&c).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && c.ExpiresAt.Before(now)) {
			result = ErrCodeExpired
			return nil
		}
		if err != nil {
			return err
		}
		if c.Attempts >= codeMaxAttempts {
			result = ErrCodeLocked
			return nil
		}
		if len(code) != 6 || !hmac.Equal([]byte(c.CodeHash), []byte(s.codeHash(userID, purpose, target, code))) {
			c.Attempts++
			if err := tx.Model(&c).Update("attempts", c.Attempts).Error; err != nil {
				return err
			}
			if left := codeMaxAttempts - c.Attempts; left > 0 {
				result = fmt.Errorf("%w, sisa %d percobaan", ErrCodeWrong, left)
			} else {
				result = ErrCodeLocked
			}
			return nil
		}
		if err := tx.Model(&c).Update("used_at", now).Error; err != nil {
			return err
		}
		updates := map[string]any{"email_verified_at": now, "email_bounced_at": nil}
		if purpose == model.CodeChange {
			var taken int64
			if err := tx.Model(&model.User{}).Unscoped().Where("LOWER(email) = ? AND id <> ?", strings.ToLower(target), userID).Count(&taken).Error; err != nil {
				return err
			}
			if taken > 0 {
				if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("pending_email", "").Error; err != nil {
					return err
				}
				result = ErrEmailAlreadyUsed
				return nil
			}
			oldEmail = u.Email
			updates["email"] = target
			updates["pending_email"] = ""
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			return err
		}
		user, err = s.lockUser(tx, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if result != nil {
		return nil, result
	}
	if oldEmail != "" {
		s.activity.Record(&userID, user.Email, model.EventEmailChanged, maskEmail(oldEmail)+" → "+maskEmail(user.Email), client)
		if err := s.outbox.Enqueue(&userID, oldEmail, model.MailKindEmailChanged, 0, s.emailChangedEmail(user, now)); err != nil {
			log.Printf("[mail] gagal antre pemberitahuan ganti email: %v", err)
		}
	} else {
		s.activity.Record(&userID, user.Email, model.EventEmailVerified, "", client)
	}
	return user, nil
}

// ─── Ganti email & preferensi ────────────────────────────────────────────────

func (s *emailService) emailTaken(email string, except uuid.UUID) (bool, error) {
	var n int64
	err := s.db.Model(&model.User{}).Unscoped().Where("LOWER(email) = ? AND id <> ?", email, except).Count(&n).Error
	return n > 0, err
}

func (s *emailService) StartEmailChange(ctx context.Context, userID uuid.UUID, req dto.ChangeEmailRequest, client dto.ClientInfo) (*dto.CodeSentResponse, error) {
	var u model.User
	if err := s.db.WithContext(ctx).Where("id = ?", userID).First(&u).Error; err != nil {
		return nil, ErrUserNotFound
	}
	if !crypto.CheckPasswordHash(req.Password, u.Password) {
		return nil, ErrEmailPassword
	}
	email := normalizeEmail(req.Email)
	if email == strings.ToLower(u.Email) {
		return nil, ErrEmailSame
	}
	if u.Role != model.RoleAdmin && s.cfg.IsAdminEmail(email) {
		return nil, ErrEmailReserved
	}
	if err := CheckEmailAddress(ctx, email, s.cfg.EmailDomainCheck); err != nil {
		return nil, err
	}
	taken, err := s.emailTaken(email, userID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrEmailAlreadyUsed
	}

	if !s.Enabled() {
		err := s.db.Model(&model.User{}).Where("id = ?", userID).
			Updates(map[string]any{"email": email, "pending_email": "", "email_verified_at": nil, "email_bounced_at": nil}).Error
		if err != nil {
			return nil, err
		}
		s.activity.Record(&userID, email, model.EventEmailChanged, maskEmail(u.Email)+" → "+maskEmail(email), client)
		return &dto.CodeSentResponse{Email: maskEmail(email)}, nil
	}

	if err := s.db.Model(&model.User{}).Where("id = ?", userID).Update("pending_email", email).Error; err != nil {
		return nil, err
	}
	return s.SendCode(userID, model.CodeChange)
}

func (s *emailService) CancelEmailChange(userID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("pending_email", "").Error; err != nil {
			return err
		}
		return tx.Model(&model.EmailCode{}).Where("user_id = ? AND purpose = ? AND used_at IS NULL", userID, model.CodeChange).Update("used_at", time.Now()).Error
	})
}

func (s *emailService) UpdatePrefs(userID uuid.UUID, p dto.EmailPrefs) error {
	raw, _ := json.Marshal(model.EmailPrefs{WeeklyOff: !p.Weekly, LoginAlertOff: !p.LoginAlert, RemindersOff: !p.Reminders})
	return s.db.Exec("UPDATE users SET email_prefs = ?::jsonb WHERE id = ?", string(raw), userID).Error
}

var unsubKinds = map[string]struct{ field, label string }{
	model.MailKindWeekly:     {"weekly_off", "ringkasan mingguan"},
	model.MailKindLoginAlert: {"login_alert_off", "pemberitahuan login perangkat baru"},
	model.MailKindReminder:   {"reminders_off", "pengingat jatuh tempo"},
}

func (s *emailService) unsubLinks(userID uuid.UUID, kind string) (page, header string) {
	tok := s.signed("unsub", userID.String()+"|"+kind)
	page = s.cfg.AppURL + "/berhenti-langganan?token=" + tok
	if s.cfg.APIURL != "" {
		header = s.cfg.APIURL + "/api/email/unsubscribe?token=" + tok
	}
	return page, header
}

func (s *emailService) Unsubscribe(token string) (*dto.UnsubscribeResponse, error) {
	payload, ok := s.unsign("unsub", token)
	if !ok {
		return nil, ErrUnsubscribeInvalid
	}
	idPart, kind, _ := strings.Cut(payload, "|")
	id, err := uuid.Parse(idPart)
	info, known := unsubKinds[kind]
	if err != nil || !known {
		return nil, ErrUnsubscribeInvalid
	}
	err = s.db.Exec("UPDATE users SET email_prefs = email_prefs || jsonb_build_object(?::text, true) WHERE id = ?", info.field, id).Error
	if err != nil {
		return nil, err
	}
	return &dto.UnsubscribeResponse{Kind: kind, Label: info.label}, nil
}

// ─── Webhook Brevo ───────────────────────────────────────────────────────────

func (s *emailService) HandleBrevo(secret string, events []dto.BrevoEvent) error {
	if s.cfg.BrevoWebhookSecret == "" || !hmac.Equal([]byte(secret), []byte(s.cfg.BrevoWebhookSecret)) {
		return ErrWebhookSecret
	}
	for _, e := range events {
		email := normalizeEmail(e.Email)
		if email == "" {
			continue
		}
		switch strings.ToLower(e.Event) {
		case "hard_bounce", "hardbounce", "invalid_email", "blocked", "spam", "complaint":
			if err := s.db.Exec("UPDATE users SET email_bounced_at = now() WHERE LOWER(email) = ? AND email_bounced_at IS NULL", email).Error; err != nil {
				return err
			}
			if err := s.db.Exec("UPDATE email_outbox SET status = ?, last_error = 'alamat email terpental' WHERE status = ? AND priority > 0 AND LOWER(to_email) = ?",
				model.MailSkipped, model.MailPending, email).Error; err != nil {
				return err
			}
		case "unsubscribed", "unsubscribe":
			raw, _ := json.Marshal(model.EmailPrefs{WeeklyOff: true, LoginAlertOff: true, RemindersOff: true})
			if err := s.db.Exec("UPDATE users SET email_prefs = ?::jsonb WHERE LOWER(email) = ?", string(raw), email).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// ─── Admin ───────────────────────────────────────────────────────────────────

func (s *emailService) MarkVerified(adminID, userID uuid.UUID) error {
	var email string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		u, err := s.lockUser(tx, userID)
		if errors.Is(err, ErrUserNotFound) {
			return ErrAdminUserNotFound
		}
		if err != nil {
			return err
		}
		if u.Role == model.RoleAdmin {
			return ErrAdminProtected
		}
		if u.EmailVerifiedAt != nil {
			return ErrAlreadyVerified
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("email_verified_at", time.Now()).Error; err != nil {
			return err
		}
		email = u.Email
		return tx.Create(&model.AdminLog{AdminID: adminID, Action: "verify_email", TargetUserID: &userID, Detail: maskEmail(u.Email)}).Error
	})
	if err == nil {
		s.activity.Record(&userID, email, model.EventEmailVerified, "ditandai admin", dto.ClientInfo{})
	}
	return err
}

func (s *emailService) Stats() (*dto.EmailStatsResponse, error) {
	now := time.Now()
	out := &dto.EmailStatsResponse{Enabled: s.Enabled(), DailyCap: dailyMailSoftCap, ProviderLimit: 300, ByKind: []dto.EmailKindStat{}, Daily: []dto.EmailDayStat{}}
	q := s.db.Model(&model.EmailOutbox{})
	if err := q.Where("status = ? AND sent_at >= ?", model.MailSent, startOfDayWIB(now)).Count(&out.SentToday).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.EmailOutbox{}).Where("status = ?", model.MailPending).Count(&out.Pending).Error; err != nil {
		return nil, err
	}
	err := s.db.Raw(`SELECT kind,
			COUNT(*) FILTER (WHERE status = 'sent') AS sent,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed,
			COUNT(*) FILTER (WHERE status = 'skipped') AS skipped,
			COUNT(*) FILTER (WHERE status = 'pending') AS pending
		FROM email_outbox WHERE created_at >= ? GROUP BY kind ORDER BY kind`, startOfDayWIB(now).AddDate(0, 0, -6)).Scan(&out.ByKind).Error
	if err != nil {
		return nil, err
	}
	err = s.db.Raw(`SELECT to_char((COALESCE(sent_at, created_at) AT TIME ZONE 'Asia/Jakarta')::date, 'YYYY-MM-DD') AS date,
			COUNT(*) FILTER (WHERE status = 'sent') AS sent,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed
		FROM email_outbox WHERE created_at >= ? GROUP BY 1 ORDER BY 1`, startOfDayWIB(now).AddDate(0, 0, -13)).Scan(&out.Daily).Error
	if err != nil {
		return nil, err
	}
	err = s.db.Raw(`SELECT
			COUNT(*) FILTER (WHERE email_verified_at IS NOT NULL) AS verified,
			COUNT(*) FILTER (WHERE email_verified_at IS NULL) AS unverified,
			COUNT(*) FILTER (WHERE email_verified_at IS NULL AND email_verify_due_at <= ?) AS overdue,
			COUNT(*) FILTER (WHERE pending_email <> '') AS pending_change,
			COUNT(*) FILTER (WHERE email_bounced_at IS NOT NULL) AS bounced
		FROM users WHERE role = ? AND status = ? AND deleted_at IS NULL`, now, model.RoleUser, model.UserActive).
		Row().Scan(&out.Verified, &out.Unverified, &out.Overdue, &out.PendingChange, &out.Bounced)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ─── Email transaksional ─────────────────────────────────────────────────────

func (s *emailService) view() emailView {
	return emailView{AppURL: s.cfg.AppURL, SettingURL: s.cfg.AppURL + "/profil"}
}

func (s *emailService) codeEmail(u *model.User, purpose, code string) Rendered {
	v := s.view()
	v.Code = code
	v.CodeNote = "Berlaku 15 menit. Jangan berikan kode ini ke siapa pun, termasuk yang mengaku dari MyFulus."
	v.Note = "Kalau kamu tidak meminta kode ini, abaikan saja email ini."
	if purpose == model.CodeChange {
		v.Title = "Konfirmasi email barumu"
		v.Preheader = "Masukkan kode ini di Profil untuk memakai email baru."
		v.Paragraphs = []string{"Halo " + u.Name + ",", "Masukkan kode di bawah di halaman Profil MyFulus untuk memakai alamat ini sebagai email login barumu."}
	} else {
		v.Title = "Verifikasi emailmu"
		v.Preheader = "Masukkan kode ini untuk memastikan email ini milikmu."
		v.Paragraphs = []string{"Halo " + u.Name + ",", "Masukkan kode di bawah untuk memastikan email ini benar milikmu. Setelah itu kamu bisa langsung masuk."}
	}
	return renderEmail(code+" adalah kode verifikasi MyFulus", v)
}

func (s *emailService) emailChangedEmail(u *model.User, at time.Time) Rendered {
	v := s.view()
	v.Title = "Email akunmu sudah diganti"
	v.Preheader = "Email login MyFulus-mu baru saja diganti."
	v.Paragraphs = []string{
		"Halo " + u.Name + ",",
		"Email login akun MyFulus-mu diganti ke " + maskEmail(u.Email) + " pada " + formatWIB(at) + ".",
		"Mulai sekarang semua email dari MyFulus dikirim ke alamat baru tersebut.",
	}
	v.Note = "Kalau kamu tidak melakukan perubahan ini, segera balas email ini supaya kami bisa membantu mengamankan akunmu."
	return renderEmail("Email akun MyFulus-mu sudah diganti", v)
}

func (s *emailService) SendApproved(userID uuid.UUID) {
	if !s.Enabled() {
		return
	}
	var u model.User
	var code string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		locked, err := s.lockUser(tx, userID)
		if err != nil {
			return err
		}
		u = *locked
		if u.EmailVerifiedAt != nil {
			return nil
		}
		code, err = s.issueCode(tx, userID, model.CodeVerify, u.Email)
		var cd *CooldownError
		if errors.As(err, &cd) {
			return nil
		}
		return err
	})
	if err != nil {
		log.Printf("[mail] gagal menyiapkan email persetujuan: %v", err)
		return
	}
	v := s.view()
	v.Title = "Akunmu sudah disetujui 🎉"
	v.Preheader = "Selamat datang di MyFulus. Masuk dan mulai catat keuanganmu."
	v.Paragraphs = []string{"Halo " + u.Name + ",", "Pendaftaranmu di MyFulus sudah disetujui admin. Selamat datang!"}
	v.ButtonText, v.ButtonURL = "Masuk ke MyFulus", s.cfg.AppURL+"/masuk"
	if code != "" {
		v.Paragraphs = append(v.Paragraphs, "Saat pertama kali masuk, masukkan kode di bawah untuk memastikan email ini milikmu.")
		v.Code = code
		v.CodeNote = "Berlaku 15 menit. Kalau sudah lewat, cukup masuk lalu minta kode baru."
	} else if u.EmailVerifiedAt == nil {
		v.Paragraphs = append(v.Paragraphs, "Saat pertama kali masuk, kamu akan diminta memasukkan kode verifikasi yang dikirim ke email ini.")
	}
	if err := s.outbox.Enqueue(&userID, u.Email, model.MailKindApproved, 0, renderEmail("Akun MyFulus-mu sudah disetujui", v)); err != nil {
		log.Printf("[mail] gagal antre email persetujuan: %v", err)
	}
}

// NotifyNewSignup memberi tahu admin lewat email bahwa ada pendaftar yang
// menunggu persetujuan.
func (s *emailService) NotifyNewSignup(user *model.User, admins []model.User) {
	if !s.Enabled() || len(admins) == 0 {
		return
	}
	var waiting int64
	s.db.Model(&model.User{}).Where("status = ?", model.UserPending).Count(&waiting)
	v := s.view()
	v.Title = "Pendaftar baru menunggu persetujuan"
	v.Preheader = user.Name + " baru mendaftar di MyFulus."
	v.Paragraphs = []string{"Ada pendaftar baru yang perlu kamu tinjau."}
	v.Rows = []emailRow{
		{Label: "Nama", Value: user.Name},
		{Label: "Email", Value: maskEmail(user.Email)},
		{Label: "Waktu daftar", Value: formatWIB(time.Now())},
		{Label: "Total menunggu", Value: strconv.FormatInt(waiting, 10) + " pendaftar"},
	}
	v.ButtonText, v.ButtonURL = "Buka Persetujuan", s.cfg.AppURL+"/admin/persetujuan"
	r := renderEmail("Pendaftar baru: "+user.Name, v)
	for i := range admins {
		a := &admins[i]
		if err := s.outbox.Enqueue(&a.ID, a.Email, model.MailKindNewSignup, 1, r); err != nil {
			log.Printf("[mail] gagal antre notifikasi pendaftar: %v", err)
		}
	}
}

func (s *emailService) LoginAlert(u *model.User, prevDevice string, client dto.ClientInfo) {
	device := truncate(utils.DescribeDevice(client.UserAgent), 120)
	if !s.Enabled() || prevDevice == "" || prevDevice == device || u.EmailVerifiedAt == nil || u.EmailBouncedAt != nil || u.EmailPrefs.LoginAlertOff {
		return
	}
	page, header := s.unsubLinks(u.ID, model.MailKindLoginAlert)
	v := s.view()
	v.UnsubURL = page
	v.Title = "Ada login dari perangkat baru"
	v.Preheader = "Akun MyFulus-mu baru saja dipakai masuk dari " + device + "."
	v.Paragraphs = []string{
		"Halo " + u.Name + ",",
		"Akun MyFulus-mu baru saja dipakai masuk dari perangkat yang berbeda dari sebelumnya.",
	}
	v.Rows = []emailRow{
		{Label: "Perangkat", Value: device},
		{Label: "Alamat IP", Value: client.IP},
		{Label: "Waktu", Value: formatWIB(time.Now())},
		{Label: "Perangkat sebelumnya", Value: prevDevice + " (sudah otomatis keluar)"},
	}
	v.Note = "Kalau ini kamu, abaikan email ini. Kalau bukan, segera ganti password supaya orang lain tidak bisa masuk lagi."
	v.ButtonText, v.ButtonURL = "Ganti password", s.cfg.AppURL+"/profil"
	r := renderEmail("Login baru ke akun MyFulus-mu", v)
	r.Unsubscribe = header
	if err := s.outbox.Enqueue(&u.ID, u.Email, model.MailKindLoginAlert, 1, r); err != nil {
		log.Printf("[mail] gagal antre peringatan login: %v", err)
	}
}

// ForgotPassword selalu terlihat berhasil bagi peminta agar email terdaftar
// tidak bisa ditebak; link hanya dikirim ke akun aktif.
func (s *emailService) ForgotPassword(email string) error {
	if !s.Enabled() {
		return ErrMailDisabled
	}
	email = normalizeEmail(email)
	var u model.User
	err := s.db.Where("LOWER(email) = ? AND status = ?", email, model.UserActive).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	now := time.Now()
	var recent int64
	if err := s.db.Model(&model.PasswordResetToken{}).Where("user_id = ? AND created_by IS NULL AND created_at > ?", u.ID, now.Add(-time.Hour)).Count(&recent).Error; err != nil {
		return err
	}
	if recent >= selfResetHourly {
		return nil
	}
	token, err := randomToken(32)
	if err != nil {
		return err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.PasswordResetToken{}).Where("user_id = ? AND used_at IS NULL AND expires_at > ?", u.ID, now).
			UpdateColumn("expires_at", now).Error; err != nil {
			return err
		}
		return tx.Create(&model.PasswordResetToken{UserID: u.ID, TokenHash: hashToken(token), ExpiresAt: now.Add(selfResetTTL)}).Error
	})
	if err != nil {
		return err
	}
	v := s.view()
	v.Title = "Atur ulang password"
	v.Preheader = "Link untuk membuat password baru, berlaku 30 menit."
	v.Paragraphs = []string{"Halo " + u.Name + ",", "Kami menerima permintaan untuk mengatur ulang password akun MyFulus-mu. Tekan tombol di bawah untuk membuat password baru."}
	v.ButtonText, v.ButtonURL = "Buat password baru", s.cfg.AppURL+"/atur-ulang-password?token="+token
	v.Note = "Link berlaku 30 menit dan hanya bisa dipakai sekali. Kalau kamu tidak meminta ini, abaikan email ini; password-mu tidak berubah."
	return s.outbox.Enqueue(&u.ID, u.Email, model.MailKindReset, 0, renderEmail("Atur ulang password MyFulus", v))
}

// ─── Email terjadwal ─────────────────────────────────────────────────────────

var (
	dayNamesID   = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	monthShortID = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
)

func formatWIB(t time.Time) string {
	t = t.In(jakarta)
	return fmt.Sprintf("%s, %d %s %d %02d.%02d WIB", dayNamesID[t.Weekday()], t.Day(), monthShortID[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

func shortDate(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), monthShortID[t.Month()-1])
}

func formatRupiah(v float64) string {
	n := strconv.FormatInt(int64(math.Round(math.Abs(v))), 10)
	var b strings.Builder
	for i, r := range n {
		if i > 0 && (len(n)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	if v < 0 && math.Round(math.Abs(v)) > 0 {
		return "−Rp " + b.String()
	}
	return "Rp " + b.String()
}

// weeklySlot menyebar ringkasan mingguan ke 7 hari agar kiriman harian merata.
func weeklySlot(id uuid.UUID) time.Weekday {
	h := fnv.New32a()
	h.Write(id[:])
	return time.Weekday(h.Sum32() % 7)
}

const eligibleUsers = `role = 'user' AND status = 'active' AND deleted_at IS NULL
	AND email_verified_at IS NOT NULL AND email_bounced_at IS NULL`

func (s *emailService) RunScheduled(now time.Time) {
	if !s.Enabled() {
		return
	}
	s.db.Exec("DELETE FROM email_codes WHERE created_at < ?", now.Add(-48*time.Hour))
	s.db.Exec("DELETE FROM email_outbox WHERE created_at < ? AND status <> ?", now.AddDate(0, 0, -30), model.MailPending)

	local := now.In(jakarta)
	if local.Hour() < scheduledHourWIB {
		return
	}
	today := dateOnly(local)
	if n, err := s.sendWeekly(today); err != nil {
		log.Printf("[mail] ringkasan mingguan: %v", err)
	} else if n > 0 {
		log.Printf("[mail] %d ringkasan mingguan diantrekan", n)
	}
	if n, err := s.sendReminders(today); err != nil {
		log.Printf("[mail] pengingat: %v", err)
	} else if n > 0 {
		log.Printf("[mail] %d pengingat diantrekan", n)
	}
}

// claim menandai email terjadwal hari ini sudah diproses, sehingga dua
// server atau dua putaran tidak mengirim ganda.
func (s *emailService) claim(column string, userID uuid.UUID, today time.Time) bool {
	res := s.db.Exec("UPDATE users SET "+column+" = ? WHERE id = ? AND ("+column+" IS NULL OR "+column+" < ?)", today, userID, today)
	return res.Error == nil && res.RowsAffected == 1
}

func (s *emailService) sendWeekly(today time.Time) (int, error) {
	var users []model.User
	err := s.db.Where(eligibleUsers+` AND NOT COALESCE((email_prefs->>'weekly_off')::boolean, false)
		AND (weekly_sent_on IS NULL OR weekly_sent_on < ?)`, today).Find(&users).Error
	if err != nil {
		return 0, err
	}
	sent := 0
	for i := range users {
		u := &users[i]
		if weeklySlot(u.ID) != today.Weekday() || !s.claim("weekly_sent_on", u.ID, today) {
			continue
		}
		r, ok, err := s.weeklyEmail(u, today)
		if err != nil {
			log.Printf("[mail] ringkasan %s: %v", u.ID, err)
			continue
		}
		if ok && s.outbox.Enqueue(&u.ID, u.Email, model.MailKindWeekly, 2, r) == nil {
			sent++
		}
	}
	return sent, nil
}

func (s *emailService) weeklyEmail(u *model.User, today time.Time) (Rendered, bool, error) {
	from, to := today.AddDate(0, 0, -7), today.AddDate(0, 0, -1)
	var totals struct {
		Income, Expense float64
		Count           int64
	}
	err := s.db.Raw(`SELECT COALESCE(SUM(amount) FILTER (WHERE type = 'income'), 0) AS income,
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS expense, COUNT(*) AS count
		FROM transactions WHERE user_id = ? AND deleted_at IS NULL AND transaction_date BETWEEN ? AND ?`, u.ID, from, to).Scan(&totals).Error
	if err != nil || totals.Count == 0 {
		return Rendered{}, false, err
	}
	var top []struct {
		Name  string
		Total float64
	}
	err = s.db.Raw(`SELECT c.name, SUM(t.amount) AS total FROM transactions t JOIN categories c ON c.id = t.category_id
		WHERE t.user_id = ? AND t.deleted_at IS NULL AND t.type = 'expense' AND t.transaction_date BETWEEN ? AND ?
		GROUP BY c.name ORDER BY total DESC LIMIT 3`, u.ID, from, to).Scan(&top).Error
	if err != nil {
		return Rendered{}, false, err
	}

	period := shortDate(from) + " – " + shortDate(to)
	v := s.view()
	v.Title = "Ringkasan mingguanmu"
	v.Preheader = "Lihat ke mana uangmu pergi pada " + period + "."
	v.Paragraphs = []string{"Halo " + u.Name + ", ini catatan keuanganmu untuk " + period + "."}
	net := totals.Income - totals.Expense
	netTone := "good"
	if net < 0 {
		netTone = "bad"
	}
	v.Rows = []emailRow{
		{Label: "Pemasukan", Value: formatRupiah(totals.Income), Tone: "good"},
		{Label: "Pengeluaran", Value: formatRupiah(totals.Expense), Tone: "bad"},
		{Label: "Selisih", Value: formatRupiah(net), Tone: netTone},
		{Label: "Jumlah transaksi", Value: strconv.FormatInt(totals.Count, 10)},
	}
	for _, t := range top {
		pct := 0.0
		if totals.Expense > 0 {
			pct = t.Total / totals.Expense * 100
		}
		v.List = append(v.List, fmt.Sprintf("%s: %s (%.0f%%)", t.Name, formatRupiah(t.Total), pct))
	}
	if len(v.List) > 0 {
		v.ListTitle = "Pengeluaran terbesar"
	}
	if summary, err := s.budgets.GetCurrentBudgets(u.ID); err == nil && summary != nil {
		var watch []string
		for _, b := range summary.Budgets {
			switch {
			case b.Kind == "savings":
			case b.Status == "over_budget":
				watch = append(watch, fmt.Sprintf("%s: lewat %s", b.Category.Name, formatRupiah(-b.Remain)))
			case b.Status == "warning" || b.Status == "used_up":
				watch = append(watch, fmt.Sprintf("%s: terpakai %.0f%%, sisa %s", b.Category.Name, b.UsedPct, formatRupiah(b.Remain)))
			}
		}
		if len(watch) > 0 {
			v.Paragraphs = append(v.Paragraphs, "Anggaran yang perlu diperhatikan: "+strings.Join(watch, "; ")+".")
		}
	}
	v.ButtonText, v.ButtonURL = "Lihat laporan lengkap", s.cfg.AppURL+"/laporan"
	page, header := s.unsubLinks(u.ID, model.MailKindWeekly)
	v.UnsubURL = page
	r := renderEmail("Ringkasan mingguan MyFulus, "+period, v)
	r.Unsubscribe = header
	return r, true, nil
}

type reminderItem struct {
	UserID uuid.UUID
	Line   string
	IsDebt bool
}

func (s *emailService) sendReminders(today time.Time) (int, error) {
	tomorrow := today.AddDate(0, 0, 1)
	var debts []struct {
		UserID       uuid.UUID
		Kind         string
		Counterparty string
		Remaining    float64
	}
	err := s.db.Raw(`SELECT user_id, kind, counterparty, principal - paid_amount AS remaining FROM debts
		WHERE deleted_at IS NULL AND due_date = ? AND principal - paid_amount > 0 ORDER BY created_at`, tomorrow).Scan(&debts).Error
	if err != nil {
		return 0, err
	}
	var recurring []struct {
		UserID   uuid.UUID
		Type     string
		Amount   float64
		Note     string
		Category string
	}
	err = s.db.Raw(`SELECT r.user_id, r.type, r.amount, r.note, c.name AS category FROM recurring_transactions r
		JOIN categories c ON c.id = r.category_id
		WHERE r.deleted_at IS NULL AND r.is_active AND r.next_run_date = ? AND (r.end_date IS NULL OR r.end_date >= ?)
		ORDER BY r.created_at`, tomorrow, tomorrow).Scan(&recurring).Error
	if err != nil {
		return 0, err
	}

	items := map[uuid.UUID][]reminderItem{}
	for _, d := range debts {
		line := fmt.Sprintf("Bayar hutang ke %s, sisa %s", d.Counterparty, formatRupiah(d.Remaining))
		if d.Kind == model.DebtReceivable {
			line = fmt.Sprintf("Tagih piutang dari %s, sisa %s", d.Counterparty, formatRupiah(d.Remaining))
		}
		items[d.UserID] = append(items[d.UserID], reminderItem{UserID: d.UserID, Line: line, IsDebt: true})
	}
	for _, r := range recurring {
		name := r.Category
		if note := strings.TrimSpace(r.Note); note != "" {
			name = note + " (" + r.Category + ")"
		}
		kind := "Pengeluaran"
		if r.Type == "income" {
			kind = "Pemasukan"
		}
		line := fmt.Sprintf("%s rutin %s sebesar %s dicatat otomatis", kind, name, formatRupiah(r.Amount))
		items[r.UserID] = append(items[r.UserID], reminderItem{UserID: r.UserID, Line: line})
	}
	if len(items) == 0 {
		return 0, nil
	}
	ids := make([]uuid.UUID, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	var users []model.User
	err = s.db.Where(eligibleUsers+` AND NOT COALESCE((email_prefs->>'reminders_off')::boolean, false)
		AND (reminder_sent_on IS NULL OR reminder_sent_on < ?) AND id IN ?`, today, ids).Find(&users).Error
	if err != nil {
		return 0, err
	}
	sent := 0
	for i := range users {
		u := &users[i]
		if !s.claim("reminder_sent_on", u.ID, today) {
			continue
		}
		if s.outbox.Enqueue(&u.ID, u.Email, model.MailKindReminder, 1, s.reminderEmail(u, tomorrow, items[u.ID])) == nil {
			sent++
		}
	}
	return sent, nil
}

func (s *emailService) reminderEmail(u *model.User, day time.Time, items []reminderItem) Rendered {
	v := s.view()
	when := dayNamesID[day.Weekday()] + ", " + shortDate(day)
	v.Title = "Pengingat untuk besok"
	v.Preheader = "Ada yang jatuh tempo besok, " + when + "."
	v.Paragraphs = []string{"Halo " + u.Name + ", ini yang perlu kamu siapkan untuk besok, " + when + "."}
	path := "/rutin"
	for _, it := range items {
		v.List = append(v.List, it.Line)
		if it.IsDebt {
			path = "/hutang"
		}
	}
	v.ButtonText, v.ButtonURL = "Buka MyFulus", s.cfg.AppURL+path
	page, header := s.unsubLinks(u.ID, model.MailKindReminder)
	v.UnsubURL = page
	r := renderEmail("Pengingat jatuh tempo besok", v)
	r.Unsubscribe = header
	return r
}
