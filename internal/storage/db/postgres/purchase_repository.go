package postgres

import (
	"context"
	"fmt"
	"merch-store-grpc/internal/models"
	"merch-store-grpc/internal/storage/db"
	"merch-store-grpc/pkg/logger"
)

type postgresPurchaseRepository struct {
	conn   db.TxManager
	logger logger.Logger
}

func NewPurchaseRepository(conn db.TxManager, log logger.Logger) db.PurchaseRepository {
	return &postgresPurchaseRepository{conn: conn, logger: log}
}

func (r *postgresPurchaseRepository) CreatePurchase(ctx context.Context, purchase *models.Purchase) (int, error) {
	pool := r.conn.GetExecutor(ctx)

	query := `
        INSERT INTO purchases (user_id, merch_name, price)
        VALUES ($1, $2, $3)
        RETURNING id
    `

	var purchaseID int
	err := pool.QueryRow(ctx, query, purchase.UserID, purchase.MerchName, purchase.Price).Scan(&purchaseID)
	if err != nil {
		r.logger.Errorw("creating purchase",
			"error", err,
			"userID", purchase.UserID,
			"merchID", purchase.MerchName,
		)
		return 0, fmt.Errorf("create purchase: %w", err)
	}

	return purchaseID, nil
}

func (r *postgresPurchaseRepository) GetPurchaseByUserID(ctx context.Context, userID int) ([]*models.Purchase, error) {
	pool := r.conn.GetExecutor(ctx)

	query := `
        SELECT id, user_id, merch_name, price, created_at
        FROM purchases
        WHERE user_id = $1
    `

	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		r.logger.Errorw("retrieving purchase list",
			"error", err,
			"userID", userID,
		)
		return nil, fmt.Errorf("retrieve purchase list: %w", err)
	}
	defer rows.Close()

	var purchases []*models.Purchase
	for rows.Next() {
		var purchase models.Purchase
		err := rows.Scan(
			&purchase.ID,
			&purchase.UserID,
			&purchase.MerchName,
			&purchase.Price,
			&purchase.CreatedAt,
		)
		if err != nil {
			r.logger.Errorw("scanning purchase data",
				"error", err,
			)
			return nil, fmt.Errorf("reading purchase data: %w", err)
		}
		purchases = append(purchases, &purchase)
	}

	if err := rows.Err(); err != nil {
		r.logger.Errorw("processing query result",
			"error", err,
		)
		return nil, fmt.Errorf("processing query result: %w", err)
	}

	return purchases, nil
}
