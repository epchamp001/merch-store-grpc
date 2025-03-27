package cache

import (
	"context"
)

type CacheRepository interface {
	SetBalance(ctx context.Context, userID int, balance int) error
	GetBalance(ctx context.Context, userID int) (int, error)
	DeductBalance(ctx context.Context, userID int, amount int) error
	IncrementBalance(ctx context.Context, userID int, amount int) error
	TransferCoins(ctx context.Context, fromUser, toUser int, amount int) error

	LoadCatalog(ctx context.Context, catalog map[string]interface{}) error
	GetPrice(ctx context.Context, merchName string) (int, error)
}
