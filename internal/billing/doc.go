// Package billing manages user credits, usage metering, and billing records.
package billing

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Incipe-win/OpenStory/internal/eventbus"
)

var (
	ErrAccountNotFound     = errors.New("credit account not found")
	ErrInsufficientCredits = errors.New("insufficient credits")
	ErrInvalidCreditAmount = errors.New("credit amount must be positive")
)

// LedgerEntry represents one credit ledger mutation.
type LedgerEntry struct {
	ID            uuid.UUID `json:"id"`
	AccountID     uuid.UUID `json:"account_id"`
	UserID        uuid.UUID `json:"user_id"`
	Type          string    `json:"type"`
	Amount        int       `json:"amount"`
	BalanceAfter  int       `json:"balance_after"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   uuid.UUID `json:"reference_id"`
	Description   string    `json:"description"`
}

// Service defines credit mutation operations.
type Service interface {
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

// Spend atomically debits credits, writes a ledger row, and appends a credit event.
func (s *PgService) Spend(ctx context.Context, userID uuid.UUID, amount int, referenceType string, referenceID uuid.UUID, description string) (*LedgerEntry, error) {
	if amount <= 0 {
		return nil, ErrInvalidCreditAmount
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin credit spend tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var accountID uuid.UUID
	var balance int
	if err := tx.QueryRow(ctx,
		`SELECT id, balance FROM credit_accounts WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&accountID, &balance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("query credit account: %w", err)
	}

	if balance < amount {
		return nil, ErrInsufficientCredits
	}

	newBalance := balance - amount
	if _, err := tx.Exec(ctx,
		`UPDATE credit_accounts
		    SET balance = $2, total_spent = total_spent + $3
		  WHERE id = $1`,
		accountID, newBalance, amount,
	); err != nil {
		return nil, fmt.Errorf("update credit account: %w", err)
	}

	entry := &LedgerEntry{
		AccountID:     accountID,
		UserID:        userID,
		Type:          "spend",
		Amount:        -amount,
		BalanceAfter:  newBalance,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO credit_ledger
			(account_id, user_id, type, amount, balance_after, reference_type, reference_id, description)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id`,
		entry.AccountID, entry.UserID, entry.Type, entry.Amount, entry.BalanceAfter,
		entry.ReferenceType, entry.ReferenceID, entry.Description,
	).Scan(&entry.ID); err != nil {
		return nil, fmt.Errorf("insert credit ledger: %w", err)
	}

	if s.outbox != nil {
		if err := s.outbox.PublishTx(ctx, tx, eventbus.TopicCreditEvents,
			eventbus.NewEvent("credits_spent", "credit_ledger", entry.ID, map[string]any{
				"account_id":     entry.AccountID,
				"amount":         entry.Amount,
				"balance_after":  entry.BalanceAfter,
				"reference_type": entry.ReferenceType,
				"reference_id":   entry.ReferenceID,
				"description":    entry.Description,
			}).WithUser(userID)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit credit spend tx: %w", err)
	}
	return entry, nil
}
