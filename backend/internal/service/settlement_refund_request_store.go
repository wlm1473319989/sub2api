package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

const (
	SettlementRefundStatusPreviewed         = "previewed"
	SettlementRefundStatusExpired           = "expired"
	SettlementRefundStatusSubmitted         = "submitted"
	SettlementRefundStatusGatewayProcessing = "gateway_processing"
	SettlementRefundStatusManualPending     = "manual_pending"
	SettlementRefundStatusCompleted         = "completed"
	SettlementRefundStatusFailed            = "failed"
	SettlementRefundStatusCancelled         = "cancelled"

	SettlementRefundModeGatewayRefund   = "gateway_refund"
	SettlementRefundModeManualTransfer  = "manual_transfer"
	SettlementRefundModeHybrid          = "hybrid"
	SettlementRefundModeEntitlementOnly = "entitlement_only"

	SettlementRefundAllocationStatusPending    = "pending"
	SettlementRefundAllocationStatusProcessing = "processing"
	SettlementRefundAllocationStatusSucceeded  = "succeeded"
	SettlementRefundAllocationStatusFailed     = "failed"
	SettlementRefundAllocationStatusSkipped    = "skipped"
)

var (
	ErrSettlementRefundStoreRequired       = infraerrors.InternalServer("SETTLEMENT_REFUND_STORE_REQUIRED", "settlement refund store requires database access")
	ErrSettlementRefundPreviewInput        = infraerrors.BadRequest("SETTLEMENT_REFUND_PREVIEW_INPUT_INVALID", "settlement refund preview input is invalid")
	ErrSettlementRefundRequestNotFound     = infraerrors.NotFound("SETTLEMENT_REFUND_REQUEST_NOT_FOUND", "settlement refund request not found")
	ErrSettlementRefundPreviewWindow       = infraerrors.BadRequest("SETTLEMENT_REFUND_PREVIEW_WINDOW_INVALID", "settlement refund preview window is invalid")
	ErrSettlementRefundPreviewTokenHash    = infraerrors.BadRequest("SETTLEMENT_REFUND_PREVIEW_TOKEN_HASH_REQUIRED", "settlement refund preview token hash is required")
	ErrSettlementRefundPreviewMode         = infraerrors.BadRequest("SETTLEMENT_REFUND_MODE_REQUIRED", "settlement refund mode is required")
	ErrSettlementRefundAllocationInput     = infraerrors.BadRequest("SETTLEMENT_REFUND_ALLOCATION_INPUT_INVALID", "settlement refund allocation input is invalid")
	ErrSettlementRefundTransactionFailed   = infraerrors.InternalServer("SETTLEMENT_REFUND_TRANSACTION_FAILED", "settlement refund transaction failed")
	ErrSettlementRefundAlreadyPending      = infraerrors.Conflict("SETTLEMENT_REFUND_ALREADY_PENDING", "settlement refund request is already pending for this subscription")
	ErrSettlementRefundSubmitInput         = infraerrors.BadRequest("SETTLEMENT_REFUND_SUBMIT_INPUT_INVALID", "settlement refund submit input is invalid")
	ErrSettlementRefundPreviewState        = infraerrors.Conflict("SETTLEMENT_REFUND_PREVIEW_STATE_INVALID", "settlement refund preview is not in previewed state")
	ErrSettlementRefundSubmitConflict      = infraerrors.Conflict("SETTLEMENT_REFUND_SUBMIT_CONFLICT", "settlement refund submit state changed")
	ErrSettlementRefundAllocationConflict  = infraerrors.Conflict("SETTLEMENT_REFUND_ALLOCATION_STATE_INVALID", "settlement refund allocation state is invalid")
	ErrSettlementRefundManualProofInput    = infraerrors.BadRequest("SETTLEMENT_REFUND_MANUAL_PROOF_INPUT_INVALID", "settlement refund manual proof input is invalid")
	ErrSettlementRefundManualProofConflict = infraerrors.Conflict("SETTLEMENT_REFUND_MANUAL_PROOF_CONFLICT", "settlement refund manual proof state changed")
	ErrSettlementRefundCompleteInput       = infraerrors.BadRequest("SETTLEMENT_REFUND_COMPLETE_INPUT_INVALID", "settlement refund complete input is invalid")
	ErrSettlementRefundCompleteConflict    = infraerrors.Conflict("SETTLEMENT_REFUND_COMPLETE_CONFLICT", "settlement refund complete state changed")
	ErrSettlementRefundCancelInput         = infraerrors.BadRequest("SETTLEMENT_REFUND_CANCEL_INPUT_INVALID", "settlement refund cancel input is invalid")
	ErrSettlementRefundCancelConflict      = infraerrors.Conflict("SETTLEMENT_REFUND_CANCEL_CONFLICT", "settlement refund cancel state changed")
)

type settlementRefundRequestStore struct {
	entClient *dbent.Client
}

type CreateSettlementRefundPreviewInput struct {
	UserID                 int64
	SubscriptionID         int64
	SettlementID           int64
	ExpectedSettlementID   int64
	Status                 string
	RefundMode             string
	Currency               string
	Reason                 *string
	RefundResidualValue    float64
	GatewayRefundableTotal float64
	ManualTransferAmount   float64
	PreviewTokenHash       string
	PreviewFingerprint     string
	PreviewIssuedAt        time.Time
	PreviewExpiresAt       time.Time
	Allocations            []CreateSettlementRefundAllocationInput
}

type CreateSettlementRefundAllocationInput struct {
	PaymentOrderID            int64
	PaymentProviderInstanceID *int64
	OrderAmount               float64
	OrderPayAmount            float64
	AlreadyRefundedAmount     float64
	RefundableOrderAmount     float64
	AllocatedRefundValue      float64
	GatewayRefundAmount       float64
	Currency                  string
	Status                    string
	GatewayRefundTradeNo      *string
	FailedReason              *string
	ProcessedAt               *time.Time
}

