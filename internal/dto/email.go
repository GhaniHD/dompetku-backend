package dto

import "time"

type EmailPrefs struct {
	Weekly     bool `json:"weekly"`
	LoginAlert bool `json:"login_alert"`
	Reminders  bool `json:"reminders"`
}

// EmailStatus ditampilkan di Profil: status verifikasi, email yang menunggu
// verifikasi, dan pilihan notifikasi.
type EmailStatus struct {
	MailEnabled  bool       `json:"mail_enabled"`
	Verified     bool       `json:"verified"`
	VerifiedAt   *time.Time `json:"verified_at"`
	Required     bool       `json:"required"`
	DueAt        *time.Time `json:"due_at"`
	PendingEmail string     `json:"pending_email"`
	Bounced      bool       `json:"bounced"`
	Prefs        EmailPrefs `json:"prefs"`
}

type CodeSentResponse struct {
	Email     string `json:"email"`
	ResendIn  int    `json:"resend_in"`
	ExpiresIn int    `json:"expires_in"`
	Sent      bool   `json:"sent"`
}

type VerifyLoginRequest struct {
	VerifyToken string `json:"verify_token" binding:"required"`
	Code        string `json:"code"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}

type ChangeEmailRequest struct {
	Email    string `json:"email"    binding:"required"`
	Password string `json:"password" binding:"required"`
}

type VerifyCodeRequest struct {
	Code string `json:"code" binding:"required"`
}

type UnsubscribeRequest struct {
	Token string `json:"token"`
}

type UnsubscribeResponse struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type FeaturesResponse struct {
	Email         bool   `json:"email"`
	AdminWhatsApp string `json:"admin_whatsapp,omitempty"`
}

type BrevoEvent struct {
	Event string `json:"event"`
	Email string `json:"email"`
}

type EmailKindStat struct {
	Kind    string `json:"kind"`
	Sent    int64  `json:"sent"`
	Failed  int64  `json:"failed"`
	Skipped int64  `json:"skipped"`
	Pending int64  `json:"pending"`
}

type EmailDayStat struct {
	Date   string `json:"date"`
	Sent   int64  `json:"sent"`
	Failed int64  `json:"failed"`
}

// EmailStatsResponse hanya berisi angka; isi dan alamat email tidak pernah ditampilkan.
type EmailStatsResponse struct {
	Enabled       bool            `json:"enabled"`
	SentToday     int64           `json:"sent_today"`
	DailyCap      int             `json:"daily_cap"`
	ProviderLimit int             `json:"provider_limit"`
	Pending       int64           `json:"pending"`
	ByKind        []EmailKindStat `json:"by_kind"`
	Daily         []EmailDayStat  `json:"daily"`
	Verified      int64           `json:"verified"`
	Unverified    int64           `json:"unverified"`
	Overdue       int64           `json:"overdue"`
	PendingChange int64           `json:"pending_change"`
	Bounced       int64           `json:"bounced"`
}
