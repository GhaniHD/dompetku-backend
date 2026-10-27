package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"dompetku/internal/dto"
	"dompetku/internal/model"
	"dompetku/internal/repository"
	"dompetku/pkg/backupcrypt"
	"dompetku/pkg/crypto"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// Penanda format di dalam file terenkripsi; jangan diubah agar backup lama tetap terbaca.
	backupFormat  = "dompetku-backup"
	backupVersion = 1
	// RestoreConfirmWord wajib diketik pengguna sebelum data diganti.
	RestoreConfirmWord = "GANTI"
)

var (
	ErrBackupAccountPassword = errors.New("password akun salah")
	ErrBackupFormat          = errors.New("file bukan backup MyFulus atau versinya tidak didukung")
	ErrBackupInvalid         = errors.New("isi backup tidak lengkap atau rusak")
	ErrBackupConfirm         = errors.New("ketik GANTI untuk mengganti semua data akun ini")
	ErrBackupMismatch        = errors.New("saldo hasil pemulihan tidak cocok dengan backup, pemulihan dibatalkan")
	ErrBackupDecrypt         = backupcrypt.ErrDecrypt
	ErrBackupNotBackup       = backupcrypt.ErrNotBackup
)

// ─── Format file (versi 1) ────────────────────────────────────────────────────
// Struktur ini sengaja terpisah dari model agar file lama tetap bisa dibaca
// walau model database berubah. Tanggal disimpan sebagai "YYYY-MM-DD".