type CreateSettlementRefundRequestInput struct {
	UserID                        int64
	SubscriptionID                int64
	SettlementID                  int64
	ExpectedSettlementID          int64
	Status                        string
	RefundMode                    string
	Currency                      string
	Reason                        *string
	RefundResidualValue           float64
	GatewayRefundableTotal        float64
	ManualTransferAmount          float64
	PreviewTokenHash              string
	PreviewFingerprint            string
	PreviewIssuedAt               time.Time
	PreviewExpiresAt              time.Time
	SubmittedAt                   time.Time
	FrozenAt                      time.Time
	OriginalSubscriptionStatus    string
	OriginalSubscriptionExpiresAt time.Time
	ManualReceiverType            *string
	ManualReceiverName            *string
	ManualReceiverAccount         *string
	ManualReceiverQRCodeImageURL  *string
	ManualReceiverRemark          *string
	Allocations                   []CreateSettlementRefundAllocationInput
}

type SubmitSettlementRefundPreviewInput struct {
	RequestID                     int64
	ExpectedStatus                string
	SubmittedAt                   time.Time
	PreviewNotExpiredAfter        time.Time
	FrozenAt                      time.Time
	OriginalSubscriptionStatus    string
	OriginalSubscriptionExpiresAt time.Time
	ManualReceiverType            *string
	ManualReceiverName            *string
	ManualReceiverAccount         *string
	ManualReceiverQRCodeImageURL  *string
	ManualReceiverRemark          *string
}

type UpdateSettlementRefundManualProofInput struct {
	RequestID      int64
	ExpectedStatus string
	Status         string
	ProofURL       string
	UploadedAt     time.Time
	OperatorUserID int64
	AdminNote      *string
}

type CompleteSettlementRefundRequestInput struct {
	RequestID      int64
	ExpectedStatus string
	CompletedAt    time.Time
}

type CancelSettlementRefundRequestInput struct {
	RequestID      int64
	ExpectedStatus string
	CancelledAt    time.Time
	AdminNote      *string
}

type UpdateSettlementRefundRequestStatusInput struct {
	RequestID      int64
	ExpectedStatus string
	Status         string
}

type UpdateSettlementRefundAllocationStatusInput struct {
	AllocationID         int64
	ExpectedStatus       string
	Status               string
	GatewayRefundTradeNo *string
	FailedReason         *string
	ProcessedAt          *time.Time
}

type SettlementRefundRequestRecord struct {
	ID                            int64
	UserID                        int64
	SubscriptionID                int64
	SettlementID                  int64
	ExpectedSettlementID          int64
	Status                        string
	RefundMode                    string
	Currency                      string
	Reason                        *string
	RefundResidualValue           float64
	GatewayRefundableTotal        float64
	ManualTransferAmount          float64
	PreviewTokenHash              string
	PreviewFingerprint            *string
	PreviewIssuedAt               time.Time
	PreviewExpiresAt              time.Time
	SubmittedAt                   *time.Time
	FrozenAt                      *time.Time
	CompletedAt                   *time.Time
	CancelledAt                   *time.Time
	OriginalSubscriptionStatus    *string
	OriginalSubscriptionExpiresAt *time.Time
	ManualReceiverType            *string
	ManualReceiverName            *string
	ManualReceiverAccount         *string
	ManualReceiverQRCodeImageURL  *string
	ManualReceiverRemark          *string
	ManualTransferProofURL        *string
	ManualTransferProofUploadedAt *time.Time
	ManualTransferOperatorUserID  *int64
	AdminNote                     *string
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	Allocations                   []SettlementRefundAllocationRecord
}

