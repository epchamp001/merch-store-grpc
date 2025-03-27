package postgres

import (
	"context"
	"fmt"
	"merch-store-grpc/internal/models"
	"merch-store-grpc/internal/storage/db"
	"merch-store-grpc/pkg/logger"
)

type postgresTransactionRepository struct {
	conn   db.TxManager
	logger logger.Logger
}

func NewTransactionRepository(conn db.TxManager, log logger.Logger) db.TransactionRepository {
	return &postgresTransactionRepository{conn: conn, logger: log}
}

func (r *postgresTransactionRepository) CreateTransaction(ctx context.Context, transaction *models.Transaction) (int, error) {
	pool := r.conn.GetExecutor(ctx)

	query := `
        INSERT INTO transactions (sender_id, receiver_id, amount, created_at)
        VALUES ($1, $2, $3, $4)
        RETURNING id
    `

	var transactionID int
	err := pool.QueryRow(ctx, query, transaction.SenderID, transaction.ReceiverID, transaction.Amount, transaction.CreatedAt).Scan(&transactionID)
	if err != nil {
		r.logger.Errorw("creating transaction",
			"error", err,
			"senderID", transaction.SenderID,
			"receiverID", transaction.ReceiverID,
		)
		return 0, fmt.Errorf("create transaction: %w", err)
	}

	return transactionID, nil
}

func (r *postgresTransactionRepository) GetTransactionByUserID(ctx context.Context, userID int) ([]*models.Transaction, error) {
	pool := r.conn.GetExecutor(ctx)

	query := `
        SELECT id, sender_id, receiver_id, amount, created_at
        FROM transactions
        WHERE sender_id = $1 OR receiver_id = $1
    `

	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		r.logger.Errorw("retrieving transaction list",
			"error", err,
			"userID", userID,
		)
		return nil, fmt.Errorf("retrieve transaction list: %w", err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		var transaction models.Transaction
		err := rows.Scan(
			&transaction.ID,
			&transaction.SenderID,
			&transaction.ReceiverID,
			&transaction.Amount,
			&transaction.CreatedAt,
		)
		if err != nil {
			r.logger.Errorw("scanning transaction data",
				"error", err,
			)
			return nil, fmt.Errorf("reading transaction data: %w", err)
		}
		transactions = append(transactions, &transaction)
	}

	if err := rows.Err(); err != nil {
		r.logger.Errorw("processing query result",
			"error", err,
		)
		return nil, fmt.Errorf("processing query result: %w", err)
	}

	return transactions, nil
}
