package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/pkg/crypto"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAdminUserNotFound  = errors.New("pengguna tidak ditemukan")
	ErrAdminInvalidStatus = errors.New("aksi ini tidak bisa dilakukan untuk status akun saat ini")
	ErrAdminProtected     = errors.New("akun admin tidak bisa diubah dari sini")
	ErrInviteNotFound     = errors.New("undangan tidak ditemukan")
	ErrAdminPassword      = errors.New("password admin salah")
	ErrAnnouncementGone   = errors.New("pengumuman tidak ditemukan atau sudah berakhir")
	ErrAnnouncementEnds   = errors.New("waktu berakhir pengumuman harus di masa depan")
)

const resetLinkTTL = 24 * time.Hour

type AdminService interface {
	Summary() (*dto.AdminSummaryResponse, error)
	ListUsers(filter dto.AdminUserFilter) ([]dto.AdminUserResponse, error)
	Approve(adminID, userID uuid.UUID) error
	Reject(adminID, userID uuid.UUID, reason string) error
	Suspend(adminID, userID uuid.UUID, reason string) error
	Activate(adminID, userID uuid.UUID) error
	Delete(adminID, userID uuid.UUID) error
	ResetLink(adminID, userID uuid.UUID) (*dto.ResetLinkResponse, error)
	ListInvites() ([]dto.InviteResponse, error)
	CreateInvite(adminID uuid.UUID, req dto.CreateInviteRequest) (*dto.InviteResponse, error)
	RevokeInvite(adminID, inviteID uuid.UUID) error
	Logs(limit int) ([]dto.AdminLogResponse, error)
	VerifyPassword(adminID uuid.UUID, password string) error
	UserDetail(userID uuid.UUID) (*dto.AdminUserDetail, error)
	AuthEvents(filter dto.AuthEventFilter) ([]dto.AuthEventResponse, error)
	ForceLogout(adminID, userID uuid.UUID, client dto.ClientInfo) error
	ListAnnouncements() ([]dto.AnnouncementResponse, error)
	CreateAnnouncement(adminID uuid.UUID, req dto.AnnouncementRequest) (*dto.AnnouncementResponse, error)
	EndAnnouncement(adminID, id uuid.UUID) error
	ActiveAnnouncements() ([]dto.AnnouncementResponse, error)
}

type adminService struct {
	db       *gorm.DB
	notif    NotificationService
	activity Activity
	emails   EmailService
	now      func() time.Time
}

func NewAdminService(db *gorm.DB, notif NotificationService, activity Activity, emails EmailService) AdminService {
	return &adminService{db: db, notif: notif, activity: activity, emails: emails, now: time.Now}
}

