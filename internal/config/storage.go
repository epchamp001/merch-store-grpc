package config

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/sourcegraph/conc/pool"
	"merch-store-grpc/pkg/logger"
	"time"
)

type StorageConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

type PostgresConfig struct {
	Hosts              []string   `mapstructure:"hosts"`
	Port               int        `mapstructure:"port"`
	Database           string     `mapstructure:"database"`
	Username           string     `mapstructure:"username"`
	Password           string     `mapstructure:"password"`
	SSLMode            string     `mapstructure:"ssl_mode"`
	ConnectionAttempts int        `mapstructure:"connection_attempts"`
	Pool               PoolConfig `mapstructure:"pool"`
}

type PoolConfig struct {
	MaxConnections    int `mapstructure:"max_connections"`
	MinConnections    int `mapstructure:"min_connections"`
	MaxLifeTime       int `mapstructure:"max_lifetime"`
	MaxIdleTime       int `mapstructure:"max_idle_time"`
	HealthCheckPeriod int `mapstructure:"health_check_period"`
}

type RedisConfig struct {
	Host               []string `mapstructure:"host"`
	Port               int      `mapstructure:"port"`
	Password           string   `mapstructure:"password"`
	DB                 int      `mapstructure:"db"`
	ConnectionAttempts int      `mapstructure:"connection_attempts"`
}

func (s *StorageConfig) ConnectionToPostgres(log logger.Logger) (*pgxpool.Pool, error) {
	cfg := s.Postgres
	dsn := s.GetDSN()

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres DSN: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.Pool.MaxConnections)
	poolConfig.MinConns = int32(cfg.Pool.MinConnections)
	poolConfig.MaxConnLifetime = time.Duration(cfg.Pool.MaxLifeTime) * time.Second
	poolConfig.MaxConnIdleTime = time.Duration(cfg.Pool.MaxIdleTime) * time.Second
	poolConfig.HealthCheckPeriod = time.Duration(cfg.Pool.HealthCheckPeriod) * time.Second

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	var attempt int
	for attempt < cfg.ConnectionAttempts {
		if err := pool.Ping(context.Background()); err == nil {
			break
		}
		attempt++
		time.Sleep(time.Second * 2)
		log.Warnw("Attempt to connect to PostgreSQL",
			"attempt", attempt,
			"max_attempts", cfg.ConnectionAttempts,
			"error", err,
		)
	}

	if attempt == cfg.ConnectionAttempts {
		return nil, fmt.Errorf("failed to connect to PostgreSQL after %d attempts", cfg.ConnectionAttempts)
	}

	return pool, nil
}

func (s *StorageConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		s.Postgres.Hosts[0], s.Postgres.Port, s.Postgres.Username, s.Postgres.Password, s.Postgres.Database, s.Postgres.SSLMode,
	)
}

func (s *StorageConfig) ConnectionToRedis(log logger.Logger) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%d", s.Redis.Host[0], s.Redis.Port)

	client := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   s.Redis.DB,
	})

	var attempt int
	var err error

	for attempt < s.Redis.ConnectionAttempts {
		err = client.Ping(context.Background()).Err()
		if err == nil {
			break
		}
		attempt++
		log.Warnw("Attempt to connect to Redis",
			"attempt", attempt,
			"max_attempts", s.Redis.ConnectionAttempts,
			"error", err,
		)
		time.Sleep(2 * time.Second)
	}

	if attempt == s.Redis.ConnectionAttempts {
		return nil, fmt.Errorf("connect to Redis after %d attempts: %w", s.Redis.ConnectionAttempts, err)
	}

	return client, nil
}
