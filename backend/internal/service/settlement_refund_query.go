package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionsettlementorder"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type SettlementRefundListFilter struct {
	UserID        *int64
	SubscriptionID *int64
	Status        string
}

type SettlementRefundRequestView struct {
	Request               *SettlementRefundRequestRecord
	User                  *User
	Subscription          *UserSubscription
	CurrentSettlementHead *SubscriptionSettlementOrderView
	ExpectedSettlementHead *SubscriptionSettlementOrderView
	GatewayRefundedTotal   float64
	SucceededAllocations   int
	FailedAllocations      int
	SkippedAllocations     int
}

type SettlementRefundSubscriptionMarker struct {
	RefundRequestID int64
	Status          string
}

func (s *SettlementRefundService) ListSettlementRefundRequests(ctx context.Context, params pagination.PaginationParams, filter *SettlementRefundListFilter) ([]SettlementRefundRequestView, *pagination.PaginationResult, error) {
	client, err := s.settlementRefundClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	where, args := settlementRefundRequestWhereClause(filter)
	total, err := settlementRefundRequestCount(ctx, client, where, args...)
	if err != nil {
		return nil, nil, err
	}

	rows, err := settlementRefundRequestList(ctx, client, where, params, args...)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return []SettlementRefundRequestView{}, paginationResultFromTotal(total, params), nil
	}

	if err := hydrateSettlementRefundRequestAllocations(ctx, client, rows); err != nil {
		return nil, nil, err
	}
	views, err := s.hydrateSettlementRefundRequestViews(ctx, client, rows, true)
	if err != nil {
		return nil, nil, err
	}
	return views, paginationResultFromTotal(total, params), nil
}

func (s *SettlementRefundService) GetSettlementRefundRequestView(ctx context.Context, requestID int64) (*SettlementRefundRequestView, error) {
	client, err := s.settlementRefundClient(ctx)
	if err != nil {
		return nil, err
	}

	record, err := querySettlementRefundRequestByID(ctx, client, requestID)
	if err != nil {
		return nil, err
	}
	record.Allocations, err = querySettlementRefundAllocations(ctx, client, requestID)
	if err != nil {
		return nil, err
	}

	views, err := s.hydrateSettlementRefundRequestViews(ctx, client, []*SettlementRefundRequestRecord{record}, true)
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, ErrSettlementRefundRequestNotFound
	}
	return &views[0], nil
}

