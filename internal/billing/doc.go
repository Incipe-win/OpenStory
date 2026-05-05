// Package billing manages user credits, usage metering, and billing records.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
	"github.com/Incipe-win/OpenStory/internal/observability"
)

const (
	TypeBonus   = "bonus"
	TypeReserve = "reserve"
	TypeConfirm = "confirm"
	TypeRefund  = "refund"
	TypeSpend   = "spend"
)

var (
	ErrAccountNotFound      = errors.New("credit account not found")
	ErrInsufficientCredits  = errors.New("insufficient credits")
	ErrInvalidCreditAmount  = errors.New("credit amount must be positive")
	ErrReservationNotFound  = errors.New("credit reservation not found")
	ErrReservationRefunded  = errors.New("credit reservation already refunded")
	ErrReservationConfirmed = errors.New("credit reservation already confirmed")
	ErrInvalidReference     = errors.New("credit reference is required")
)

// Account is a user's credit account snapshot.
type Account struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Balance     int       `json:"balance"`
	TotalEarned int       `json:"total_earned"`
	TotalSpent  int       `json:"total_spent"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LedgerEntry represents one credit ledger mutation.
type LedgerEntry struct {
	ID            uuid.UUID       `json:"id"`
	AccountID     uuid.UUID       `json:"account_id"`
	UserID        uuid.UUID       `json:"user_id"`
	Type          string          `json:"type"`
	Amount        int             `json:"amount"`
	BalanceAfter  int             `json:"balance_after"`
	ReferenceType string          `json:"reference_type"`
	ReferenceID   uuid.UUID       `json:"reference_id"`
	Description   string          `json:"description"`
	MetadataJSON  json.RawMessage `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
}

// Service defines credit query and mutation operations.
type Service interface {
	GetAccount(ctx context.Context, userID uuid.UUID) (*Account, error)
	Reserve(ctx context.Context, userID uuid.UUID, amount int, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error)
	Confirm(ctx context.Context, userID uuid.UUID, actualAmount int, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error)
	Refund(ctx context.Context, userID uuid.UUID, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error)
	Spend(ctx context.Context, userID uuid.UUID, amount int, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error)
}

// PgService implements credit accounting using PostgreSQL.
type PgService struct {
	pool   *pgxpool.Pool
	outbox *eventbus.OutboxWriter
}

// NewPgService creates a PostgreSQL-backed billing service.
func NewPgService(pool *pgxpool.Pool, outbox *eventbus.OutboxWriter) *PgService {
	return &PgService{pool: pool, outbox: outbox}
}