func (s *adminService) Summary() (*dto.AdminSummaryResponse, error) {
	var out dto.AdminSummaryResponse
	now := s.now()
	err := s.db.Raw(`SELECT
			COUNT(*) AS total_users,
			COUNT(*) FILTER (WHERE status = 'pending') AS pending,
			COUNT(*) FILTER (WHERE status = 'active') AS active,
			COUNT(*) FILTER (WHERE status = 'suspended') AS suspended,
			COUNT(*) FILTER (WHERE status = 'rejected') AS rejected,
			COUNT(*) FILTER (WHERE last_login_at >= ?) AS active30d,
			COUNT(*) FILTER (WHERE created_at >= ?) AS new7d
		FROM users WHERE deleted_at IS NULL AND role = 'user'`, now.AddDate(0, 0, -30), now.AddDate(0, 0, -7)).
		Row().Scan(&out.TotalUsers, &out.Pending, &out.Active, &out.Suspended, &out.Rejected, &out.Active30d, &out.New7d)
	if err != nil {
		return nil, err
	}
	err = s.db.Model(&model.Invite{}).
		Where("revoked_at IS NULL AND expires_at > ? AND used_count < max_uses", now).
		Count(&out.ActiveInvites).Error
	if err != nil {
		return nil, err
	}
	err = s.db.Raw(`SELECT
			(SELECT COUNT(*) FROM users WHERE role = 'user' AND deleted_at IS NULL AND last_seen_at >= ?),
			(SELECT COUNT(*) FROM auth_events WHERE event = 'login_failed' AND created_at >= ?)`,
		now.Add(-OnlineWindow), now.Add(-24*time.Hour)).Row().Scan(&out.OnlineNow, &out.LoginFailed24h)
	if err != nil {
		return nil, err
	}
	if err := s.db.Raw(`SELECT h.hour, COUNT(u.id) AS users
		FROM generate_series(date_trunc('hour', CAST(? AS timestamptz)) - interval '23 hours', date_trunc('hour', CAST(? AS timestamptz)), interval '1 hour') AS h(hour)
		LEFT JOIN presence_hours p ON p.hour = h.hour
		LEFT JOIN users u ON u.id = p.user_id AND u.role = 'user'
		GROUP BY h.hour ORDER BY h.hour`, now, now).Scan(&out.Online24h).Error; err != nil {
		return nil, err
	}
	if err := s.db.Raw(`SELECT w.week, COUNT(u.id) AS count
		FROM generate_series(date_trunc('week', CAST(? AS timestamptz)) - interval '7 weeks', date_trunc('week', CAST(? AS timestamptz)), interval '1 week') AS w(week)
		LEFT JOIN users u ON u.role = 'user' AND u.deleted_at IS NULL AND u.created_at >= w.week AND u.created_at < w.week + interval '1 week'
		GROUP BY w.week ORDER BY w.week`, now, now).Scan(&out.Registrations).Error; err != nil {
		return nil, err
	}
	err = s.db.Raw(`SELECT email, COUNT(*) AS attempts, COUNT(DISTINCT ip) AS ips, MAX(created_at) AS last_at
		FROM auth_events WHERE event IN ('login_failed', 'login_locked') AND created_at >= ?
		GROUP BY email HAVING COUNT(*) >= 5 ORDER BY attempts DESC LIMIT 10`, now.Add(-time.Hour)).Scan(&out.Suspicious).Error
	if out.Suspicious == nil {
		out.Suspicious = []dto.SuspiciousLogin{}
	}
	return &out, err
}

func (s *adminService) VerifyPassword(adminID uuid.UUID, password string) error {
	var admin model.User
	if err := s.db.Where("id = ? AND role = ?", adminID, model.RoleAdmin).First(&admin).Error; err != nil {
		return ErrAdminPassword
	}
	if !crypto.CheckPasswordHash(password, admin.Password) {
		return ErrAdminPassword
	}
	return nil
}

func (s *adminService) userQuery() *gorm.DB {
	return s.db.Table("users u").
		Select(`u.id, u.name, u.email, u.role, u.status, u.status_reason, u.created_at, u.approved_at, u.last_login_at,
			u.last_seen_at, u.last_seen_at >= ? AS online, u.last_login_device,
			u.email_verified_at IS NOT NULL AS email_verified, u.email_bounced_at IS NOT NULL AS email_bounced, u.pending_email <> '' AS email_pending,
			i.code AS invite_code,
			(SELECT COUNT(*) FROM wallets w WHERE w.user_id = u.id AND w.deleted_at IS NULL) AS wallets,
			(SELECT COUNT(*) FROM transactions t WHERE t.user_id = u.id AND t.deleted_at IS NULL) AS transactions`, s.now().Add(-OnlineWindow)).
		Joins("LEFT JOIN invites i ON i.id = u.invite_id").
		Where("u.deleted_at IS NULL")
}

func (s *adminService) UserDetail(userID uuid.UUID) (*dto.AdminUserDetail, error) {
	var users []dto.AdminUserResponse
	if err := s.userQuery().Where("u.id = ?", userID).Scan(&users).Error; err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, ErrAdminUserNotFound
	}
	events, err := s.AuthEvents(dto.AuthEventFilter{UserID: userID.String(), Limit: 50})
	if err != nil {
		return nil, err
	}
	return &dto.AdminUserDetail{User: users[0], Events: events}, nil
}

func (s *adminService) AuthEvents(filter dto.AuthEventFilter) ([]dto.AuthEventResponse, error) {
	limit := filter.Limit
	if limit == 0 {
		limit = 100
	}
	q := s.db.Table("auth_events e").
		Select("e.id, e.user_id, u.name AS user_name, e.email, e.event, e.detail, e.ip, e.device, e.created_at").
		Joins("LEFT JOIN users u ON u.id = e.user_id").
		Where("e.created_at >= ?", s.now().Add(-activityRetention))
	if filter.Event != "" {
		q = q.Where("e.event = ?", filter.Event)
	}
	if filter.UserID != "" {
		q = q.Where("e.user_id = ?", filter.UserID)
	}
	if term := strings.TrimSpace(filter.Query); term != "" {
		like := "%" + strings.ToLower(term) + "%"
		q = q.Where("LOWER(e.email) LIKE ? OR LOWER(COALESCE(u.name, '')) LIKE ? OR e.ip LIKE ?", like, like, like)
	}
	out := []dto.AuthEventResponse{}
	err := q.Order("e.created_at DESC").Limit(limit).Scan(&out).Error
	return out, err
}

