package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SubscriptionRefundRequest holds the staged settlement refund request state.
type SubscriptionRefundRequest struct {
	ent.Schema
}

func (SubscriptionRefundRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscription_refund_requests"},
	}
}

func (SubscriptionRefundRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("subscription_id"),
		field.Int64("settlement_id"),
		field.Int64("expected_settlement_id"),

		field.String("status").
			MaxLen(32).
			Default("previewed"),
		field.String("refund_mode").
			MaxLen(32),
		field.String("currency").
			Optional().
			Nillable().
			MaxLen(10),
		field.String("reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),

		field.Float("refund_residual_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("gateway_refundable_total").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("manual_transfer_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),

		field.String("preview_token_hash").
			MaxLen(128),
		field.Time("preview_issued_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("preview_expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("submitted_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("frozen_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("completed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("cancelled_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.String("original_subscription_status").
			Optional().
			Nillable().
			MaxLen(20),
		field.Time("original_subscription_expires_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.String("manual_receiver_type").
			Optional().
			Nillable().
			MaxLen(32),
		field.String("manual_receiver_name").
			Optional().
			Nillable().
			MaxLen(100),
		field.String("manual_receiver_account").
			Optional().
			Nillable().
			MaxLen(255),
		field.String("manual_receiver_qr_image_url").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("manual_transfer_proof_url").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("manual_transfer_proof_uploaded_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("manual_transfer_operator_user_id").
			Optional().
			Nillable(),

		field.String("admin_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SubscriptionRefundRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("subscription_refund_requests").
			Field("user_id").
			Unique().
			Required(),
		edge.From("subscription", UserSubscription.Type).
			Ref("refund_requests").
			Field("subscription_id").
			Unique().
			Required(),
		edge.From("settlement", SubscriptionSettlementOrder.Type).
			Ref("refund_requests").
			Field("settlement_id").
			Unique().
			Required(),
		edge.From("expected_settlement", SubscriptionSettlementOrder.Type).
			Ref("expected_refund_requests").
			Field("expected_settlement_id").
			Unique().
			Required(),
		edge.From("manual_transfer_operator", User.Type).
			Ref("operated_subscription_refund_requests").
			Field("manual_transfer_operator_user_id").
			Unique(),
		edge.To("allocations", SubscriptionRefundAllocation.Type),
	}
}

func (SubscriptionRefundRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("subscription_id"),
		index.Fields("settlement_id"),
		index.Fields("status"),
		index.Fields("preview_expires_at"),
		index.Fields("subscription_id").
			StorageKey("subscriptionrefundrequest_subscription_open").
			Unique().
			Annotations(entsql.IndexWhere("status IN ('previewed', 'submitted', 'gateway_processing', 'manual_pending', 'failed')")),
	}
}