type SettlementRefundAllocationRecord struct {
	ID                        int64
	RefundRequestID           int64
	PaymentOrderID            int64
	PaymentProviderInstanceID *int64
	OrderAmount               float64
	OrderPayAmount            float64
	AlreadyRefundedAmount     float64
	RefundableOrderAmount     float64
	AllocatedRefundValue      float64
	GatewayRefundAmount       float64
	Currency                  string
	Status                    string
	GatewayRefundTradeNo      *string
	FailedReason              *string
	ProcessedAt               *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

func newSettlementRefundRequestStore(entClient *dbent.Client) *settlementRefundRequestStore {
	return &settlementRefundRequestStore{entClient: entClient}
}

func (s *settlementRefundRequestStore) CreateSettlementRefundPreview(ctx context.Context, input CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	if err := validateCreateSettlementRefundPreviewInput(input); err != nil {
		return nil, err
	}
	if input.Status == "" {
		input.Status = SettlementRefundStatusPreviewed
	}

	var record *SettlementRefundRequestRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		if expireErr := expireSettlementRefundPreviewedRequests(txCtx, client, input.SubscriptionID, input.PreviewIssuedAt); expireErr != nil {
			return expireErr
		}
		created, createErr := insertSettlementRefundRequest(txCtx, client, input)
		if createErr != nil {
			return createErr
		}
		created.Allocations = make([]SettlementRefundAllocationRecord, 0, len(input.Allocations))
		for _, allocationInput := range input.Allocations {
			allocation, allocationErr := insertSettlementRefundAllocation(txCtx, client, created.ID, allocationInput)
			if allocationErr != nil {
				return allocationErr
			}
			created.Allocations = append(created.Allocations, *allocation)
		}
		record = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) CreateSettlementRefundRequest(ctx context.Context, input CreateSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	if err := validateCreateSettlementRefundRequestInput(input); err != nil {
		return nil, err
	}
	if input.Status == "" {
		input.Status = SettlementRefundStatusSubmitted
	}

	var record *SettlementRefundRequestRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		created, createErr := insertSettlementRefundRequestSubmitted(txCtx, client, input)
		if createErr != nil {
			return createErr
		}
		created.Allocations = make([]SettlementRefundAllocationRecord, 0, len(input.Allocations))
		for _, allocationInput := range input.Allocations {
			allocation, allocationErr := insertSettlementRefundAllocation(txCtx, client, created.ID, allocationInput)
			if allocationErr != nil {
				return allocationErr
			}
			created.Allocations = append(created.Allocations, *allocation)
		}
		record = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) SubmitSettlementRefundPreview(ctx context.Context, input SubmitSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	if err := validateSubmitSettlementRefundPreviewInput(input); err != nil {
		return nil, err
	}

	var record *SettlementRefundRequestRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		updated, updateErr := updateSettlementRefundPreviewSubmitted(txCtx, client, input)
		if updateErr != nil {
			return updateErr
		}
		record = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) GetSettlementRefundRequest(ctx context.Context, id int64) (*SettlementRefundRequestRecord, error) {
	if id <= 0 {
		return nil, ErrSettlementRefundPreviewInput
	}
	client, err := s.settlementRefundClientFromContext(ctx)
	if err != nil {
		return nil, err
	}

	record, err := querySettlementRefundRequestByID(ctx, client, id)
	if err != nil {
		return nil, err
	}
	allocations, err := querySettlementRefundAllocations(ctx, client, id)
	if err != nil {
		return nil, err
	}
	record.Allocations = allocations
	return record, nil
}

func (s *settlementRefundRequestStore) UpdateSettlementRefundManualProof(ctx context.Context, input UpdateSettlementRefundManualProofInput) (*SettlementRefundRequestRecord, error) {
	if err := validateUpdateSettlementRefundManualProofInput(input); err != nil {
		return nil, err
	}

	var record *SettlementRefundRequestRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		updated, updateErr := updateSettlementRefundManualProof(txCtx, client, input)
		if updateErr != nil {
			return updateErr
		}
		record = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) CompleteSettlementRefundRequest(ctx context.Context, input CompleteSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	if err := validateCompleteSettlementRefundRequestInput(input); err != nil {
		return nil, err
	}

	var record *SettlementRefundRequestRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		updated, updateErr := updateSettlementRefundCompleted(txCtx, client, input)
		if updateErr != nil {
			return updateErr
		}
		record = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) CancelSettlementRefundRequest(ctx context.Context, input CancelSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	if err := validateCancelSettlementRefundRequestInput(input); err != nil {
		return nil, err
	}

	var record *SettlementRefundRequestRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		updated, updateErr := updateSettlementRefundCancelled(txCtx, client, input)
		if updateErr != nil {
			return updateErr
		}
		record = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) UpdateSettlementRefundRequestStatus(ctx context.Context, input UpdateSettlementRefundRequestStatusInput) (*SettlementRefundRequestRecord, error) {
	if err := validateUpdateSettlementRefundRequestStatusInput(input); err != nil {
		return nil, err
	}

	var record *SettlementRefundRequestRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		updated, updateErr := updateSettlementRefundRequestStatus(txCtx, client, input)
		if updateErr != nil {
			return updateErr
		}
		record = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) UpdateSettlementRefundAllocationStatus(ctx context.Context, input UpdateSettlementRefundAllocationStatusInput) (*SettlementRefundAllocationRecord, error) {
	if err := validateUpdateSettlementRefundAllocationStatusInput(input); err != nil {
		return nil, err
	}

	var record *SettlementRefundAllocationRecord
	err := s.withSettlementRefundTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		updated, updateErr := updateSettlementRefundAllocationStatus(txCtx, client, input)
		if updateErr != nil {
			return updateErr
		}
		record = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *settlementRefundRequestStore) settlementRefundClientFromContext(ctx context.Context) (*dbent.Client, error) {
	if s == nil || s.entClient == nil {
		return nil, ErrSettlementRefundStoreRequired
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client(), nil
	}
	return s.entClient, nil
}

func (s *settlementRefundRequestStore) withSettlementRefundTx(ctx context.Context, fn func(context.Context, *dbent.Client) error) error {
	if s == nil || s.entClient == nil {
		return ErrSettlementRefundStoreRequired
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return ErrSettlementRefundTransactionFailed.WithCause(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return ErrSettlementRefundTransactionFailed.WithCause(err)
	}
	committed = true
	return nil
}

func validateCreateSettlementRefundPreviewInput(input CreateSettlementRefundPreviewInput) error {
	if input.UserID <= 0 || input.SubscriptionID <= 0 || input.SettlementID <= 0 || input.ExpectedSettlementID <= 0 {
		return ErrSettlementRefundPreviewInput
	}
	if input.RefundMode == "" {
		return ErrSettlementRefundPreviewMode
	}
	if input.PreviewTokenHash == "" {
		return ErrSettlementRefundPreviewTokenHash
	}
	if strings.TrimSpace(input.PreviewFingerprint) == "" {
		return ErrSettlementRefundPreviewTokenHash
	}
	if input.PreviewIssuedAt.IsZero() || input.PreviewExpiresAt.IsZero() || !input.PreviewExpiresAt.After(input.PreviewIssuedAt) {
		return ErrSettlementRefundPreviewWindow
	}
	for _, allocation := range input.Allocations {
		if allocation.PaymentOrderID <= 0 {
			return ErrSettlementRefundAllocationInput
		}
	}
	return nil
}

func validateCreateSettlementRefundRequestInput(input CreateSettlementRefundRequestInput) error {
	if input.UserID <= 0 || input.SubscriptionID <= 0 || input.SettlementID <= 0 || input.ExpectedSettlementID <= 0 {
		return ErrSettlementRefundSubmitInput
	}
	if strings.TrimSpace(input.RefundMode) == "" {
		return ErrSettlementRefundPreviewMode
	}
	if strings.TrimSpace(input.PreviewTokenHash) == "" || strings.TrimSpace(input.PreviewFingerprint) == "" {
		return ErrSettlementRefundPreviewTokenHash
	}
	if input.PreviewIssuedAt.IsZero() || input.PreviewExpiresAt.IsZero() || !input.PreviewExpiresAt.After(input.PreviewIssuedAt) {
		return ErrSettlementRefundPreviewWindow
	}
	if input.SubmittedAt.IsZero() || input.FrozenAt.IsZero() {
		return ErrSettlementRefundSubmitInput
	}
	if strings.TrimSpace(input.OriginalSubscriptionStatus) == "" || input.OriginalSubscriptionExpiresAt.IsZero() {
		return ErrSettlementRefundSubmitInput
	}
	for _, allocation := range input.Allocations {
		if allocation.PaymentOrderID <= 0 {
			return ErrSettlementRefundAllocationInput
		}
	}
	return nil
}

func validateSubmitSettlementRefundPreviewInput(input SubmitSettlementRefundPreviewInput) error {
	if input.RequestID <= 0 {
		return ErrSettlementRefundSubmitInput
	}
	if strings.TrimSpace(input.ExpectedStatus) == "" {
		return ErrSettlementRefundSubmitInput
	}
	if input.SubmittedAt.IsZero() || input.FrozenAt.IsZero() {
		return ErrSettlementRefundSubmitInput
	}
	if input.PreviewNotExpiredAfter.IsZero() {
		return ErrSettlementRefundSubmitInput
	}
	if strings.TrimSpace(input.OriginalSubscriptionStatus) == "" {
		return ErrSettlementRefundSubmitInput
	}
	if input.OriginalSubscriptionExpiresAt.IsZero() {
		return ErrSettlementRefundSubmitInput
	}
	return nil
}

func validateUpdateSettlementRefundManualProofInput(input UpdateSettlementRefundManualProofInput) error {
	if input.RequestID <= 0 || input.OperatorUserID <= 0 {
		return ErrSettlementRefundManualProofInput
	}
	if strings.TrimSpace(input.ExpectedStatus) == "" || strings.TrimSpace(input.Status) == "" {
		return ErrSettlementRefundManualProofInput
	}
	if strings.TrimSpace(input.ProofURL) == "" || input.UploadedAt.IsZero() {
		return ErrSettlementRefundManualProofInput
	}
	return nil
}

func validateCompleteSettlementRefundRequestInput(input CompleteSettlementRefundRequestInput) error {
	if input.RequestID <= 0 || input.CompletedAt.IsZero() {
		return ErrSettlementRefundCompleteInput
	}
	if strings.TrimSpace(input.ExpectedStatus) == "" {
		return ErrSettlementRefundCompleteInput
	}
	return nil
}

func validateCancelSettlementRefundRequestInput(input CancelSettlementRefundRequestInput) error {
	if input.RequestID <= 0 || input.CancelledAt.IsZero() {
		return ErrSettlementRefundCancelInput
	}
	if strings.TrimSpace(input.ExpectedStatus) == "" {
		return ErrSettlementRefundCancelInput
	}
	return nil
}

func validateUpdateSettlementRefundRequestStatusInput(input UpdateSettlementRefundRequestStatusInput) error {
	if input.RequestID <= 0 {
		return ErrSettlementRefundSubmitInput
	}
	if strings.TrimSpace(input.ExpectedStatus) == "" || strings.TrimSpace(input.Status) == "" {
		return ErrSettlementRefundSubmitInput
	}
	return nil
}

func validateUpdateSettlementRefundAllocationStatusInput(input UpdateSettlementRefundAllocationStatusInput) error {
	if input.AllocationID <= 0 {
		return ErrSettlementRefundAllocationInput
	}
	if strings.TrimSpace(input.ExpectedStatus) == "" || strings.TrimSpace(input.Status) == "" {
		return ErrSettlementRefundAllocationInput
	}
	return nil
}

func insertSettlementRefundRequest(ctx context.Context, client *dbent.Client, input CreateSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	input.RefundResidualValue = roundSettlementRefundValue(input.RefundResidualValue)
	input.GatewayRefundableTotal = roundSettlementAmountValue(input.GatewayRefundableTotal)
	input.ManualTransferAmount = roundSettlementRefundValue(input.ManualTransferAmount)

	rows, err := client.QueryContext(ctx, `
INSERT INTO subscription_refund_requests (
    user_id,
    subscription_id,
    settlement_id,
    expected_settlement_id,
    status,
    refund_mode,
    currency,
    reason,
    refund_residual_value,
    gateway_refundable_total,
    manual_transfer_amount,
    preview_token_hash,
    preview_fingerprint,
    preview_issued_at,
    preview_expires_at,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW())
RETURNING id, created_at, updated_at`,
		input.UserID,
		input.SubscriptionID,
		input.SettlementID,
		input.ExpectedSettlementID,
		input.Status,
		input.RefundMode,
		input.Currency,
		nullableStringArg(input.Reason),
		input.RefundResidualValue,
		input.GatewayRefundableTotal,
		input.ManualTransferAmount,
		input.PreviewTokenHash,
		input.PreviewFingerprint,
		input.PreviewIssuedAt,
		input.PreviewExpiresAt,
	)
	if err != nil {
		if mapped := mapSettlementRefundRequestInsertError(err); mapped != nil {
			return nil, mapped
		}
		return nil, fmt.Errorf("insert settlement refund request: %w", err)
	}
	defer func() { _ = rows.Close() }()

	record := &SettlementRefundRequestRecord{
		UserID:                 input.UserID,
		SubscriptionID:         input.SubscriptionID,
		SettlementID:           input.SettlementID,
		ExpectedSettlementID:   input.ExpectedSettlementID,
		Status:                 input.Status,
		RefundMode:             input.RefundMode,
		Currency:               input.Currency,
		Reason:                 input.Reason,
		RefundResidualValue:    input.RefundResidualValue,
		GatewayRefundableTotal: input.GatewayRefundableTotal,
		ManualTransferAmount:   input.ManualTransferAmount,
		PreviewTokenHash:       input.PreviewTokenHash,
		PreviewFingerprint:     settlementRefundNullableReason(input.PreviewFingerprint),
		PreviewIssuedAt:        input.PreviewIssuedAt,
		PreviewExpiresAt:       input.PreviewExpiresAt,
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("insert settlement refund request result: %w", err)
		}
		return nil, fmt.Errorf("insert settlement refund request returned no rows")
	}
	if err := rows.Scan(&record.ID, &record.CreatedAt, &record.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan settlement refund request insert result: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund request insert result: %w", err)
	}
	return record, nil
}

func insertSettlementRefundRequestSubmitted(ctx context.Context, client *dbent.Client, input CreateSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	input.RefundResidualValue = roundSettlementRefundValue(input.RefundResidualValue)
	input.GatewayRefundableTotal = roundSettlementAmountValue(input.GatewayRefundableTotal)
	input.ManualTransferAmount = roundSettlementRefundValue(input.ManualTransferAmount)

	rows, err := client.QueryContext(ctx, `
INSERT INTO subscription_refund_requests (
    user_id,
    subscription_id,
    settlement_id,
    expected_settlement_id,
    status,
    refund_mode,
    currency,
    reason,
    refund_residual_value,
    gateway_refundable_total,
    manual_transfer_amount,
    preview_token_hash,
    preview_fingerprint,
    preview_issued_at,
    preview_expires_at,
    submitted_at,
    frozen_at,
    original_subscription_status,
    original_subscription_expires_at,
    manual_receiver_type,
    manual_receiver_name,
    manual_receiver_account,
    manual_receiver_qr_image_url,
    manual_receiver_remark,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, NOW(), NOW())
RETURNING id, created_at, updated_at`,
		input.UserID,
		input.SubscriptionID,
		input.SettlementID,
		input.ExpectedSettlementID,
		input.Status,
		input.RefundMode,
		input.Currency,
		nullableStringArg(input.Reason),
		input.RefundResidualValue,
		input.GatewayRefundableTotal,
		input.ManualTransferAmount,
		input.PreviewTokenHash,
		input.PreviewFingerprint,
		input.PreviewIssuedAt,
		input.PreviewExpiresAt,
		input.SubmittedAt,
		input.FrozenAt,
		input.OriginalSubscriptionStatus,
		input.OriginalSubscriptionExpiresAt,
		nullableStringArg(input.ManualReceiverType),
		nullableStringArg(input.ManualReceiverName),
		nullableStringArg(input.ManualReceiverAccount),
		nullableStringArg(input.ManualReceiverQRCodeImageURL),
		nullableStringArg(input.ManualReceiverRemark),
	)
	if err != nil {
		if mapped := mapSettlementRefundRequestInsertError(err); mapped != nil {
			return nil, mapped
		}
		return nil, fmt.Errorf("insert settlement refund request submitted: %w", err)
	}
	defer func() { _ = rows.Close() }()

	record := &SettlementRefundRequestRecord{
		UserID:                        input.UserID,
		SubscriptionID:                input.SubscriptionID,
		SettlementID:                  input.SettlementID,
		ExpectedSettlementID:          input.ExpectedSettlementID,
		Status:                        input.Status,
		RefundMode:                    input.RefundMode,
		Currency:                      input.Currency,
		Reason:                        input.Reason,
		RefundResidualValue:           input.RefundResidualValue,
		GatewayRefundableTotal:        input.GatewayRefundableTotal,
		ManualTransferAmount:          input.ManualTransferAmount,
		PreviewTokenHash:              input.PreviewTokenHash,
		PreviewFingerprint:            settlementRefundNullableReason(input.PreviewFingerprint),
		PreviewIssuedAt:               input.PreviewIssuedAt,
		PreviewExpiresAt:              input.PreviewExpiresAt,
		SubmittedAt:                   &input.SubmittedAt,
		FrozenAt:                      &input.FrozenAt,
		OriginalSubscriptionStatus:    settlementRefundNullableReason(input.OriginalSubscriptionStatus),
		OriginalSubscriptionExpiresAt: &input.OriginalSubscriptionExpiresAt,
		ManualReceiverType:            input.ManualReceiverType,
		ManualReceiverName:            input.ManualReceiverName,
		ManualReceiverAccount:         input.ManualReceiverAccount,
		ManualReceiverQRCodeImageURL:  input.ManualReceiverQRCodeImageURL,
		ManualReceiverRemark:          input.ManualReceiverRemark,
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("insert settlement refund request submitted result: %w", err)
		}
		return nil, fmt.Errorf("insert settlement refund request submitted returned no rows")
	}
	if err := rows.Scan(&record.ID, &record.CreatedAt, &record.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan settlement refund request submitted insert result: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund request submitted result: %w", err)
	}
	return record, nil
}