func (s *SettlementRefundService) GetUserSettlementRefundRequestView(ctx context.Context, userID, requestID int64) (*SettlementRefundRequestView, error) {
	view, err := s.GetSettlementRefundRequestView(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if view == nil || view.Request == nil || view.Request.UserID != userID {
		return nil, ErrSettlementRefundRequestNotFound
	}
	return view, nil
}

func (s *SettlementRefundService) ListUserSettlementRefundRequests(ctx context.Context, userID int64, params pagination.PaginationParams) ([]SettlementRefundRequestView, *pagination.PaginationResult, error) {
	return s.ListSettlementRefundRequests(ctx, params, &SettlementRefundListFilter{UserID: &userID})
}

func (s *SettlementRefundService) GetActiveRefundMarkersBySubscriptionIDs(ctx context.Context, subscriptionIDs []int64) (map[int64]SettlementRefundSubscriptionMarker, error) {
	ids := settlementRefundUniquePositiveInt64s(subscriptionIDs)
	if len(ids) == 0 {
		return map[int64]SettlementRefundSubscriptionMarker{}, nil
	}

	client, err := s.settlementRefundClient(ctx)
	if err != nil {
		return nil, err
	}
	return querySettlementRefundSubscriptionMarkers(ctx, client, ids)
}

func (s *SettlementRefundService) hydrateSettlementRefundRequestViews(ctx context.Context, client *dbent.Client, records []*SettlementRefundRequestRecord, includeAllocations bool) ([]SettlementRefundRequestView, error) {
	if len(records) == 0 {
		return []SettlementRefundRequestView{}, nil
	}

	userIDs := make([]int64, 0, len(records))
	subscriptionIDs := make([]int64, 0, len(records))
	settlementIDs := make([]int64, 0, len(records)*2)
	seenUsers := make(map[int64]struct{}, len(records))
	seenSubscriptions := make(map[int64]struct{}, len(records))
	seenSettlements := make(map[int64]struct{}, len(records)*2)
	for _, record := range records {
		if record == nil {
			continue
		}
		if _, ok := seenUsers[record.UserID]; !ok {
			seenUsers[record.UserID] = struct{}{}
			userIDs = append(userIDs, record.UserID)
		}
		if _, ok := seenSubscriptions[record.SubscriptionID]; !ok {
			seenSubscriptions[record.SubscriptionID] = struct{}{}
			subscriptionIDs = append(subscriptionIDs, record.SubscriptionID)
		}
		if _, ok := seenSettlements[record.SettlementID]; !ok {
			seenSettlements[record.SettlementID] = struct{}{}
			settlementIDs = append(settlementIDs, record.SettlementID)
		}
		if _, ok := seenSettlements[record.ExpectedSettlementID]; !ok {
			seenSettlements[record.ExpectedSettlementID] = struct{}{}
			settlementIDs = append(settlementIDs, record.ExpectedSettlementID)
		}
	}

	usersByID := make(map[int64]*User, len(userIDs))
	if len(userIDs) > 0 {
		users, err := client.User.Query().
			Where(dbuser.IDIn(userIDs...)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("load settlement refund users: %w", err)
		}
		for _, user := range users {
			usersByID[user.ID] = userEntityToService(user)
		}
	}

	subscriptionsByID := make(map[int64]*UserSubscription, len(subscriptionIDs))
	if len(subscriptionIDs) > 0 {
		subs, err := client.UserSubscription.Query().
			Where(usersubscription.IDIn(subscriptionIDs...)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("load settlement refund subscriptions: %w", err)
		}
		for _, sub := range subs {
			subscriptionsByID[sub.ID] = userSubscriptionEntityToService(sub)
		}
	}

	settlementsByID := make(map[int64]*SubscriptionSettlementOrderView, len(settlementIDs))
	if len(settlementIDs) > 0 {
		settlements, err := client.SubscriptionSettlementOrder.Query().
			Where(subscriptionsettlementorder.IDIn(settlementIDs...)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("load settlement refund settlements: %w", err)
		}
		for _, settlement := range settlements {
			view := subscriptionSettlementOrderViewFromEnt(settlement)
			settlementsByID[settlement.ID] = &view
		}
	}

	views := make([]SettlementRefundRequestView, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		view := SettlementRefundRequestView{
			Request:                record,
			User:                   usersByID[record.UserID],
			Subscription:           subscriptionsByID[record.SubscriptionID],
			CurrentSettlementHead:  settlementsByID[record.SettlementID],
			ExpectedSettlementHead: settlementsByID[record.ExpectedSettlementID],
		}
		if includeAllocations {
			for _, allocation := range record.Allocations {
				switch allocation.Status {
				case SettlementRefundAllocationStatusSucceeded:
					view.SucceededAllocations++
					view.GatewayRefundedTotal = roundSettlementRefundValue(view.GatewayRefundedTotal + allocation.GatewayRefundAmount)
				case SettlementRefundAllocationStatusFailed:
					view.FailedAllocations++
				case SettlementRefundAllocationStatusSkipped:
					view.SkippedAllocations++
				}
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func settlementRefundRequestWhereClause(filter *SettlementRefundListFilter) (string, []any) {
	clauses := []string{"status NOT IN ('previewed', 'expired')"}
	args := make([]any, 0, 3)
	if filter == nil {
		return strings.Join(clauses, " AND "), args
	}
	if filter.UserID != nil && *filter.UserID > 0 {
		args = append(args, *filter.UserID)
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if filter.SubscriptionID != nil && *filter.SubscriptionID > 0 {
		args = append(args, *filter.SubscriptionID)
		clauses = append(clauses, fmt.Sprintf("subscription_id = $%d", len(args)))
	}
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	return strings.Join(clauses, " AND "), args
}

func settlementRefundRequestCount(ctx context.Context, client *dbent.Client, where string, args ...any) (int64, error) {
	query := `SELECT COUNT(*) FROM subscription_refund_requests WHERE ` + where
	var total int64
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("count settlement refund requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, fmt.Errorf("count settlement refund requests: %w", err)
		}
		return 0, nil
	}
	if err := rows.Scan(&total); err != nil {
		return 0, fmt.Errorf("count settlement refund requests: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("count settlement refund requests: %w", err)
	}
	return total, nil
}

func settlementRefundRequestList(ctx context.Context, client *dbent.Client, where string, params pagination.PaginationParams, args ...any) ([]*SettlementRefundRequestRecord, error) {
	limit := params.Limit()
	offset := params.Offset()
	query := `
SELECT
    id,
    user_id,
    subscription_id,
    settlement_id,
    expected_settlement_id,
    status,
    refund_mode,
    COALESCE(currency, ''),
    reason,
    refund_residual_value::double precision,
    gateway_refundable_total::double precision,
    manual_transfer_amount::double precision,
    preview_token_hash,
    preview_fingerprint,
    preview_issued_at,
    preview_expires_at,
    submitted_at,
    frozen_at,
    completed_at,
    cancelled_at,
    original_subscription_status,
    original_subscription_expires_at,
    manual_receiver_type,
    manual_receiver_name,
    manual_receiver_account,
    manual_receiver_qr_image_url,
    manual_receiver_remark,
    manual_transfer_proof_url,
    manual_transfer_proof_uploaded_at,
    manual_transfer_operator_user_id,
    admin_note,
    created_at,
    updated_at
FROM subscription_refund_requests
WHERE ` + where + `
ORDER BY created_at DESC, id DESC
OFFSET $` + strconv.Itoa(len(args)+1) + ` LIMIT $` + strconv.Itoa(len(args)+2)

	rows, err := client.QueryContext(ctx, query, append(args, offset, limit)...)
	if err != nil {
		return nil, fmt.Errorf("list settlement refund requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	records := make([]*SettlementRefundRequestRecord, 0)
	for rows.Next() {
		record, scanErr := scanSettlementRefundRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund request list: %w", err)
	}
	return records, nil
}

func querySettlementRefundSubscriptionMarkers(ctx context.Context, client *dbent.Client, subscriptionIDs []int64) (map[int64]SettlementRefundSubscriptionMarker, error) {
	if len(subscriptionIDs) == 0 {
		return map[int64]SettlementRefundSubscriptionMarker{}, nil
	}

	statuses := []string{
		SettlementRefundStatusSubmitted,
		SettlementRefundStatusGatewayProcessing,
		SettlementRefundStatusManualPending,
		SettlementRefundStatusFailed,
	}
	args := make([]any, 0, len(subscriptionIDs)+len(statuses))
	subscriptionPlaceholders := make([]string, 0, len(subscriptionIDs))
	for _, subscriptionID := range subscriptionIDs {
		args = append(args, subscriptionID)
		subscriptionPlaceholders = append(subscriptionPlaceholders, "$"+strconv.Itoa(len(args)))
	}
	statusPlaceholders := make([]string, 0, len(statuses))
	for _, status := range statuses {
		args = append(args, status)
		statusPlaceholders = append(statusPlaceholders, "$"+strconv.Itoa(len(args)))
	}

	query := `
SELECT DISTINCT ON (subscription_id)
    subscription_id,
    id,
    status
FROM subscription_refund_requests
WHERE subscription_id IN (` + strings.Join(subscriptionPlaceholders, ", ") + `)
  AND status IN (` + strings.Join(statusPlaceholders, ", ") + `)
ORDER BY subscription_id, created_at DESC, id DESC`

	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query settlement refund subscription markers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	markers := make(map[int64]SettlementRefundSubscriptionMarker, len(subscriptionIDs))
	for rows.Next() {
		var (
			subscriptionID int64
			refundRequestID int64
			status string
		)
		if err := rows.Scan(&subscriptionID, &refundRequestID, &status); err != nil {
			return nil, fmt.Errorf("scan settlement refund subscription marker: %w", err)
		}
		markers[subscriptionID] = SettlementRefundSubscriptionMarker{
			RefundRequestID: refundRequestID,
			Status:          strings.TrimSpace(status),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund subscription markers: %w", err)
	}
	return markers, nil
}

func paginationResultFromTotal(total int64, params pagination.PaginationParams) *pagination.PaginationResult {
	limit := params.Limit()
	pages := int(total) / limit
	if int(total)%limit > 0 {
		pages++
	}
	return &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: limit,
		Pages:    pages,
	}
}

func settlementRefundUniquePositiveInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func hydrateSettlementRefundRequestAllocations(ctx context.Context, client *dbent.Client, records []*SettlementRefundRequestRecord) error {
	if len(records) == 0 {
		return nil
	}
	requestIDs := make([]int64, 0, len(records))
	for _, record := range records {
		if record == nil || record.ID <= 0 {
			continue
		}
		requestIDs = append(requestIDs, record.ID)
	}
	if len(requestIDs) == 0 {
		return nil
	}

	allocationsByRequestID, err := querySettlementRefundAllocationsByRequestIDs(ctx, client, requestIDs)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record == nil {
			continue
		}
		record.Allocations = allocationsByRequestID[record.ID]
		if record.Allocations == nil {
			record.Allocations = []SettlementRefundAllocationRecord{}
		}
	}
	return nil
}

func querySettlementRefundAllocationsByRequestIDs(ctx context.Context, client *dbent.Client, requestIDs []int64) (map[int64][]SettlementRefundAllocationRecord, error) {
	ids := settlementRefundUniquePositiveInt64s(requestIDs)
	result := make(map[int64][]SettlementRefundAllocationRecord, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	args := make([]any, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for _, requestID := range ids {
		args = append(args, requestID)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}

	query := `
SELECT
    id,
    refund_request_id,
    payment_order_id,
    payment_provider_instance_id,
    order_amount::double precision,
    order_pay_amount::double precision,
    already_refunded_amount::double precision,
    refundable_order_amount::double precision,
    allocated_refund_value::double precision,
    gateway_refund_amount::double precision,
    COALESCE(currency, ''),
    status,
    gateway_refund_trade_no,
    failed_reason,
    processed_at,
    created_at,
    updated_at
FROM subscription_refund_allocations
WHERE refund_request_id IN (` + strings.Join(placeholders, ", ") + `)
ORDER BY refund_request_id ASC, id ASC`

	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query settlement refund allocations by request ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		allocation, scanErr := scanSettlementRefundAllocation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[allocation.RefundRequestID] = append(result[allocation.RefundRequestID], *allocation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund allocations by request ids: %w", err)
	}
	return result, nil
}

func userEntityToService(u *dbent.User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:            u.ID,
		Email:         u.Email,
		Username:      u.Username,
		Role:          u.Role,
		Balance:       u.Balance,
		Concurrency:   u.Concurrency,
		Status:        u.Status,
		AllowedGroups: nil,
		LastActiveAt:  u.LastActiveAt,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
		DeletedAt:     u.DeletedAt,
		Notes:         u.Notes,
		LastUsedAt:    nil,
	}
}

func userSubscriptionEntityToService(m *dbent.UserSubscription) *UserSubscription {
	if m == nil {
		return nil
	}
	return &UserSubscription{
		ID:                 m.ID,
		UserID:             m.UserID,
		PlanID:             m.PlanID,
		PlanNameSnapshot:   m.PlanNameSnapshot,
		PlanPriceSnapshot:  m.PlanPriceSnapshot,
		StartsAt:           m.StartsAt,
		ExpiresAt:          m.ExpiresAt,
		Status:             m.Status,
		DailyWindowStart:   m.DailyWindowStart,
		WeeklyWindowStart:  m.WeeklyWindowStart,
		MonthlyWindowStart: m.MonthlyWindowStart,
		DailyUsageUSD:      m.DailyUsageUsd,
		WeeklyUsageUSD:     m.WeeklyUsageUsd,
		MonthlyUsageUSD:    m.MonthlyUsageUsd,
		DailyQuotaKnives:   m.DailyQuotaKnives,
		WeeklyQuotaKnives:  m.WeeklyQuotaKnives,
		MonthlyQuotaKnives: m.MonthlyQuotaKnives,
		DailyUsedKnives:    m.DailyUsedKnives,
		WeeklyUsedKnives:   m.WeeklyUsedKnives,
		MonthlyUsedKnives:  m.MonthlyUsedKnives,
		SupersededByID:     m.SupersededByID,
		AssignedBy:         m.AssignedBy,
		AssignedAt:         m.AssignedAt,
		Notes:              derefString(m.Notes),
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}
