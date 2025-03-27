package db

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"merch-store-grpc/internal/models"
)

type Repository interface {
	UserRepository
	PurchaseRepository
	TransactionRepository
}

type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) (int, error)
	GetUserByID(ctx context.Context, userID int) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	UpdateBalance(ctx context.Context, userID int, newBalance int) error
}

type PurchaseRepository interface {
	CreatePurchase(ctx context.Context, purchase *models.Purchase) (int, error)
	GetPurchaseByUserID(ctx context.Context, userID int) ([]*models.Purchase, error)
}

type TransactionRepository interface {
	CreateTransaction(ctx context.Context, transaction *models.Transaction) (int, error)
	GetTransactionByUserID(ctx context.Context, userID int) ([]*models.Transaction, error)
}

type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type TxManager interface {
	GetExecutor(ctx context.Context) Executor
	WithTx(ctx context.Context, isoLevel pgx.TxIsoLevel, accessMode pgx.TxAccessMode, fn func(ctx context.Context) error) error
}

type postgresRepository struct {
	UserRepository
	PurchaseRepository
	TransactionRepository
}

func NewRepository(
	userRepo UserRepository,
	purchaseRepo PurchaseRepository,
	transactionRepo TransactionRepository,
) Repository {
	return &postgresRepository{
		UserRepository:        userRepo,
		PurchaseRepository:    purchaseRepo,
		TransactionRepository: transactionRepo,
	}
}