func mapSettlementRefundRequestInsertError(err error) error {
	if err == nil {
		return nil
	}
	pqErr, ok := err.(*pq.Error)
	if !ok {
		return nil
	}
	if pqErr.Code != "23505" {
		return nil
	}
	constraint := strings.TrimSpace(pqErr.Constraint)
	switch constraint {
	case "idx_subscription_refund_requests_subscription_processing", "subscriptionrefundrequest_subscription_processing":
		return ErrSettlementRefundAlreadyPending.WithCause(err)
	default:
		return nil
	}
}

func insertSettlementRefundAllocation(ctx context.Context, client *dbent.Client, requestID int64, input CreateSettlementRefundAllocationInput) (*SettlementRefundAllocationRecord, error) {
	input.OrderAmount = roundSettlementAmountValue(input.OrderAmount)
	input.OrderPayAmount = roundSettlementAmountValue(input.OrderPayAmount)
	input.AlreadyRefundedAmount = roundSettlementAmountValue(input.AlreadyRefundedAmount)
	input.RefundableOrderAmount = roundSettlementAmountValue(input.RefundableOrderAmount)
	input.AllocatedRefundValue = roundSettlementRefundValue(input.AllocatedRefundValue)
	input.GatewayRefundAmount = roundSettlementAmountValue(input.GatewayRefundAmount)

	if input.Status == "" {
		input.Status = SettlementRefundAllocationStatusPending
		if input.GatewayRefundAmount <= 0 {
			input.Status = SettlementRefundAllocationStatusSkipped
		}
	}
	rows, err := client.QueryContext(ctx, `
INSERT INTO subscription_refund_allocations (
    refund_request_id,
    payment_order_id,
    payment_provider_instance_id,
    order_amount,
    order_pay_amount,
    already_refunded_amount,
    refundable_order_amount,
    allocated_refund_value,
    gateway_refund_amount,
    currency,
    status,
    gateway_refund_trade_no,
    failed_reason,
    processed_at,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), $11, $12, $13, $14, NOW(), NOW())
RETURNING id, created_at, updated_at`,
		requestID,
		input.PaymentOrderID,
		nullableInt64Arg(input.PaymentProviderInstanceID),
		input.OrderAmount,
		input.OrderPayAmount,
		input.AlreadyRefundedAmount,
		input.RefundableOrderAmount,
		input.AllocatedRefundValue,
		input.GatewayRefundAmount,
		input.Currency,
		input.Status,
		nullableStringArg(input.GatewayRefundTradeNo),
		nullableStringArg(input.FailedReason),
		nullableTimeArg(input.ProcessedAt),
	)
	if err != nil {
		return nil, fmt.Errorf("insert settlement refund allocation: %w", err)
	}
	defer func() { _ = rows.Close() }()

	record := &SettlementRefundAllocationRecord{
		RefundRequestID:           requestID,
		PaymentOrderID:            input.PaymentOrderID,
		PaymentProviderInstanceID: input.PaymentProviderInstanceID,
		OrderAmount:               input.OrderAmount,
		OrderPayAmount:            input.OrderPayAmount,
		AlreadyRefundedAmount:     input.AlreadyRefundedAmount,
		RefundableOrderAmount:     input.RefundableOrderAmount,
		AllocatedRefundValue:      input.AllocatedRefundValue,
		GatewayRefundAmount:       input.GatewayRefundAmount,
		Currency:                  input.Currency,
		Status:                    input.Status,
		GatewayRefundTradeNo:      input.GatewayRefundTradeNo,
		FailedReason:              input.FailedReason,
		ProcessedAt:               input.ProcessedAt,
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("insert settlement refund allocation result: %w", err)
		}
		return nil, fmt.Errorf("insert settlement refund allocation returned no rows")
	}
	if err := rows.Scan(&record.ID, &record.CreatedAt, &record.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan settlement refund allocation insert result: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund allocation insert result: %w", err)
	}
	return record, nil
}

