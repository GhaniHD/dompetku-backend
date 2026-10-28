package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const (
	activityRetention = 7 * 24 * time.Hour
	presenceInterval  = time.Minute
	OnlineWindow      = 2 * time.Minute
	AdminIdleTimeout  = 30 * time.Minute
)

// Activity mencatat log aktivitas akun dan kapan pengguna terakhir aktif.
// Isinya hanya data keamanan akun, tidak pernah data keuangan.
type Activity interface {
	Record(userID *uuid.UUID, email, event, detail string, client dto.ClientInfo)
	Touch(ctx context.Context, userID uuid.UUID)
	Forget(userID uuid.UUID)
	Purge(now time.Time) error
}

type activity struct {
	db   *gorm.DB
	now  func() time.Time
	mu   sync.Mutex
	seen map[uuid.UUID]time.Time
}

func NewActivity(db *gorm.DB) Activity {
	return &activity{db: db, now: time.Now, seen: map[uuid.UUID]time.Time{}}
}

func (a *activity) Record(userID *uuid.UUID, email, event, detail string, client dto.ClientInfo) {
	e := model.AuthEvent{
		UserID: userID,
		Email:  truncate(strings.ToLower(strings.TrimSpace(email)), 100),
		Event:  event,
		Detail: truncate(detail, 200),
		IP:     truncate(client.IP, 64),
		Device: truncate(utils.DescribeDevice(client.UserAgent), 120),
	}
	if err := a.db.Create(&e).Error; err != nil {
		log.Printf("[activity] gagal mencatat %s: %v", event, err)
	}
}

// Touch menyimpan waktu aktif terakhir paling sering sekali per menit per
// pengguna, sehingga tidak menambah beban di setiap request.
func (a *activity) Touch(ctx context.Context, userID uuid.UUID) {
	now := a.now()
	a.mu.Lock()
	if last, ok := a.seen[userID]; ok && now.Sub(last) < presenceInterval {
		a.mu.Unlock()
		return
	}
	a.seen[userID] = now
	a.mu.Unlock()

	db := a.db.WithContext(ctx)
	if err := db.Exec("UPDATE users SET last_seen_at = ? WHERE id = ?", now, userID).Error; err != nil {
		log.Printf("[activity] gagal menyimpan last_seen: %v", err)
	}
	if err := db.Exec(`INSERT INTO presence_hours (hour, user_id) VALUES (date_trunc('hour', CAST(? AS timestamptz)), ?) ON CONFLICT DO NOTHING`, now, userID).Error; err != nil {
		log.Printf("[activity] gagal menyimpan presence: %v", err)
	}
}

func (a *activity) Forget(userID uuid.UUID) {
	a.mu.Lock()
	delete(a.seen, userID)
	a.mu.Unlock()
}

func (a *activity) Purge(now time.Time) error {
	cutoff := now.Add(-activityRetention)
	if err := a.db.Exec("DELETE FROM auth_events WHERE created_at < ?", cutoff).Error; err != nil {
		return err
	}
	return a.db.Exec("DELETE FROM presence_hours WHERE hour < ?", cutoff).Error
}

// StartActivityCleanup menghapus log aktivitas yang lebih tua dari 7 hari,
// saat server start lalu setiap jam.
func StartActivityCleanup(lc fx.Lifecycle, a Activity) {
	ctx, cancel := context.WithCancel(context.Background())
	run := func() {
		if err := a.Purge(time.Now()); err != nil {
			log.Printf("[activity] gagal membersihkan log lama: %v", err)
		}
	}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				run()
				ticker := time.NewTicker(time.Hour)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						run()
					}
				}
			}()
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}
