-- +goose Up
CREATE TABLE purchases (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    merch_name TEXT NOT NULL,
    price INT,
    created_at TIMESTAMP DEFAULT now()
);
-- +goose Down
DROP TABLE purchases;