type backupPayload struct {
	Format      string    `json:"format"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	SourceName  string    `json:"source_name"`
	SourceEmail string    `json:"source_email"`
	Profile     struct {
		BudgetStartDay int    `json:"budget_start_day"`
		IncomeType     string `json:"income_type"`
		// Opsional: backup lama belum memiliki kolom ini.
		DailyCategoryIDs []uuid.UUID `json:"daily_category_ids,omitempty"`
	} `json:"profile"`
	Categories    []bCategory    `json:"categories"`
	Wallets       []bWallet      `json:"wallets"`
	Recurring     []bRecurring   `json:"recurring"`
	Transactions  []bTransaction `json:"transactions"`
	Transfers     []bTransfer    `json:"transfers"`
	Adjustments   []bAdjustment  `json:"adjustments"`
	Debts         []bDebt        `json:"debts"`
	DebtMovements []bDebtMove    `json:"debt_movements"`
	Goals         []bGoal        `json:"goals"`
	Budgets       []bBudget      `json:"budgets"`
}

type bMeta struct {
	ID        uuid.UUID  `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type bCategory struct {
	bMeta
	Name string `json:"name"`
	Type string `json:"type"`
}

type bWallet struct {
	bMeta
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Balance        float64 `json:"balance"`
	OpeningBalance float64 `json:"opening_balance"`
	IsArchived     bool    `json:"is_archived"`
}

type bRecurring struct {
	bMeta
	WalletID    uuid.UUID  `json:"wallet_id"`
	CategoryID  uuid.UUID  `json:"category_id"`
	Type        string     `json:"type"`
	Amount      float64    `json:"amount"`
	Note        string     `json:"note"`
	Frequency   string     `json:"frequency"`
	AnchorDay   int        `json:"anchor_day"`
	StartDate   string     `json:"start_date"`
	EndDate     string     `json:"end_date,omitempty"`
	NextRunDate string     `json:"next_run_date"`
	IsActive    bool       `json:"is_active"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
}

type bTransaction struct {
	bMeta
	WalletID    uuid.UUID  `json:"wallet_id"`
	CategoryID  uuid.UUID  `json:"category_id"`
	Amount      float64    `json:"amount"`
	Type        string     `json:"type"`
	Note        string     `json:"note"`
	Date        string     `json:"date"`
	RecurringID *uuid.UUID `json:"recurring_id,omitempty"`
}

type bTransfer struct {
	bMeta
	FromWalletID     uuid.UUID  `json:"from_wallet_id"`
	ToWalletID       uuid.UUID  `json:"to_wallet_id"`
	Amount           float64    `json:"amount"`
	Fee              float64    `json:"fee"`
	FeeTransactionID *uuid.UUID `json:"fee_transaction_id,omitempty"`
	Note             string     `json:"note"`
	Date             string     `json:"date"`
}

type bAdjustment struct {
	bMeta
	WalletID        uuid.UUID `json:"wallet_id"`
	Amount          float64   `json:"amount"`
	PreviousBalance float64   `json:"previous_balance"`
	NewBalance      float64   `json:"new_balance"`
	Note            string    `json:"note"`
	Date            string    `json:"date"`
}

type bDebt struct {
	bMeta
	Kind           string  `json:"kind"`
	Counterparty   string  `json:"counterparty"`
	Principal      float64 `json:"principal"`
	PaidAmount     float64 `json:"paid_amount"`
	Note           string  `json:"note"`
	DueDate        string  `json:"due_date,omitempty"`
	LastRemindedOn string  `json:"last_reminded_on,omitempty"`
}

type bDebtMove struct {
	bMeta
	DebtID      uuid.UUID  `json:"debt_id"`
	WalletID    *uuid.UUID `json:"wallet_id,omitempty"`
	Kind        string     `json:"kind"`
	Amount      float64    `json:"amount"`
	WalletDelta float64    `json:"wallet_delta"`
	Date        string     `json:"date"`
	Note        string     `json:"note"`
}

type bGoal struct {
	bMeta
	WalletID     uuid.UUID  `json:"wallet_id"`
	Name         string     `json:"name"`
	TargetAmount float64    `json:"target_amount"`
	Deadline     string     `json:"deadline,omitempty"`
	AchievedAt   *time.Time `json:"achieved_at,omitempty"`
}

type bBudget struct {
	bMeta
	Kind       string     `json:"kind"`
	CategoryID *uuid.UUID `json:"category_id,omitempty"`
	WalletID   *uuid.UUID `json:"wallet_id,omitempty"`
	GoalID     *uuid.UUID `json:"goal_id,omitempty"`
	Amount     float64    `json:"amount"`
	Month      int        `json:"month"`
	Year       int        `json:"year"`
	Notes      string     `json:"notes"`
}

func meta(id uuid.UUID, created time.Time, deleted gorm.DeletedAt) bMeta {
	m := bMeta{ID: id, CreatedAt: created}
	if deleted.Valid {
		t := deleted.Time
		m.DeletedAt = &t
	}
	return m
}

func ymd(t time.Time) string { return t.Format("2006-01-02") }

func ymdPtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return ymd(*t)
}

// ─── Service ──────────────────────────────────────────────────────────────────

type BackupService interface {
	Export(ctx context.Context, userID uuid.UUID, req dto.BackupExportRequest) ([]byte, string, error)
	Preview(ctx context.Context, userID uuid.UUID, accountPassword, backupPassword string, file []byte) (*dto.BackupPreview, error)
	Restore(ctx context.Context, userID uuid.UUID, accountPassword, backupPassword, confirm string, file []byte) (*dto.BackupPreview, error)
}

type backupService struct {
	db       *gorm.DB
	userRepo repository.UserRepository
	notif    NotificationService
	now      func() time.Time
}

func NewBackupService(db *gorm.DB, userRepo repository.UserRepository, notif NotificationService) BackupService {
	return &backupService{db: db, userRepo: userRepo, notif: notif, now: time.Now}
}

func (s *backupService) checkAccount(ctx context.Context, userID uuid.UUID, password string) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	if !crypto.CheckPasswordHash(password, user.Password) {
		return nil, ErrBackupAccountPassword
	}
	return user, nil
}

func (s *backupService) Export(ctx context.Context, userID uuid.UUID, req dto.BackupExportRequest) ([]byte, string, error) {
	user, err := s.checkAccount(ctx, userID, req.Password)
	if err != nil {
		return nil, "", err
	}
	p, err := s.collect(ctx, user)
	if err != nil {
		return nil, "", err
	}
	plain, err := json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	sealed, err := backupcrypt.Seal(plain, req.BackupPassword)
	if err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("myfulus-backup-%s.dpk", p.CreatedAt.In(jakarta).Format("2006-01-02"))
	return sealed, name, nil
}

// collect membaca semua data milik pengguna, termasuk baris yang sudah dihapus,
// agar hasil pemulihan identik dengan aslinya.
func (s *backupService) collect(ctx context.Context, user *model.User) (*backupPayload, error) {
	p := &backupPayload{Format: backupFormat, Version: backupVersion, CreatedAt: s.now().UTC(), SourceName: user.Name, SourceEmail: user.Email}
	p.Profile.BudgetStartDay = user.BudgetStartDay
	p.Profile.IncomeType = incomeTypeOf(user)
	p.Profile.DailyCategoryIDs = user.DailyCategoryIDs

	var categories []model.Category
	var wallets []model.Wallet
	var recurring []model.RecurringTransaction
	var txs []model.Transaction
	var transfers []model.Transfer
	var adjustments []model.BalanceAdjustment
	var debts []model.Debt
	var moves []model.DebtMovement
	var goals []model.Goal
	var budgets []model.Budget
	for _, q := range []struct {
		dest  any
		order string
	}{
		{&categories, "created_at, id"}, {&wallets, "created_at, id"}, {&recurring, "created_at, id"},
		{&txs, "transaction_date, created_at, id"}, {&transfers, "transfer_date, created_at, id"},
		{&adjustments, "adjusted_at, created_at, id"}, {&debts, "created_at, id"}, {&moves, "movement_date, created_at, id"},
		{&goals, "created_at, id"}, {&budgets, "year, month, created_at, id"},
	} {
		if err := s.db.WithContext(ctx).Unscoped().Where("user_id = ?", user.ID).Order(q.order).Find(q.dest).Error; err != nil {
			return nil, err
		}
	}

	for _, c := range categories {
		p.Categories = append(p.Categories, bCategory{meta(c.ID, c.CreatedAt, c.DeletedAt), c.Name, c.Type})
	}
	for _, w := range wallets {
		p.Wallets = append(p.Wallets, bWallet{meta(w.ID, w.CreatedAt, w.DeletedAt), w.Name, w.Type, w.Balance, w.OpeningBalance, w.IsArchived})
	}
	for _, r := range recurring {
		p.Recurring = append(p.Recurring, bRecurring{meta(r.ID, r.CreatedAt, r.DeletedAt), r.WalletID, r.CategoryID, r.Type, r.Amount, r.Note,
			r.Frequency, r.AnchorDay, ymd(r.StartDate), ymdPtr(r.EndDate), ymd(r.NextRunDate), r.IsActive, r.LastRunAt})
	}
	for _, t := range txs {
		p.Transactions = append(p.Transactions, bTransaction{meta(t.ID, t.CreatedAt, t.DeletedAt), t.WalletID, t.CategoryID, t.Amount, t.Type, t.Note, ymd(t.TransactionDate), t.RecurringID})
	}
	for _, t := range transfers {
		p.Transfers = append(p.Transfers, bTransfer{meta(t.ID, t.CreatedAt, t.DeletedAt), t.FromWalletID, t.ToWalletID, t.Amount, t.Fee, t.FeeTransactionID, t.Note, ymd(t.TransferDate)})
	}
	for _, a := range adjustments {
		p.Adjustments = append(p.Adjustments, bAdjustment{meta(a.ID, a.CreatedAt, a.DeletedAt), a.WalletID, a.Amount, a.PreviousBalance, a.NewBalance, a.Note, ymd(a.AdjustedAt)})
	}
	for _, d := range debts {
		p.Debts = append(p.Debts, bDebt{meta(d.ID, d.CreatedAt, d.DeletedAt), d.Kind, d.Counterparty, d.Principal, d.PaidAmount, d.Note, ymdPtr(d.DueDate), ymdPtr(d.LastRemindedOn)})
	}
	for _, m := range moves {
		p.DebtMovements = append(p.DebtMovements, bDebtMove{meta(m.ID, m.CreatedAt, m.DeletedAt), m.DebtID, m.WalletID, m.Kind, m.Amount, m.WalletDelta, ymd(m.MovementDate), m.Note})
	}
	for _, g := range goals {
		p.Goals = append(p.Goals, bGoal{meta(g.ID, g.CreatedAt, g.DeletedAt), g.WalletID, g.Name, g.TargetAmount, ymdPtr(g.Deadline), g.AchievedAt})
	}
	for _, b := range budgets {
		notes := ""
		if b.Notes != nil {
			notes = *b.Notes
		}
		p.Budgets = append(p.Budgets, bBudget{meta(b.ID, b.CreatedAt, b.DeletedAt), b.Kind, b.CategoryID, b.WalletID, b.GoalID, b.Amount, b.Month, b.Year, notes})
	}
	return p, nil
}

func (s *backupService) open(file []byte, backupPassword string) (*backupPayload, error) {
	plain, err := backupcrypt.Open(file, backupPassword)
	if err != nil {
		return nil, err
	}
	var p backupPayload
	if err := json.Unmarshal(plain, &p); err != nil || p.Format != backupFormat {
		return nil, ErrBackupFormat
	}
	if p.Version < 1 || p.Version > backupVersion {
		return nil, ErrBackupFormat
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// validate memastikan setiap referensi menunjuk ke data yang ada di file yang sama.
func (p *backupPayload) validate() error {
	has := func(ids map[uuid.UUID]bool, id *uuid.UUID) bool { return id == nil || ids[*id] }
	set := func(n int, id func(int) uuid.UUID) map[uuid.UUID]bool {
		m := make(map[uuid.UUID]bool, n)
		for i := 0; i < n; i++ {
			m[id(i)] = true
		}
		return m
	}
	cats := set(len(p.Categories), func(i int) uuid.UUID { return p.Categories[i].ID })
	wallets := set(len(p.Wallets), func(i int) uuid.UUID { return p.Wallets[i].ID })
	recurring := set(len(p.Recurring), func(i int) uuid.UUID { return p.Recurring[i].ID })
	txs := set(len(p.Transactions), func(i int) uuid.UUID { return p.Transactions[i].ID })
	debts := set(len(p.Debts), func(i int) uuid.UUID { return p.Debts[i].ID })
	goals := set(len(p.Goals), func(i int) uuid.UUID { return p.Goals[i].ID })

	ok := true
	for _, r := range p.Recurring {
		ok = ok && wallets[r.WalletID] && cats[r.CategoryID]
	}
	for _, t := range p.Transactions {
		ok = ok && wallets[t.WalletID] && cats[t.CategoryID] && has(recurring, t.RecurringID)
	}
	for _, t := range p.Transfers {
		ok = ok && wallets[t.FromWalletID] && wallets[t.ToWalletID] && has(txs, t.FeeTransactionID)
	}
	for _, a := range p.Adjustments {
		ok = ok && wallets[a.WalletID]
	}
	for _, m := range p.DebtMovements {
		ok = ok && debts[m.DebtID] && has(wallets, m.WalletID)
	}
	for _, g := range p.Goals {
		ok = ok && wallets[g.WalletID]
	}
	for _, b := range p.Budgets {
		ok = ok && has(cats, b.CategoryID) && has(wallets, b.WalletID) && has(goals, b.GoalID)
	}
	if !ok {
		return ErrBackupInvalid
	}
	return nil
}

func (p *backupPayload) preview() *dto.BackupPreview {
	out := &dto.BackupPreview{
		CreatedAt: p.CreatedAt, SourceName: p.SourceName, SourceEmail: p.SourceEmail,
	}
	for _, c := range p.Categories {
		if c.DeletedAt == nil {
			out.Categories++
		}
	}
	for _, w := range p.Wallets {
		if w.DeletedAt == nil {
			out.Wallets++
			out.TotalBalance += w.Balance
		}
	}
	for _, t := range p.Transactions {
		if t.DeletedAt != nil {
			continue
		}
		out.Transactions++
		if out.FirstDate == "" || t.Date < out.FirstDate {
			out.FirstDate = t.Date
		}
		if t.Date > out.LastDate {
			out.LastDate = t.Date
		}
	}
	for _, t := range p.Transfers {
		if t.DeletedAt == nil {
			out.Transfers++
		}
	}
	for _, b := range p.Budgets {
		if b.DeletedAt == nil {
			out.Budgets++
		}
	}
	for _, r := range p.Recurring {
		if r.DeletedAt == nil {
			out.Recurring++
		}
	}
	for _, d := range p.Debts {
		if d.DeletedAt == nil {
			out.Debts++
		}
	}
	for _, g := range p.Goals {
		if g.DeletedAt == nil {
			out.Goals++
		}
	}
	out.TotalBalance = math.Round(out.TotalBalance*100) / 100
	return out
}

func (s *backupService) Preview(ctx context.Context, userID uuid.UUID, accountPassword, backupPassword string, file []byte) (*dto.BackupPreview, error) {
	if _, err := s.checkAccount(ctx, userID, accountPassword); err != nil {
		return nil, err
	}
	p, err := s.open(file, backupPassword)
	if err != nil {
		return nil, err
	}
	return p.preview(), nil
}

func (s *backupService) Restore(ctx context.Context, userID uuid.UUID, accountPassword, backupPassword, confirm string, file []byte) (*dto.BackupPreview, error) {
	if strings.TrimSpace(confirm) != RestoreConfirmWord {
		return nil, ErrBackupConfirm
	}
	if _, err := s.checkAccount(ctx, userID, accountPassword); err != nil {
		return nil, err
	}
	p, err := s.open(file, backupPassword)
	if err != nil {
		return nil, err
	}

	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&locked).Error; err != nil {
			return err
		}
		if err := wipeUserData(tx, userID); err != nil {
			return err
		}
		ids, err := insertPayload(tx, userID, p)
		if err != nil {
			return err
		}
		if err := verifyBalances(tx, userID, p); err != nil {
			return err
		}
		day := p.Profile.BudgetStartDay
		if day < 1 || day > 31 {
			day = 1
		}
		incomeType := p.Profile.IncomeType
		if incomeType != model.IncomeIrregular {
			incomeType = model.IncomeSalary
		}
		return tx.Model(&model.User{ID: userID}).Select("budget_start_day", "income_type", "income_setup_at", "daily_category_ids").Updates(&model.User{
			BudgetStartDay: day, IncomeType: incomeType, IncomeSetupAt: &now, DailyCategoryIDs: restoredDailyCategories(p, ids),
		}).Error
	})
	if err != nil {
		return nil, err
	}

	summary := p.preview()
	_ = s.notif.CreateSystem(userID, "Data dipulihkan dari backup",
		fmt.Sprintf("%d dompet dan %d transaksi dari backup %s berhasil dipulihkan.",
			summary.Wallets, summary.Transactions, p.CreatedAt.In(jakarta).Format("02-01-2006 15:04")))
	return summary, nil
}

// wipeUserData menghapus permanen data keuangan pengguna dengan urutan yang
// aman terhadap foreign key. Profil, notifikasi, dan akun tidak disentuh.
func wipeUserData(tx *gorm.DB, userID uuid.UUID) error {
	for _, table := range []string{
		"budgets", "goals", "debt_movements", "debts", "transfers",
		"balance_adjustments", "transactions", "recurring_transactions", "wallets", "categories",
	} {
		if err := tx.Exec("DELETE FROM "+table+" WHERE user_id = ?", userID).Error; err != nil {
			return fmt.Errorf("hapus %s: %w", table, err)
		}
	}
	return nil
}

type idMap map[uuid.UUID]uuid.UUID

func (m idMap) get(old uuid.UUID) uuid.UUID {
	if id, ok := m[old]; ok {
		return id
	}
	id := uuid.New()
	m[old] = id
	return id
}

func (m idMap) ptr(old *uuid.UUID) *uuid.UUID {
	if old == nil {
		return nil
	}
	id := m.get(*old)
	return &id
}

func parseDay(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func parseDayPtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t := parseDay(s)
	return &t
}

func deletedAt(t *time.Time) gorm.DeletedAt {
	if t == nil {
		return gorm.DeletedAt{}
	}
	return gorm.DeletedAt{Time: *t, Valid: true}
}

// insertPayload memasukkan isi backup dengan ID baru, karena akun asal bisa
// saja masih ada dan memakai ID yang sama.
func insertPayload(tx *gorm.DB, userID uuid.UUID, p *backupPayload) (idMap, error) {
	ids := idMap{}
	insert := func(rows any) error {
		return tx.Omit(clause.Associations).CreateInBatches(rows, 500).Error
	}

	categories := make([]model.Category, 0, len(p.Categories))
	for _, c := range p.Categories {
		categories = append(categories, model.Category{ID: ids.get(c.ID), UserID: userID, Name: c.Name, Type: c.Type, CreatedAt: c.CreatedAt, DeletedAt: deletedAt(c.DeletedAt)})
	}
	wallets := make([]model.Wallet, 0, len(p.Wallets))
	for _, w := range p.Wallets {
		wallets = append(wallets, model.Wallet{ID: ids.get(w.ID), UserID: userID, Name: w.Name, Type: w.Type, Balance: w.Balance,
			OpeningBalance: w.OpeningBalance, IsArchived: w.IsArchived, CreatedAt: w.CreatedAt, DeletedAt: deletedAt(w.DeletedAt)})
	}
	recurring := make([]model.RecurringTransaction, 0, len(p.Recurring))
	for _, r := range p.Recurring {
		recurring = append(recurring, model.RecurringTransaction{ID: ids.get(r.ID), UserID: userID, WalletID: ids.get(r.WalletID), CategoryID: ids.get(r.CategoryID),
			Type: r.Type, Amount: r.Amount, Note: r.Note, Frequency: r.Frequency, AnchorDay: r.AnchorDay, StartDate: parseDay(r.StartDate),
			EndDate: parseDayPtr(r.EndDate), NextRunDate: parseDay(r.NextRunDate), IsActive: r.IsActive, LastRunAt: r.LastRunAt,
			CreatedAt: r.CreatedAt, DeletedAt: deletedAt(r.DeletedAt)})
	}
	txs := make([]model.Transaction, 0, len(p.Transactions))
	for _, t := range p.Transactions {
		txs = append(txs, model.Transaction{ID: ids.get(t.ID), UserID: userID, WalletID: ids.get(t.WalletID), CategoryID: ids.get(t.CategoryID),
			Amount: t.Amount, Type: t.Type, Note: t.Note, TransactionDate: parseDay(t.Date), RecurringID: ids.ptr(t.RecurringID),
			CreatedAt: t.CreatedAt, DeletedAt: deletedAt(t.DeletedAt)})
	}
	transfers := make([]model.Transfer, 0, len(p.Transfers))
	for _, t := range p.Transfers {
		transfers = append(transfers, model.Transfer{ID: ids.get(t.ID), UserID: userID, FromWalletID: ids.get(t.FromWalletID), ToWalletID: ids.get(t.ToWalletID),
			Amount: t.Amount, Fee: t.Fee, FeeTransactionID: ids.ptr(t.FeeTransactionID), Note: t.Note, TransferDate: parseDay(t.Date),
			CreatedAt: t.CreatedAt, DeletedAt: deletedAt(t.DeletedAt)})
	}
	adjustments := make([]model.BalanceAdjustment, 0, len(p.Adjustments))
	for _, a := range p.Adjustments {
		adjustments = append(adjustments, model.BalanceAdjustment{ID: ids.get(a.ID), UserID: userID, WalletID: ids.get(a.WalletID), Amount: a.Amount,
			PreviousBalance: a.PreviousBalance, NewBalance: a.NewBalance, Note: a.Note, AdjustedAt: parseDay(a.Date),
			CreatedAt: a.CreatedAt, DeletedAt: deletedAt(a.DeletedAt)})
	}
	debts := make([]model.Debt, 0, len(p.Debts))
	for _, d := range p.Debts {
		debts = append(debts, model.Debt{ID: ids.get(d.ID), UserID: userID, Kind: d.Kind, Counterparty: d.Counterparty, Principal: d.Principal,
			PaidAmount: d.PaidAmount, Note: d.Note, DueDate: parseDayPtr(d.DueDate), LastRemindedOn: parseDayPtr(d.LastRemindedOn),
			CreatedAt: d.CreatedAt, DeletedAt: deletedAt(d.DeletedAt)})
	}
	moves := make([]model.DebtMovement, 0, len(p.DebtMovements))
	for _, m := range p.DebtMovements {
		moves = append(moves, model.DebtMovement{ID: ids.get(m.ID), DebtID: ids.get(m.DebtID), UserID: userID, WalletID: ids.ptr(m.WalletID), Kind: m.Kind,
			Amount: m.Amount, WalletDelta: m.WalletDelta, MovementDate: parseDay(m.Date), Note: m.Note, CreatedAt: m.CreatedAt, DeletedAt: deletedAt(m.DeletedAt)})
	}
	goals := make([]model.Goal, 0, len(p.Goals))
	for _, g := range p.Goals {
		goals = append(goals, model.Goal{ID: ids.get(g.ID), UserID: userID, WalletID: ids.get(g.WalletID), Name: g.Name, TargetAmount: g.TargetAmount,
			Deadline: parseDayPtr(g.Deadline), AchievedAt: g.AchievedAt, CreatedAt: g.CreatedAt, DeletedAt: deletedAt(g.DeletedAt)})
	}
	budgets := make([]model.Budget, 0, len(p.Budgets))
	for _, b := range p.Budgets {
		notes := b.Notes
		kind := b.Kind
		if kind == "" {
			kind = model.BudgetExpense
		}
		budgets = append(budgets, model.Budget{ID: ids.get(b.ID), UserID: userID, Kind: kind, CategoryID: ids.ptr(b.CategoryID), WalletID: ids.ptr(b.WalletID),
			GoalID: ids.ptr(b.GoalID), Amount: b.Amount, Month: b.Month, Year: b.Year, Notes: &notes, CreatedAt: b.CreatedAt, UpdatedAt: b.CreatedAt,
			DeletedAt: deletedAt(b.DeletedAt)})
	}

	for _, step := range []struct {
		name string
		rows any
		n    int
	}{
		{"kategori", &categories, len(categories)}, {"dompet", &wallets, len(wallets)}, {"transaksi rutin", &recurring, len(recurring)},
		{"transaksi", &txs, len(txs)}, {"pindahan", &transfers, len(transfers)}, {"penyesuaian saldo", &adjustments, len(adjustments)},
		{"hutang", &debts, len(debts)}, {"pergerakan hutang", &moves, len(moves)}, {"target", &goals, len(goals)}, {"anggaran", &budgets, len(budgets)},
	} {
		if step.n == 0 {
			continue
		}
		if err := insert(step.rows); err != nil {
			return nil, fmt.Errorf("pulihkan %s: %w", step.name, err)
		}
	}
	return ids, nil
}

// restoredDailyCategories memetakan pilihan jatah harian ke ID kategori baru;
// kategori yang tidak ada di backup atau sudah dihapus diabaikan.
func restoredDailyCategories(p *backupPayload, ids idMap) []uuid.UUID {
	usable := map[uuid.UUID]bool{}
	for _, c := range p.Categories {
		if c.Type == "expense" && c.DeletedAt == nil {
			usable[c.ID] = true
		}
	}
	out := []uuid.UUID{}
	for _, id := range p.Profile.DailyCategoryIDs {
		if usable[id] {
			out = append(out, ids[id])
		}
	}
	return out
}

// verifyBalances menghitung ulang saldo setiap dompet dari mutasinya dan
// memastikan hasilnya sama persis dengan saldo di backup.
func verifyBalances(tx *gorm.DB, userID uuid.UUID, p *backupPayload) error {
	balances, err := repository.NewExportRepository(tx).BalancesBefore(userID, "9999-12-31")
	if err != nil {
		return err
	}
	// Dompet yang sudah dihapus tidak diperiksa: mutasinya ikut terhapus sehingga
	// saldo tersimpannya memang tidak lagi sama dengan jumlah mutasi.
	var wallets []model.Wallet
	if err := tx.Where("user_id = ?", userID).Find(&wallets).Error; err != nil {
		return err
	}
	active := 0
	for _, w := range p.Wallets {
		if w.DeletedAt == nil {
			active++
		}
	}
	if len(wallets) != active {
		return ErrBackupMismatch
	}
	for _, w := range wallets {
		if math.Abs(balances[w.ID]-w.Balance) > 0.005 {
			return ErrBackupMismatch
		}
	}
	return nil
}