func expireSettlementRefundPreviewedRequests(ctx context.Context, client *dbent.Client, subscriptionID int64, updatedAt time.Time) error {
	if subscriptionID <= 0 {
		return ErrSettlementRefundPreviewInput
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	_, err := client.ExecContext(ctx, `
UPDATE subscription_refund_requests
SET
    status = $1,
    updated_at = $2
WHERE subscription_id = $3 AND status = $4`,
		SettlementRefundStatusExpired,
		updatedAt,
		subscriptionID,
		SettlementRefundStatusPreviewed,
	)
	if err != nil {
		return fmt.Errorf("expire settlement refund previews: %w", err)
	}
	return nil
}

func updateSettlementRefundPreviewSubmitted(ctx context.Context, client *dbent.Client, input SubmitSettlementRefundPreviewInput) (*SettlementRefundRequestRecord, error) {
	rows, err := client.QueryContext(ctx, `
UPDATE subscription_refund_requests
SET
    status = $1,
    submitted_at = $2,
    frozen_at = $3,
    original_subscription_status = $4,
    original_subscription_expires_at = $5,
    manual_receiver_type = $6,
    manual_receiver_name = $7,
    manual_receiver_account = $8,
    manual_receiver_qr_image_url = $9,
    manual_receiver_remark = $10,
    updated_at = NOW()
WHERE id = $11 AND status = $12 AND preview_expires_at >= $13
RETURNING
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
    updated_at`,
		SettlementRefundStatusSubmitted,
		input.SubmittedAt,
		input.FrozenAt,
		input.OriginalSubscriptionStatus,
		input.OriginalSubscriptionExpiresAt,
		nullableStringArg(input.ManualReceiverType),
		nullableStringArg(input.ManualReceiverName),
		nullableStringArg(input.ManualReceiverAccount),
		nullableStringArg(input.ManualReceiverQRCodeImageURL),
		nullableStringArg(input.ManualReceiverRemark),
		input.RequestID,
		input.ExpectedStatus,
		input.PreviewNotExpiredAfter,
	)
	if err != nil {
		return nil, fmt.Errorf("update settlement refund preview submitted: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("update settlement refund preview submitted result: %w", err)
		}
		return nil, ErrSettlementRefundSubmitConflict
	}
	record, err := scanSettlementRefundRequest(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund preview submitted result: %w", err)
	}
	return record, nil
}

func updateSettlementRefundManualProof(ctx context.Context, client *dbent.Client, input UpdateSettlementRefundManualProofInput) (*SettlementRefundRequestRecord, error) {
	rows, err := client.QueryContext(ctx, `
UPDATE subscription_refund_requests
SET
    status = $1,
    manual_transfer_proof_url = $2,
    manual_transfer_proof_uploaded_at = $3,
    manual_transfer_operator_user_id = $4,
    admin_note = $5,
    updated_at = NOW()
WHERE id = $6 AND status = $7
RETURNING
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
    updated_at`,
		input.Status,
		input.ProofURL,
		input.UploadedAt,
		input.OperatorUserID,
		nullableStringArg(input.AdminNote),
		input.RequestID,
		input.ExpectedStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("update settlement refund manual proof: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("update settlement refund manual proof result: %w", err)
		}
		return nil, ErrSettlementRefundManualProofConflict
	}
	record, err := scanSettlementRefundRequest(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund manual proof result: %w", err)
	}
	return record, nil
}

func updateSettlementRefundCompleted(ctx context.Context, client *dbent.Client, input CompleteSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	rows, err := client.QueryContext(ctx, `
UPDATE subscription_refund_requests
SET
    status = $1,
    completed_at = $2,
    updated_at = NOW()
WHERE id = $3 AND status = $4
RETURNING
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
    updated_at`,
		SettlementRefundStatusCompleted,
		input.CompletedAt,
		input.RequestID,
		input.ExpectedStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("update settlement refund completed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("update settlement refund completed result: %w", err)
		}
		return nil, ErrSettlementRefundCompleteConflict
	}
	record, err := scanSettlementRefundRequest(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund completed result: %w", err)
	}
	return record, nil
}

func updateSettlementRefundCancelled(ctx context.Context, client *dbent.Client, input CancelSettlementRefundRequestInput) (*SettlementRefundRequestRecord, error) {
	rows, err := client.QueryContext(ctx, `
UPDATE subscription_refund_requests
SET
    status = $1,
    cancelled_at = $2,
    admin_note = $3,
    updated_at = NOW()
WHERE id = $4 AND status = $5
RETURNING
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
    updated_at`,
		SettlementRefundStatusCancelled,
		input.CancelledAt,
		nullableStringArg(input.AdminNote),
		input.RequestID,
		input.ExpectedStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("update settlement refund cancelled: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("update settlement refund cancelled result: %w", err)
		}
		return nil, ErrSettlementRefundCancelConflict
	}
	record, err := scanSettlementRefundRequest(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund cancelled result: %w", err)
	}
	return record, nil
}

func updateSettlementRefundRequestStatus(ctx context.Context, client *dbent.Client, input UpdateSettlementRefundRequestStatusInput) (*SettlementRefundRequestRecord, error) {
	rows, err := client.QueryContext(ctx, `
UPDATE subscription_refund_requests
SET
    status = $1,
    updated_at = NOW()
WHERE id = $2 AND status = $3
RETURNING
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
    updated_at`,
		input.Status,
		input.RequestID,
		input.ExpectedStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("update settlement refund request status: %w", err)
	}
	rowsClosed := false
	defer func() {
		if !rowsClosed {
			_ = rows.Close()
		}
	}()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("update settlement refund request status result: %w", err)
		}
		return nil, ErrSettlementRefundSubmitConflict
	}
	record, err := scanSettlementRefundRequest(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund request status result: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close settlement refund request status result: %w", err)
	}
	rowsClosed = true

	allocations, err := querySettlementRefundAllocations(ctx, client, input.RequestID)
	if err != nil {
		return nil, err
	}
	record.Allocations = allocations
	return record, nil
}

