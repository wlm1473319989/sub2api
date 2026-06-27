package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const settlementRefundPreviewCacheKeyPrefix = "settlement_refund_preview:"

func settlementRefundPreviewCacheKey(userID, subscriptionID int64) string {
	return fmt.Sprintf("%s%d:%d", settlementRefundPreviewCacheKeyPrefix, userID, subscriptionID)
}

type settlementRefundPreviewCache struct {
	rdb *redis.Client
}

func NewSettlementRefundPreviewCache(rdb *redis.Client) service.SettlementRefundPreviewCache {
	return &settlementRefundPreviewCache{rdb: rdb}
}

func (c *settlementRefundPreviewCache) GetSettlementRefundPreview(ctx context.Context, userID, subscriptionID int64) (*service.SettlementRefundPreviewCacheEntry, error) {
	val, err := c.rdb.Get(ctx, settlementRefundPreviewCacheKey(userID, subscriptionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	var entry service.SettlementRefundPreviewCacheEntry
	if err := json.Unmarshal(val, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (c *settlementRefundPreviewCache) SetSettlementRefundPreview(ctx context.Context, entry *service.SettlementRefundPreviewCacheEntry, ttl time.Duration) error {
	if entry == nil {
		return nil
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, settlementRefundPreviewCacheKey(entry.UserID, entry.SubscriptionID), payload, ttl).Err()
}

func (c *settlementRefundPreviewCache) DeleteSettlementRefundPreview(ctx context.Context, userID, subscriptionID int64) error {
	return c.rdb.Del(ctx, settlementRefundPreviewCacheKey(userID, subscriptionID)).Err()
}
