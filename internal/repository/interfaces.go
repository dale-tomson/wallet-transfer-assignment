package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/dale-tomson/wallet-transfer-assignment/internal/domain"
)

type queryExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Transactor defines transaction boundary execution
type Transactor interface {
	RunInTx(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// WalletRepository defines data access methods for wallets
type WalletRepository interface {
	CreateWallet(ctx context.Context, tx *sql.Tx, wallet *domain.Wallet) error
	GetWalletForUpdate(ctx context.Context, tx *sql.Tx, id string) (*domain.Wallet, error)
	UpdateBalances(ctx context.Context, tx *sql.Tx, fromWalletID, toWalletID string, amount int64) error
}

// TransferRepository defines data access methods for transfers and ledger entries
type TransferRepository interface {
	CreateTransferAndEntries(ctx context.Context, tx *sql.Tx, transfer *domain.Transfer, debit *domain.LedgerEntry, credit *domain.LedgerEntry) error
}

// IdempotencyRepository defines data access methods for idempotency leases and records
type IdempotencyRepository interface {
	Reserve(ctx context.Context, tx *sql.Tx, key string, requestHash string, ttl time.Duration) (*domain.IdempotencyRecord, error)
	Resolve(ctx context.Context, tx *sql.Tx, key string, code int, responseBody string) error
	ReleaseOrSetFailed(ctx context.Context, tx *sql.Tx, key string) error
}