func updateSettlementRefundAllocationStatus(ctx context.Context, client *dbent.Client, input UpdateSettlementRefundAllocationStatusInput) (*SettlementRefundAllocationRecord, error) {
	rows, err := client.QueryContext(ctx, `
UPDATE subscription_refund_allocations
SET
    status = $1,
    gateway_refund_trade_no = $2,
    failed_reason = $3,
    processed_at = $4,
    updated_at = NOW()
WHERE id = $5 AND status = $6
RETURNING
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
    updated_at`,
		input.Status,
		nullableStringArg(input.GatewayRefundTradeNo),
		nullableStringArg(input.FailedReason),
		nullableTimeArg(input.ProcessedAt),
		input.AllocationID,
		input.ExpectedStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("update settlement refund allocation status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("update settlement refund allocation status result: %w", err)
		}
		return nil, ErrSettlementRefundAllocationConflict
	}
	record, err := scanSettlementRefundAllocation(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund allocation status result: %w", err)
	}
	return record, nil
}

func querySettlementRefundRequestByID(ctx context.Context, client *dbent.Client, id int64) (*SettlementRefundRequestRecord, error) {
	rows, err := client.QueryContext(ctx, `
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
WHERE id = $1
LIMIT 1`, id)
	if err != nil {
		return nil, fmt.Errorf("query settlement refund request: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query settlement refund request result: %w", err)
		}
		return nil, ErrSettlementRefundRequestNotFound
	}
	record, err := scanSettlementRefundRequest(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund request: %w", err)
	}
	return record, nil
}