func (s *adminService) ForceLogout(adminID, userID uuid.UUID, client dto.ClientInfo) error {
	user, err := s.transition(adminID, userID, "force_logout", []string{model.UserActive}, func(*model.User) map[string]any {
		return map[string]any{"token_version": gorm.Expr("token_version + 1"), "session_reason": model.SessionAdmin, "last_seen_at": nil}
	}, "")
	if err != nil {
		return err
	}
	s.activity.Forget(userID)
	s.activity.Record(&userID, user.Email, model.EventForcedLogout, "dikeluarkan oleh admin", client)
	return nil
}

func announcementResponse(a model.Announcement, now time.Time) dto.AnnouncementResponse {
	return dto.AnnouncementResponse{
		ID: a.ID, Message: a.Message, Level: a.Level, EndsAt: a.EndsAt, EndedAt: a.EndedAt, CreatedAt: a.CreatedAt,
		Active: a.EndedAt == nil && (a.EndsAt == nil || a.EndsAt.After(now)),
	}
}

func (s *adminService) ListAnnouncements() ([]dto.AnnouncementResponse, error) {
	var list []model.Announcement
	if err := s.db.Order("created_at DESC").Limit(50).Find(&list).Error; err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]dto.AnnouncementResponse, 0, len(list))
	for _, a := range list {
		out = append(out, announcementResponse(a, now))
	}
	return out, nil
}

func (s *adminService) ActiveAnnouncements() ([]dto.AnnouncementResponse, error) {
	var list []model.Announcement
	now := s.now()
	if err := s.db.Where("ended_at IS NULL AND (ends_at IS NULL OR ends_at > ?)", now).Order("created_at DESC").Limit(5).Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]dto.AnnouncementResponse, 0, len(list))
	for _, a := range list {
		out = append(out, announcementResponse(a, now))
	}
	return out, nil
}

func (s *adminService) CreateAnnouncement(adminID uuid.UUID, req dto.AnnouncementRequest) (*dto.AnnouncementResponse, error) {
	level := req.Level
	if level == "" {
		level = model.AnnouncementInfo
	}
	if req.EndsAt != nil && !req.EndsAt.After(s.now()) {
		return nil, ErrAnnouncementEnds
	}
	a := model.Announcement{Message: strings.TrimSpace(req.Message), Level: level, EndsAt: req.EndsAt, CreatedBy: adminID}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&a).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminLog{AdminID: adminID, Action: "announcement_create", Detail: truncate(a.Message, 120)}).Error
	})
	if err != nil {
		return nil, err
	}
	resp := announcementResponse(a, s.now())
	return &resp, nil
}

func (s *adminService) EndAnnouncement(adminID, id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Announcement{}).Where("id = ? AND ended_at IS NULL", id).Update("ended_at", s.now())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrAnnouncementGone
		}
		return tx.Create(&model.AdminLog{AdminID: adminID, Action: "announcement_end"}).Error
	})
}

func (s *adminService) ListUsers(filter dto.AdminUserFilter) ([]dto.AdminUserResponse, error) {
	q := s.userQuery()
	if filter.Online {
		q = q.Where("u.role = ? AND u.last_seen_at >= ?", model.RoleUser, s.now().Add(-OnlineWindow))
	}
	if filter.Status != "" {
		q = q.Where("u.status = ?", filter.Status)
	}
	if term := strings.TrimSpace(filter.Query); term != "" {
		like := "%" + strings.ToLower(term) + "%"
		q = q.Where("LOWER(u.name) LIKE ? OR LOWER(u.email) LIKE ?", like, like)
	}
	var out []dto.AdminUserResponse
	err := q.Order("CASE u.status WHEN 'pending' THEN 0 ELSE 1 END, u.created_at DESC").Scan(&out).Error
	return out, err
}

