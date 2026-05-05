package billing

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Incipe-win/OpenStory/internal/config"
	"github.com/Incipe-win/OpenStory/internal/db"
)

func setupBillingTest(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err)
	pool, err := db.NewPool(context.Background(), cfg.Database.URL)
	if err != nil {
		t.Skipf("skipping billing integration test: database not available: %v", err)
	}
	userID := uuid.New()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO users (id, email, username, password_hash, display_name)
		 VALUES ($1, $2, $3, 'test-hash', 'Billing Test')`,
		userID, "billing-"+userID.String()+"@example.com", "billing-"+userID.String(),
	)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(),
		`INSERT INTO credit_accounts (user_id, balance, total_earned) VALUES ($1, 100, 100)`,
		userID,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM audit_logs WHERE user_id = $1", userID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM credit_ledger WHERE user_id = $1", userID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM credit_accounts WHERE user_id = $1", userID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
		pool.Close()
	})
	return pool, userID
}

func TestReserveConfirmRefundIdempotency(t *testing.T) {
	pool, userID := setupBillingTest(t)
	ctx := context.Background()
	svc := NewPgService(pool, nil)

	refConfirm := uuid.New()
	_, err := svc.Reserve(ctx, userID, 30, "generation_task", refConfirm, "reserve")
	require.NoError(t, err)
	_, err = svc.Reserve(ctx, userID, 30, "generation_task", refConfirm, "reserve duplicate")
	require.NoError(t, err)
	assertAccount(t, svc, userID, 70, 0)
	assertLedgerCount(t, pool, userID, refConfirm, TypeReserve, 1)

	_, err = svc.Confirm(ctx, userID, 20, "generation_task", refConfirm, "confirm")
	require.NoError(t, err)
	_, err = svc.Confirm(ctx, userID, 20, "generation_task", refConfirm, "confirm duplicate")
	require.NoError(t, err)
	assertAccount(t, svc, userID, 80, 20)
	assertLedgerCount(t, pool, userID, refConfirm, TypeConfirm, 1)
	assertLedgerCount(t, pool, userID, refConfirm, TypeRefund, 1)

	refRefund := uuid.New()
	_, err = svc.Reserve(ctx, userID, 15, "generation_task", refRefund, "reserve")
	require.NoError(t, err)
	_, err = svc.Refund(ctx, userID, "generation_task", refRefund, "refund")
	require.NoError(t, err)
	_, err = svc.Refund(ctx, userID, "generation_task", refRefund, "refund duplicate")
	require.NoError(t, err)
	assertAccount(t, svc, userID, 80, 20)
	assertLedgerCount(t, pool, userID, refRefund, TypeReserve, 1)
	assertLedgerCount(t, pool, userID, refRefund, TypeRefund, 1)
}

func assertAccount(t *testing.T, svc *PgService, userID uuid.UUID, balance, spent int) {
	t.Helper()
	account, err := svc.GetAccount(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, balance, account.Balance)
	require.Equal(t, spent, account.TotalSpent)
}

func assertLedgerCount(t *testing.T, pool *pgxpool.Pool, userID, referenceID uuid.UUID, entryType string, want int) {
	t.Helper()
	var got int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM credit_ledger WHERE user_id = $1 AND reference_id = $2 AND type = $3`,
		userID, referenceID, entryType,
	).Scan(&got)
	require.NoError(t, err)
	require.Equal(t, want, got)
}
