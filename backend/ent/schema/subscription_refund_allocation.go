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

// SubscriptionRefundAllocation stores one payment order refund allocation.
type SubscriptionRefundAllocation struct {
	ent.Schema
}

func (SubscriptionRefundAllocation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscription_refund_allocations"},
	}
}

func (SubscriptionRefundAllocation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("refund_request_id"),
		field.Int64("payment_order_id"),
		field.Int64("payment_provider_instance_id").
			Optional().
			Nillable(),

		field.Float("order_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("order_pay_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("already_refunded_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("refundable_order_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("allocated_refund_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("gateway_refund_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.String("currency").
			Optional().
			Nillable().
			MaxLen(10),

		field.String("status").
			MaxLen(32).
			Default("pending"),
		field.String("gateway_refund_trade_no").
			Optional().
			Nillable().
			MaxLen(128),
		field.String("failed_reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("processed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
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

func (SubscriptionRefundAllocation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("refund_request", SubscriptionRefundRequest.Type).
			Ref("allocations").
			Field("refund_request_id").
			Unique().
			Required(),
		edge.From("payment_order", PaymentOrder.Type).
			Ref("subscription_refund_allocations").
			Field("payment_order_id").
			Unique().
			Required(),
		edge.From("payment_provider_instance", PaymentProviderInstance.Type).
			Ref("subscription_refund_allocations").
			Field("payment_provider_instance_id").
			Unique(),
	}
}

func (SubscriptionRefundAllocation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("refund_request_id", "payment_order_id").Unique(),
		index.Fields("refund_request_id"),
		index.Fields("payment_order_id"),
		index.Fields("status"),
	}
}
