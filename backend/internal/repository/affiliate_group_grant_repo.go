package repository

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type affiliateGroupGrantRepository struct {
	client *dbent.Client
}

func NewAffiliateGroupGrantRepository(client *dbent.Client, _ *sql.DB) service.AffiliateGroupGrantRepository {
	return &affiliateGroupGrantRepository{client: client}
}

func (r *affiliateGroupGrantRepository) GrantForOrder(ctx context.Context, input service.AffiliateGroupGrantInput) (*service.AffiliateGroupGrant, bool, error) {
	if input.UserID <= 0 || input.GroupID <= 0 || input.SourceOrderID <= 0 || input.PayAmount <= 0 || input.GrantDays <= 0 {
		return nil, false, nil
	}
	if input.StartsAt.IsZero() || input.Now.IsZero() {
		return nil, false, errors.New("invalid affiliate group grant period")
	}

	var grant *service.AffiliateGroupGrant
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := txClient.ExecContext(txCtx, "SELECT pg_advisory_xact_lock($1)", affiliateGroupGrantLockKey(input.UserID, input.GroupID)); err != nil {
			return fmt.Errorf("lock affiliate group grant window: %w", err)
		}

		rows, err := txClient.QueryContext(txCtx, `
WITH current_window AS (
    SELECT MAX(expires_at) AS max_expires_at
    FROM affiliate_group_grants
    WHERE user_id = $1
      AND group_id = $2
      AND expires_at > $8
),
period AS (
    SELECT CASE
        WHEN max_expires_at IS NOT NULL AND max_expires_at > $7 THEN max_expires_at
        ELSE $7
    END AS starts_at
    FROM current_window
),
inserted AS (
    INSERT INTO affiliate_group_grants (
        user_id,
        group_id,
        source_user_id,
        source_order_id,
        pay_amount,
        grant_days,
        starts_at,
        expires_at,
        created_at,
        updated_at
    )
    SELECT
        $1,
        $2,
        $3,
        $4,
        $5,
        $6,
        starts_at,
        starts_at + ($6::integer * INTERVAL '1 day'),
        NOW(),
        NOW()
    FROM period
    ON CONFLICT (user_id, group_id, source_order_id) DO NOTHING
    RETURNING
        id,
        user_id,
        group_id,
        source_user_id,
        source_order_id,
        pay_amount::double precision,
        grant_days,
        starts_at,
        expires_at,
        created_at,
        updated_at
)
SELECT
    id,
    user_id,
    group_id,
    source_user_id,
    source_order_id,
    pay_amount,
    grant_days,
    starts_at,
    expires_at,
    created_at,
    updated_at
FROM inserted`,
			input.UserID,
			input.GroupID,
			nullableGrantSourceUserID(input.SourceUserID),
			input.SourceOrderID,
			input.PayAmount,
			input.GrantDays,
			input.StartsAt,
			input.Now,
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return nil
		}
		out, err := scanAffiliateGroupGrant(rows)
		if err != nil {
			return err
		}
		grant = out
		return rows.Err()
	})
	if err != nil {
		return nil, false, err
	}
	return grant, grant != nil, nil
}

func (r *affiliateGroupGrantRepository) ListActiveByUserID(ctx context.Context, userID int64, now time.Time) ([]service.TimedGroupGrant, error) {
	if userID <= 0 {
		return nil, nil
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT group_id, MAX(expires_at) AS expires_at
FROM affiliate_group_grants
WHERE user_id = $1
  AND expires_at > $2
GROUP BY group_id
ORDER BY group_id`, userID, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.TimedGroupGrant, 0)
	for rows.Next() {
		var item service.TimedGroupGrant
		if err := rows.Scan(&item.GroupID, &item.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *affiliateGroupGrantRepository) ClaimExpiredForProcessing(ctx context.Context, now time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 500
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
WITH due AS (
    SELECT id, user_id
    FROM affiliate_group_grants
    WHERE expires_at <= $1
      AND expired_processed_at IS NULL
    ORDER BY expires_at, id
    LIMIT $2
    FOR UPDATE SKIP LOCKED
),
updated AS (
    UPDATE affiliate_group_grants agg
    SET expired_processed_at = $1,
        updated_at = NOW()
    FROM due
    WHERE agg.id = due.id
    RETURNING due.user_id
)
SELECT DISTINCT user_id
FROM updated
ORDER BY user_id`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	userIDs := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (r *affiliateGroupGrantRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin affiliate group grant transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit affiliate group grant transaction: %w", err)
	}
	return nil
}

func scanAffiliateGroupGrant(rows interface {
	Scan(dest ...any) error
}) (*service.AffiliateGroupGrant, error) {
	var out service.AffiliateGroupGrant
	var sourceUserID sql.NullInt64
	if err := rows.Scan(
		&out.ID,
		&out.UserID,
		&out.GroupID,
		&sourceUserID,
		&out.SourceOrderID,
		&out.PayAmount,
		&out.GrantDays,
		&out.StartsAt,
		&out.ExpiresAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if sourceUserID.Valid {
		out.SourceUserID = &sourceUserID.Int64
	}
	return &out, nil
}

func nullableGrantSourceUserID(userID int64) any {
	if userID <= 0 {
		return nil
	}
	return userID
}

func affiliateGroupGrantLockKey(userID, groupID int64) int64 {
	h := fnv.New64a()
	var buf [16]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(userID))
	binary.LittleEndian.PutUint64(buf[8:], uint64(groupID))
	_, _ = h.Write(buf[:])
	return int64(h.Sum64())
}