// transition mengubah status user di dalam DB transaction setelah memvalidasi
// status asal, lalu mencatat log admin.
func (s *adminService) transition(adminID, userID uuid.UUID, action string, from []string, apply func(u *model.User) map[string]any, detail string) (*model.User, error) {
	var user model.User
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAdminUserNotFound
		}
		if err != nil {
			return err
		}
		if user.Role == model.RoleAdmin || user.ID == adminID {
			return ErrAdminProtected
		}
		allowed := false
		for _, st := range from {
			allowed = allowed || user.Status == st
		}
		if !allowed {
			return ErrAdminInvalidStatus
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(apply(&user)).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminLog{AdminID: adminID, Action: action, TargetUserID: &userID, Detail: detail}).Error
	})
	return &user, err
}

func (s *adminService) Approve(adminID, userID uuid.UUID) error {
	user, err := s.transition(adminID, userID, "approve", []string{model.UserPending, model.UserRejected}, func(u *model.User) map[string]any {
		updates := map[string]any{"status": model.UserActive, "status_reason": "", "approved_at": s.now(), "approved_by": adminID}
		if s.emails.RequiresVerification(u) {
			updates["email_verify_due_at"] = s.now()
		}
		return updates
	}, "")
	if err == nil {
		_ = s.notif.CreateSystem(user.ID, "Akun disetujui", "Selamat datang di MyFulus! Mulai dengan menambahkan dompet pertamamu.")
		s.emails.SendApproved(user.ID)
	}
	return err
}

func (s *adminService) Reject(adminID, userID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	_, err := s.transition(adminID, userID, "reject", []string{model.UserPending}, func(*model.User) map[string]any {
		return map[string]any{"status": model.UserRejected, "status_reason": reason}
	}, reason)
	return err
}

func (s *adminService) Suspend(adminID, userID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	_, err := s.transition(adminID, userID, "suspend", []string{model.UserActive}, func(*model.User) map[string]any {
		return map[string]any{"status": model.UserSuspended, "status_reason": reason, "token_version": gorm.Expr("token_version + 1"), "session_reason": model.SessionAdmin}
	}, reason)
	return err
}

func (s *adminService) Activate(adminID, userID uuid.UUID) error {
	_, err := s.transition(adminID, userID, "activate", []string{model.UserSuspended}, func(*model.User) map[string]any {
		return map[string]any{"status": model.UserActive, "status_reason": ""}
	}, "")
	return err
}

func (s *adminService) Delete(adminID, userID uuid.UUID) error {
	var avatar string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		err := tx.Where("id = ?", userID).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAdminUserNotFound
		}
		if err != nil {
			return err
		}
		if user.Role == model.RoleAdmin || user.ID == adminID {
			return ErrAdminProtected
		}
		if err := tx.Create(&model.AdminLog{AdminID: adminID, Action: "delete", TargetUserID: &userID, Detail: maskEmail(user.Email)}).Error; err != nil {
			return err
		}
		avatar = user.AvatarURL
		return repository.PurgeUser(tx, userID)
	})
	if err == nil {
		removeAvatar(avatar)
	}
	return err
}

// maskEmail menyamarkan email akun yang sudah dihapus permanen, misalnya gh***@gmail.com.
func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return "***"
	}
	local := email[:at]
	if len(local) > 2 {
		local = local[:2]
	}
	return local + "***" + email[at:]
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *adminService) ResetLink(adminID, userID uuid.UUID) (*dto.ResetLinkResponse, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	expires := s.now().Add(resetLinkTTL)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		err := tx.Where("id = ?", userID).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAdminUserNotFound
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&model.PasswordResetToken{}).
			Where("user_id = ? AND used_at IS NULL", userID).
			UpdateColumn("expires_at", s.now()).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.PasswordResetToken{UserID: userID, TokenHash: hashToken(token), ExpiresAt: expires, CreatedBy: &adminID}).Error; err != nil {
			return err
		}
		return tx.Create(&model.AdminLog{AdminID: adminID, Action: "reset_link", TargetUserID: &userID}).Error
	})
	if err != nil {
		return nil, err
	}
	return &dto.ResetLinkResponse{Token: token, ExpiresAt: expires}, nil
}

const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func inviteCode() (string, error) {
	out := make([]byte, 8)
	for i := range out {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = inviteAlphabet[n.Int64()]
	}
	return string(out), nil
}