func (s *PgService) GetAccount(ctx context.Context, userID uuid.UUID) (*Account, error) {
	account := &Account{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, balance, total_earned, total_spent, created_at, updated_at
		 FROM credit_accounts WHERE user_id = $1`,
		userID,
	).Scan(&account.ID, &account.UserID, &account.Balance, &account.TotalEarned, &account.TotalSpent, &account.CreatedAt, &account.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query credit account: %w", err)
	}
	return account, nil
}

func (s *PgService) Reserve(ctx context.Context, userID uuid.UUID, amount int, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error) {
	failed := true
	defer func() { observability.ObserveBilling("reserve", failed) }()
	if amount <= 0 {
		return nil, ErrInvalidCreditAmount
	}
	if referenceType == "" || referenceID == uuid.Nil {
		return nil, ErrInvalidReference
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin credit reserve tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	account, err := lockAccount(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	existing, err := findLedger(ctx, tx, userID, TypeReserve, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit idempotent reserve tx: %w", err)
		}
		failed = false
		return existing, nil
	}

	if account.Balance < amount {
		return nil, ErrInsufficientCredits
	}
	newBalance := account.Balance - amount
	if _, err := tx.Exec(ctx,
		`UPDATE credit_accounts SET balance = $2 WHERE id = $1`,
		account.ID, newBalance,
	); err != nil {
		return nil, fmt.Errorf("update reserved balance: %w", err)
	}

	entry := &LedgerEntry{
		AccountID:     account.ID,
		UserID:        userID,
		Type:          TypeReserve,
		Amount:        -amount,
		BalanceAfter:  newBalance,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
		MetadataJSON:  json.RawMessage(`{}`),
	}
	if err := insertLedger(ctx, tx, entry); err != nil {
		return nil, err
	}
	if err := s.publishCreditEvent(ctx, tx, "credits_reserved", entry); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, userID, "credits_reserved", "generation_task", referenceID, map[string]any{"amount": amount, "balance_after": newBalance}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit credit reserve tx: %w", err)
	}
	failed = false
	return entry, nil
}

func (s *PgService) Confirm(ctx context.Context, userID uuid.UUID, actualAmount int, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error) {
	failed := true
	defer func() { observability.ObserveBilling("confirm", failed) }()
	if actualAmount < 0 {
		return nil, ErrInvalidCreditAmount
	}
	if referenceType == "" || referenceID == uuid.Nil {
		return nil, ErrInvalidReference
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin credit confirm tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	account, err := lockAccount(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	existingConfirm, err := findLedger(ctx, tx, userID, TypeConfirm, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	if existingConfirm != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit idempotent confirm tx: %w", err)
		}
		failed = false
		return existingConfirm, nil
	}
	existingRefund, err := findLedger(ctx, tx, userID, TypeRefund, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	if existingRefund != nil {
		return nil, ErrReservationRefunded
	}

	reserve, err := findLedger(ctx, tx, userID, TypeReserve, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	if reserve == nil {
		return nil, ErrReservationNotFound
	}
	reservedAmount := -reserve.Amount
	newBalance := account.Balance

	switch {
	case actualAmount > reservedAmount:
		extra := actualAmount - reservedAmount
		if account.Balance < extra {
			return nil, ErrInsufficientCredits
		}
		newBalance = account.Balance - extra
		if _, err := tx.Exec(ctx,
			`UPDATE credit_accounts SET balance = $2, total_spent = total_spent + $3 WHERE id = $1`,
			account.ID, newBalance, actualAmount,
		); err != nil {
			return nil, fmt.Errorf("update confirmed extra balance: %w", err)
		}
	case actualAmount < reservedAmount:
		refundAmount := reservedAmount - actualAmount
		newBalance = account.Balance + refundAmount
		if _, err := tx.Exec(ctx,
			`UPDATE credit_accounts SET balance = $2, total_spent = total_spent + $3 WHERE id = $1`,
			account.ID, newBalance, actualAmount,
		); err != nil {
			return nil, fmt.Errorf("update confirmed refund balance: %w", err)
		}
		refundEntry := &LedgerEntry{
			AccountID:     account.ID,
			UserID:        userID,
			Type:          TypeRefund,
			Amount:        refundAmount,
			BalanceAfter:  newBalance,
			ReferenceType: referenceType,
			ReferenceID:   referenceID,
			Description:   "Refund unused reserved credits",
			MetadataJSON:  mustJSON(map[string]any{"reserved": reservedAmount, "actual": actualAmount}),
		}
		if err := insertLedger(ctx, tx, refundEntry); err != nil {
			return nil, err
		}
		if err := s.publishCreditEvent(ctx, tx, "credits_refunded", refundEntry); err != nil {
			return nil, err
		}
	default:
		if _, err := tx.Exec(ctx,
			`UPDATE credit_accounts SET total_spent = total_spent + $2 WHERE id = $1`,
			account.ID, actualAmount,
		); err != nil {
			return nil, fmt.Errorf("update confirmed spend: %w", err)
		}
	}

	entry := &LedgerEntry{
		AccountID:     account.ID,
		UserID:        userID,
		Type:          TypeConfirm,
		Amount:        0,
		BalanceAfter:  newBalance,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
		MetadataJSON:  mustJSON(map[string]any{"reserved": reservedAmount, "actual": actualAmount}),
	}
	if err := insertLedger(ctx, tx, entry); err != nil {
		return nil, err
	}
	if err := s.publishCreditEvent(ctx, tx, "credits_confirmed", entry); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, userID, "credits_confirmed", "generation_task", referenceID, map[string]any{"reserved": reservedAmount, "actual": actualAmount, "balance_after": newBalance}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit credit confirm tx: %w", err)
	}
	failed = false
	return entry, nil
}

func (s *PgService) Refund(ctx context.Context, userID uuid.UUID, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error) {
	failed := true
	defer func() { observability.ObserveBilling("refund", failed) }()
	if referenceType == "" || referenceID == uuid.Nil {
		return nil, ErrInvalidReference
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin credit refund tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	account, err := lockAccount(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	existingRefund, err := findLedger(ctx, tx, userID, TypeRefund, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	if existingRefund != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit idempotent refund tx: %w", err)
		}
		failed = false
		return existingRefund, nil
	}
	existingConfirm, err := findLedger(ctx, tx, userID, TypeConfirm, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	if existingConfirm != nil {
		return nil, ErrReservationConfirmed
	}

	reserve, err := findLedger(ctx, tx, userID, TypeReserve, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	if reserve == nil {
		return nil, ErrReservationNotFound
	}
	refundAmount := -reserve.Amount
	newBalance := account.Balance + refundAmount
	if _, err := tx.Exec(ctx,
		`UPDATE credit_accounts SET balance = $2 WHERE id = $1`,
		account.ID, newBalance,
	); err != nil {
		return nil, fmt.Errorf("update refunded balance: %w", err)
	}
	entry := &LedgerEntry{
		AccountID:     account.ID,
		UserID:        userID,
		Type:          TypeRefund,
		Amount:        refundAmount,
		BalanceAfter:  newBalance,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
		MetadataJSON:  mustJSON(map[string]any{"reserved": refundAmount}),
	}
	if err := insertLedger(ctx, tx, entry); err != nil {
		return nil, err
	}
	if err := s.publishCreditEvent(ctx, tx, "credits_refunded", entry); err != nil {
		return nil, err
	}
	if err := insertAuditTx(ctx, tx, userID, "credits_refunded", "generation_task", referenceID, map[string]any{"amount": refundAmount, "balance_after": newBalance}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit credit refund tx: %w", err)
	}
	failed = false
	return entry, nil
}

// Spend is retained for callers that need an immediate reserve+confirm debit.
func (s *PgService) Spend(ctx context.Context, userID uuid.UUID, amount int, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error) {
	if _, err := s.Reserve(ctx, userID, amount, referenceType, referenceID, description); err != nil {
		return nil, err
	}
	return s.Confirm(ctx, userID, amount, referenceType, referenceID, description)
}

func lockAccount(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*Account, error) {
	account := &Account{}
	err := tx.QueryRow(ctx,
		`SELECT id, user_id, balance, total_earned, total_spent, created_at, updated_at
		 FROM credit_accounts WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&account.ID, &account.UserID, &account.Balance, &account.TotalEarned, &account.TotalSpent, &account.CreatedAt, &account.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock credit account: %w", err)
	}
	return account, nil
}

