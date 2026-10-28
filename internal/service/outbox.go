package service

import (
	"context"
	"log"
	"time"

	"dompetku/internal/model"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const (
	// Brevo gratis: 300 email per hari. Email biasa berhenti di 250 agar
	// kode verifikasi dan reset password tetap punya jatah.
	dailyMailSoftCap = 250
	dailyMailHardCap = 295
	maxMailAttempts  = 5
	outboxBatch      = 20
	outboxInterval   = 20 * time.Second
)

var mailBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

type Outbox interface {
	Enqueue(userID *uuid.UUID, to, kind string, priority int, r Rendered) error
	Flush(now time.Time) (int, error)
}

type outbox struct {
	db     *gorm.DB
	mailer Mailer
}

func NewOutbox(db *gorm.DB, mailer Mailer) Outbox {
	return &outbox{db: db, mailer: mailer}
}

func (o *outbox) Enqueue(userID *uuid.UUID, to, kind string, priority int, r Rendered) error {
	if !o.mailer.Enabled() || to == "" {
		return nil
	}
	return o.db.Create(&model.EmailOutbox{
		UserID: userID, ToEmail: to, Subject: truncate(r.Subject, 200), HTML: r.HTML, Text: r.Text,
		Kind: kind, Priority: priority, Unsubscribe: r.Unsubscribe, Status: model.MailPending, NextAttemptAt: time.Now(),
	}).Error
}

func startOfDayWIB(now time.Time) time.Time {
	t := now.In(jakarta)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jakarta)
}

// Flush mengirim email yang sudah waktunya, dengan coba ulang bertahap dan
// batas harian.
func (o *outbox) Flush(now time.Time) (int, error) {
	if !o.mailer.Enabled() {
		return 0, nil
	}
	var sentToday int64
	if err := o.db.Model(&model.EmailOutbox{}).Where("status = ? AND sent_at >= ?", model.MailSent, startOfDayWIB(now)).Count(&sentToday).Error; err != nil {
		return 0, err
	}
	if sentToday >= dailyMailHardCap {
		return 0, nil
	}
	q := o.db.Where("status = ? AND next_attempt_at <= ?", model.MailPending, now)
	if sentToday >= dailyMailSoftCap {
		q = q.Where("priority = 0")
	}
	var batch []model.EmailOutbox
	if err := q.Order("priority, created_at").Limit(outboxBatch).Find(&batch).Error; err != nil {
		return 0, err
	}

	sent := 0
	for _, m := range batch {
		if m.UserID != nil && m.Priority > 0 {
			var bounced int64
			o.db.Model(&model.User{}).Where("id = ? AND email_bounced_at IS NOT NULL AND email = ?", *m.UserID, m.ToEmail).Count(&bounced)
			if bounced > 0 {
				o.db.Model(&m).Updates(map[string]any{"status": model.MailSkipped, "last_error": "alamat email pernah terpental"})
				continue
			}
		}
		err := o.mailer.Send(Mail{To: m.ToEmail, Subject: m.Subject, HTML: m.HTML, Text: m.Text, Unsubscribe: m.Unsubscribe})
		attempts := m.Attempts + 1
		if err == nil {
			o.db.Model(&m).Updates(map[string]any{"status": model.MailSent, "attempts": attempts, "sent_at": now, "last_error": ""})
			sent++
			continue
		}
		log.Printf("[mail] gagal kirim %s ke %s: %v", m.Kind, maskEmail(m.ToEmail), err)
		updates := map[string]any{"attempts": attempts, "last_error": truncate(err.Error(), 300)}
		if attempts >= maxMailAttempts {
			updates["status"] = model.MailFailed
		} else {
			updates["next_attempt_at"] = now.Add(mailBackoff[min(attempts-1, len(mailBackoff)-1)])
		}
		o.db.Model(&m).Updates(updates)
	}
	return sent, nil
}

// StartMailWorker mengirim antrean email setiap 20 detik dan menjalankan
// email terjadwal (ringkasan mingguan, pengingat) setiap 15 menit.
func StartMailWorker(lc fx.Lifecycle, o Outbox, emails EmailService) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				send := time.NewTicker(outboxInterval)
				jobs := time.NewTicker(15 * time.Minute)
				defer send.Stop()
				defer jobs.Stop()
				emails.RunScheduled(time.Now())
				for {
					select {
					case <-ctx.Done():
						return
					case <-send.C:
						if _, err := o.Flush(time.Now()); err != nil {
							log.Printf("[mail] antrean: %v", err)
						}
					case <-jobs.C:
						emails.RunScheduled(time.Now())
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
