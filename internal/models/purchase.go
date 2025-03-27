package models

import "time"

type Purchase struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	MerchName string    `json:"merch_name"`
	Price     int       `json:"price"`
	CreatedAt time.Time `json:"created_at"`
}