func querySettlementRefundAllocations(ctx context.Context, client *dbent.Client, requestID int64) ([]SettlementRefundAllocationRecord, error) {
	rows, err := client.QueryContext(ctx, `
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
WHERE refund_request_id = $1
ORDER BY id ASC`, requestID)
	if err != nil {
		return nil, fmt.Errorf("query settlement refund allocations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	allocations := make([]SettlementRefundAllocationRecord, 0)
	for rows.Next() {
		allocation, scanErr := scanSettlementRefundAllocation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		allocations = append(allocations, *allocation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settlement refund allocations: %w", err)
	}
	return allocations, nil
}

func scanSettlementRefundRequest(scanner interface{ Scan(dest ...any) error }) (*SettlementRefundRequestRecord, error) {
	var record SettlementRefundRequestRecord
	var reason sql.NullString
	var previewFingerprint sql.NullString
	var submittedAt sql.NullTime
	var frozenAt sql.NullTime
	var completedAt sql.NullTime
	var cancelledAt sql.NullTime
	var originalStatus sql.NullString
	var originalExpiresAt sql.NullTime
	var manualReceiverType sql.NullString
	var manualReceiverName sql.NullString
	var manualReceiverAccount sql.NullString
	var manualReceiverQRCodeImageURL sql.NullString
	var manualReceiverRemark sql.NullString
	var manualTransferProofURL sql.NullString
	var manualTransferProofUploadedAt sql.NullTime
	var manualTransferOperatorUserID sql.NullInt64
	var adminNote sql.NullString

	if err := scanner.Scan(
		&record.ID,
		&record.UserID,
		&record.SubscriptionID,
		&record.SettlementID,
		&record.ExpectedSettlementID,
		&record.Status,
		&record.RefundMode,
		&record.Currency,
		&reason,
		&record.RefundResidualValue,
		&record.GatewayRefundableTotal,
		&record.ManualTransferAmount,
		&record.PreviewTokenHash,
		&previewFingerprint,
		&record.PreviewIssuedAt,
		&record.PreviewExpiresAt,
		&submittedAt,
		&frozenAt,
		&completedAt,
		&cancelledAt,
		&originalStatus,
		&originalExpiresAt,
		&manualReceiverType,
		&manualReceiverName,
		&manualReceiverAccount,
		&manualReceiverQRCodeImageURL,
		&manualReceiverRemark,
		&manualTransferProofURL,
		&manualTransferProofUploadedAt,
		&manualTransferOperatorUserID,
		&adminNote,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan settlement refund request: %w", err)
	}

	record.Reason = nullStringPtr(reason)
	record.PreviewFingerprint = nullStringPtr(previewFingerprint)
	record.RefundResidualValue = roundSettlementRefundValue(record.RefundResidualValue)
	record.GatewayRefundableTotal = roundSettlementAmountValue(record.GatewayRefundableTotal)
	record.ManualTransferAmount = roundSettlementRefundValue(record.ManualTransferAmount)
	record.SubmittedAt = nullTimePtr(submittedAt)
	record.FrozenAt = nullTimePtr(frozenAt)
	record.CompletedAt = nullTimePtr(completedAt)
	record.CancelledAt = nullTimePtr(cancelledAt)
	record.OriginalSubscriptionStatus = nullStringPtr(originalStatus)
	record.OriginalSubscriptionExpiresAt = nullTimePtr(originalExpiresAt)
	record.ManualReceiverType = nullStringPtr(manualReceiverType)
	record.ManualReceiverName = nullStringPtr(manualReceiverName)
	record.ManualReceiverAccount = nullStringPtr(manualReceiverAccount)
	record.ManualReceiverQRCodeImageURL = nullStringPtr(manualReceiverQRCodeImageURL)
	record.ManualReceiverRemark = nullStringPtr(manualReceiverRemark)
	record.ManualTransferProofURL = nullStringPtr(manualTransferProofURL)
	record.ManualTransferProofUploadedAt = nullTimePtr(manualTransferProofUploadedAt)
	record.ManualTransferOperatorUserID = nullInt64Ptr(manualTransferOperatorUserID)
	record.AdminNote = nullStringPtr(adminNote)
	return &record, nil
}

func scanSettlementRefundAllocation(scanner interface{ Scan(dest ...any) error }) (*SettlementRefundAllocationRecord, error) {
	var record SettlementRefundAllocationRecord
	var providerID sql.NullInt64
	var gatewayTradeNo sql.NullString
	var failedReason sql.NullString
	var processedAt sql.NullTime

	if err := scanner.Scan(
		&record.ID,
		&record.RefundRequestID,
		&record.PaymentOrderID,
		&providerID,
		&record.OrderAmount,
		&record.OrderPayAmount,
		&record.AlreadyRefundedAmount,
		&record.RefundableOrderAmount,
		&record.AllocatedRefundValue,
		&record.GatewayRefundAmount,
		&record.Currency,
		&record.Status,
		&gatewayTradeNo,
		&failedReason,
		&processedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan settlement refund allocation: %w", err)
	}

	record.PaymentProviderInstanceID = nullInt64Ptr(providerID)
	record.OrderAmount = roundSettlementAmountValue(record.OrderAmount)
	record.OrderPayAmount = roundSettlementAmountValue(record.OrderPayAmount)
	record.AlreadyRefundedAmount = roundSettlementAmountValue(record.AlreadyRefundedAmount)
	record.RefundableOrderAmount = roundSettlementAmountValue(record.RefundableOrderAmount)
	record.AllocatedRefundValue = roundSettlementRefundValue(record.AllocatedRefundValue)
	record.GatewayRefundAmount = roundSettlementAmountValue(record.GatewayRefundAmount)
	record.GatewayRefundTradeNo = nullStringPtr(gatewayTradeNo)
	record.FailedReason = nullStringPtr(failedReason)
	record.ProcessedAt = nullTimePtr(processedAt)
	return &record, nil
}

func nullableStringArg(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTimeArg(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt64Arg(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}
