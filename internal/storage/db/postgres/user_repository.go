package postgres

import (
	"context"
	"fmt"
	"merch-store-grpc/internal/models"
	"merch-store-grpc/internal/storage/db"
	"merch-store-grpc/pkg/logger"
	"time"
)

type postgresUserRepository struct {
	conn   db.TxManager
	logger logger.Logger
}

func NewUserRepository(conn db.TxManager, log logger.Logger) db.UserRepository {
	return &postgresUserRepository{conn: conn, logger: log}
}

func (r *postgresUserRepository) CreateUser(ctx context.Context, user *models.User) (int, error) {
	pool := r.conn.GetExecutor(ctx)

	query := `
		INSERT INTO users (username, password_hash, balance, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	user.CreatedAt = time.Now()

	var userID int
	err := pool.QueryRow(ctx, query, user.Username, user.PasswordHash, user.Balance, user.CreatedAt).Scan(&userID)
	if err != nil {
		r.logger.Errorw("creating a user",
			"error", err,
			"username", user.Username,
		)
		return 0, fmt.Errorf("create user: %w", err)
	}

	return userID, nil
}

func (r *postgresUserRepository) GetUserByID(ctx context.Context, userID int) (*models.User, error) {
	pool := r.conn.GetExecutor(ctx)

	query := `
		SELECT id, username, password_hash, balance, created_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := pool.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Balance,
		&user.CreatedAt,
	)
	if err != nil {
		r.logger.Errorw("getting a user by ID",
			"error", err,
			"userID", userID,
		)
		return nil, fmt.Errorf("get a user by ID: %w", err)
	}

	return &user, nil
}

func (r *postgresUserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	pool := r.conn.GetExecutor(ctx)

	query := `
		SELECT id, username, password_hash, balance, created_at
		FROM users
		WHERE username = $1
	`

	var user models.User
	err := pool.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Balance,
		&user.CreatedAt,
	)
	if err != nil {
		r.logger.Warnw("getting a user by username",
			"error", err,
			"username", username,
		)
		return nil, fmt.Errorf("get a user by username: %w", err)
	}

	return &user, nil
}

func (r *postgresUserRepository) UpdateBalance(ctx context.Context, userID int, newBalance int) error {
	pool := r.conn.GetExecutor(ctx)

	query := `
		UPDATE users
		SET balance = $1
		WHERE id = $2
	`

	result, err := pool.Exec(ctx, query, newBalance, userID)
	if err != nil {
		r.logger.Errorw("updating a user balance",
			"error", err,
			"userID", userID,
			"newBalance", newBalance,
		)
		return fmt.Errorf("update user balance: %w", err)
	}

	if result.RowsAffected() == 0 {
		r.logger.Warnw("User not found",
			"userID", userID,
		)
		return fmt.Errorf("user with ID %d not found", userID)
	}

	return nil
}