func findLedger(ctx context.Context, tx pgx.Tx, userID uuid.UUID, entryType, referenceType string, referenceID uuid.UUID) (*LedgerEntry, error) {
	entry := &LedgerEntry{}
	err := tx.QueryRow(ctx,
		`SELECT id, account_id, user_id, type, amount, balance_after, reference_type, reference_id, description, metadata_json, created_at
		 FROM credit_ledger
		 WHERE user_id = $1 AND type = $2 AND reference_type = $3 AND reference_id = $4`,
		userID, entryType, referenceType, referenceID,
	).Scan(&entry.ID, &entry.AccountID, &entry.UserID, &entry.Type, &entry.Amount, &entry.BalanceAfter,
		&entry.ReferenceType, &entry.ReferenceID, &entry.Description, &entry.MetadataJSON, &entry.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query credit ledger: %w", err)
	}
	return entry, nil
}

func insertLedger(ctx context.Context, tx pgx.Tx, entry *LedgerEntry) error {
	if entry.MetadataJSON == nil {
		entry.MetadataJSON = json.RawMessage(`{}`)
	}
	err := tx.QueryRow(ctx,
		`INSERT INTO credit_ledger
			(account_id, user_id, type, amount, balance_after, reference_type, reference_id, description, metadata_json)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at`,
		entry.AccountID, entry.UserID, entry.Type, entry.Amount, entry.BalanceAfter,
		entry.ReferenceType, entry.ReferenceID, entry.Description, entry.MetadataJSON,
	).Scan(&entry.ID, &entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert credit ledger: %w", err)
	}
	return nil
}

func (s *PgService) publishCreditEvent(ctx context.Context, tx pgx.Tx, eventType string, entry *LedgerEntry) error {
	if s.outbox == nil {
		return nil
	}
	return s.outbox.PublishTx(ctx, tx, eventbus.TopicCreditEvents,
		eventbus.NewEvent(eventType, "credit_ledger", entry.ID, map[string]any{
			"account_id":     entry.AccountID,
			"amount":         entry.Amount,
			"balance_after":  entry.BalanceAfter,
			"reference_type": entry.ReferenceType,
			"reference_id":   entry.ReferenceID,
			"description":    entry.Description,
			"type":           entry.Type,
		}).WithUser(entry.UserID))
}

func insertAuditTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, action, resourceType string, resourceID uuid.UUID, values any) error {
	newJSON, _ := json.Marshal(values)
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (user_id, action, resource_type, resource_id, new_values_json, trace_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, action, resourceType, resourceID, newJSON, observability.TraceIDFromContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("insert billing audit log: %w", err)
	}
	return nil
}

func mustJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
