package idempotency

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type postgresRepository struct {
	db *sqlx.DB
}

func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

type recordRow struct {
	Key          uuid.UUID     `db:"key"`
	UserID       int64         `db:"user_id"`
	Endpoint     string        `db:"endpoint"`
	RequestHash  string        `db:"request_hash"`
	Status       string        `db:"status"`
	ResponseCode sql.NullInt32 `db:"response_code"`
	ResponseBody []byte        `db:"response_body"`
	CreatedAt    time.Time     `db:"created_at"`
	ExpiresAt    time.Time     `db:"expires_at"`
}

func (r *postgresRepository) Get(ctx context.Context, key uuid.UUID) (*Record, error) {
	var row recordRow
	err := r.db.GetContext(ctx, &row,
		`SELECT * FROM idempotency_keys WHERE key = $1 AND expires_at > now()`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rec := &Record{
		Key:          row.Key,
		UserID:       row.UserID,
		Endpoint:     row.Endpoint,
		RequestHash:  row.RequestHash,
		Status:       Status(row.Status),
		ResponseBody: row.ResponseBody,
		CreatedAt:    row.CreatedAt,
		ExpiresAt:    row.ExpiresAt,
	}
	if row.ResponseCode.Valid {
		rec.ResponseCode = int(row.ResponseCode.Int32)
	}
	return rec, nil
}

func (r *postgresRepository) Reserve(ctx context.Context, key uuid.UUID, userID int64, endpoint, requestHash string, ttl time.Duration) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (key, user_id, endpoint, request_hash, status, expires_at)
		VALUES ($1, $2, $3, $4, 'pending', now() + $5::interval)
		ON CONFLICT (key) DO NOTHING`,
		key, userID, endpoint, requestHash, fmtInterval(ttl),
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (r *postgresRepository) Complete(ctx context.Context, key uuid.UUID, statusCode int, body []byte) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status = 'completed', response_code = $1, response_body = $2
		WHERE key = $3`,
		statusCode, body, key,
	)
	return err
}

func fmtInterval(d time.Duration) string {
	return fmt.Sprintf("%d seconds", int64(d.Seconds()))
}