func (s *adminService) inviteResponse(i model.Invite, usedBy []string) dto.InviteResponse {
	status := "aktif"
	switch {
	case i.RevokedAt != nil:
		status = "dibatalkan"
	case i.UsedCount >= i.MaxUses:
		status = "habis"
	case !s.now().Before(i.ExpiresAt):
		status = "kedaluwarsa"
	}
	if usedBy == nil {
		usedBy = []string{}
	}
	return dto.InviteResponse{
		ID: i.ID, Code: i.Code, Note: i.Note, MaxUses: i.MaxUses, UsedCount: i.UsedCount,
		ExpiresAt: i.ExpiresAt, RevokedAt: i.RevokedAt, Status: status, CreatedAt: i.CreatedAt, UsedBy: usedBy,
	}
}

func (s *adminService) ListInvites() ([]dto.InviteResponse, error) {
	var invites []model.Invite
	if err := s.db.Order("created_at DESC").Limit(100).Find(&invites).Error; err != nil {
		return nil, err
	}
	type use struct {
		InviteID uuid.UUID
		Name     string
	}
	var uses []use
	if err := s.db.Raw(`SELECT invite_id, name FROM users WHERE invite_id IS NOT NULL AND deleted_at IS NULL ORDER BY created_at`).Scan(&uses).Error; err != nil {
		return nil, err
	}
	byInvite := map[uuid.UUID][]string{}
	for _, u := range uses {
		byInvite[u.InviteID] = append(byInvite[u.InviteID], u.Name)
	}
	out := make([]dto.InviteResponse, 0, len(invites))
	for _, i := range invites {
		out = append(out, s.inviteResponse(i, byInvite[i.ID]))
	}
	return out, nil
}

func (s *adminService) CreateInvite(adminID uuid.UUID, req dto.CreateInviteRequest) (*dto.InviteResponse, error) {
	if req.MaxUses == 0 {
		req.MaxUses = 1
	}
	if req.ValidDays == 0 {
		req.ValidDays = 7
	}
	var invite model.Invite
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for attempt := 0; attempt < 5; attempt++ {
			code, err := inviteCode()
			if err != nil {
				return err
			}
			invite = model.Invite{
				Code: code, Note: strings.TrimSpace(req.Note), MaxUses: req.MaxUses,
				ExpiresAt: s.now().AddDate(0, 0, req.ValidDays), CreatedBy: adminID,
			}
			var exists int64
			if err := tx.Model(&model.Invite{}).Where("code = ?", code).Count(&exists).Error; err != nil {
				return err
			}
			if exists == 0 {
				if err := tx.Create(&invite).Error; err != nil {
					return err
				}
				return tx.Create(&model.AdminLog{AdminID: adminID, Action: "invite_create", Detail: code}).Error
			}
		}
		return errors.New("gagal membuat kode undangan unik, coba lagi")
	})
	if err != nil {
		return nil, err
	}
	resp := s.inviteResponse(invite, nil)
	return &resp, nil
}

func (s *adminService) RevokeInvite(adminID, inviteID uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Invite{}).Where("id = ? AND revoked_at IS NULL", inviteID).UpdateColumn("revoked_at", s.now())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInviteNotFound
		}
		return tx.Create(&model.AdminLog{AdminID: adminID, Action: "invite_revoke", Detail: inviteID.String()}).Error
	})
}

func (s *adminService) Logs(limit int) ([]dto.AdminLogResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var out []dto.AdminLogResponse
	err := s.db.Raw(`SELECT l.id, a.name AS admin_name, l.action, t.name AS target_name, t.email AS target_email, l.detail, l.created_at
		FROM admin_logs l
		JOIN users a ON a.id = l.admin_id
		LEFT JOIN users t ON t.id = l.target_user_id
		ORDER BY l.created_at DESC LIMIT ?`, limit).Scan(&out).Error
	return out, err
}

// EnsureAdmins menjadikan email di ADMIN_EMAILS sebagai admin aktif saat server start.
func EnsureAdmins(db *gorm.DB, emails []string) error {
	if len(emails) == 0 {
		return nil
	}
	return db.Model(&model.User{}).
		Where("LOWER(email) IN ?", emails).
		Updates(map[string]any{"role": model.RoleAdmin, "status": model.UserActive}).Error
}
