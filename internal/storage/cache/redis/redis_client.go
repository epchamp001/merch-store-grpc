package redis

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"merch-store-grpc/internal/storage/cache"
	"merch-store-grpc/pkg/logger"
)

type RedisCacheRepository struct {
	rdb    *redis.Client
	logger logger.Logger
}

func NewRedisCacheRepository(rdb *redis.Client, logger logger.Logger) cache.CacheRepository {
	return &RedisCacheRepository{rdb: rdb, logger: logger}
}

var (
	// Скрипт для списания баланса (используется в DeductBalance)
	deductBalanceScript = redis.NewScript(`
	local current = tonumber(redis.call("GET", KEYS[1]) or "0")
	if current < tonumber(ARGV[1]) then
		return -1
	end
	return redis.call("DECRBY", KEYS[1], ARGV[1])
	`)

	// Скрипт для перевода монет (используется в TransferCoins)
	transferCoinsScript = redis.NewScript(`
	local from_balance = tonumber(redis.call("GET", KEYS[1]) or "0")
	if from_balance < tonumber(ARGV[1]) then
		return -1
	end
	redis.call("DECRBY", KEYS[1], ARGV[1])
	redis.call("INCRBY", KEYS[2], ARGV[1])
	return 1
	`)
)

func (r *RedisCacheRepository) SetBalance(ctx context.Context, userID int, balance int) error {
	key := fmt.Sprintf("balance:%d", userID)
	return r.rdb.Set(ctx, key, balance, 0).Err()
}

func (r *RedisCacheRepository) GetBalance(ctx context.Context, userID int) (int, error) {
	key := fmt.Sprintf("balance:%d", userID)
	return r.rdb.Get(ctx, key).Int()
}

func (r *RedisCacheRepository) DeductBalance(ctx context.Context, userID int, amount int) error {
	key := fmt.Sprintf("balance:%d", userID)
	res, err := deductBalanceScript.Run(ctx, r.rdb, []string{key}, amount).Result()
	if err != nil {
		return err
	}

	if res.(int64) < 0 {
		return fmt.Errorf("insufficient funds")
	}
	return nil
}

func (r *RedisCacheRepository) IncrementBalance(ctx context.Context, userID int, amount int) error {
	key := fmt.Sprintf("balance:%d", userID)
	return r.rdb.IncrBy(ctx, key, int64(amount)).Err()
}

func (r *RedisCacheRepository) TransferCoins(ctx context.Context, fromUser, toUser int, amount int) error {
	fromKey := fmt.Sprintf("balance:%d", fromUser)
	toKey := fmt.Sprintf("balance:%d", toUser)
	res, err := transferCoinsScript.Run(ctx, r.rdb, []string{fromKey, toKey}, amount).Result()
	if err != nil {
		return err
	}
	if res.(int64) < 0 {
		return fmt.Errorf("insufficient funds for transfer")
	}
	return nil
}

func (r *RedisCacheRepository) LoadCatalog(ctx context.Context, catalog map[string]interface{}) error {
	return r.rdb.HSet(ctx, "merch_catalog", catalog).Err()
}

func (r *RedisCacheRepository) GetPrice(ctx context.Context, merchName string) (int, error) {
	price, err := r.rdb.HGet(ctx, "merch_catalog", merchName).Int()
	if err != nil {
		return 0, fmt.Errorf("getting the price for %s: %w", merchName, err)
	}
	return price, nil
}
