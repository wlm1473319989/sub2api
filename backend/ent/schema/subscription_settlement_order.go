package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SubscriptionSettlementOrder holds the schema definition for the subscription settlement chain.
type SubscriptionSettlementOrder struct {
	ent.Schema
}

func (SubscriptionSettlementOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscription_settlement_orders"},
	}
}

func (SubscriptionSettlementOrder) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("prev_settlement_id").
			Optional().
			Nillable(),
		field.String("action_type").
			MaxLen(32),
		field.String("action_source").
			MaxLen(32),
		field.String("status").
			MaxLen(16).
			Default(domain.SettlementStatusEffective),
		field.String("trigger_ref_type").
			MaxLen(32),
		field.Int64("trigger_ref_id").
			Optional().
			Nillable(),
		field.Int64("operator_user_id"),
		field.String("action_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),

		field.Float("carry_in_residual_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("action_delta_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("after_settlement_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("refund_residual_value").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("writeoff_value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),

		field.Int64("after_user_subscription_id").
			Optional().
			Nillable(),
		field.Int64("after_plan_id").
			Optional().
			Nillable(),
		field.String("after_plan_name_snapshot").
			Optional().
			Nillable().
			MaxLen(100),
		field.Float("after_plan_price_snapshot").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int("after_validity_days_snapshot").
			Optional().
			Nillable(),
		field.String("after_validity_unit_snapshot").
			Optional().
			Nillable().
			MaxLen(16),
		field.Time("after_starts_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("after_expires_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Float("after_daily_quota_knives_snapshot").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("after_weekly_quota_knives_snapshot").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("after_monthly_quota_knives_snapshot").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.String("after_subscription_status").
			MaxLen(16),

		field.Time("effective_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("closed_at").
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

func (SubscriptionSettlementOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("subscription_settlement_orders").
			Field("user_id").
			Unique().
			Required(),
		edge.From("operator", User.Type).
			Ref("operated_subscription_settlement_orders").
			Field("operator_user_id").
			Unique().
			Required(),
		edge.To("next", SubscriptionSettlementOrder.Type).
			Unique(),
		edge.From("previous", SubscriptionSettlementOrder.Type).
			Ref("next").
			Field("prev_settlement_id").
			Unique(),
		edge.From("after_user_subscription", UserSubscription.Type).
			Ref("settlement_orders").
			Field("after_user_subscription_id").
			Unique(),
		edge.From("after_plan", SubscriptionPlan.Type).
			Ref("settlement_orders").
			Field("after_plan_id").
			Unique(),
	}
}

func (SubscriptionSettlementOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("action_type"),
		index.Fields("action_source"),
		index.Fields("trigger_ref_type", "trigger_ref_id"),
		index.Fields("after_user_subscription_id"),
		index.Fields("after_plan_id"),
		index.Fields("effective_at"),
		index.Fields("user_id").
			StorageKey("subscriptionsettlementorder_user_effective").
			Unique().
			Annotations(entsql.IndexWhere("status = 'effective'")),
	}
}
