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

// RechargeBonusRule defines one non-stackable balance recharge promotion.
type RechargeBonusRule struct {
	ent.Schema
}

func (RechargeBonusRule) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "recharge_bonus_rules"}}
}

func (RechargeBonusRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).NotEmpty(),
		field.Bool("enabled").Default(true),
		field.Int("priority").Default(0),
		field.Float("min_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.Float("max_amount").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.String("bonus_type").MaxLen(20),
		field.Float("bonus_value").SchemaType(map[string]string{dialect.Postgres: "decimal(20,4)"}),
		field.Time("starts_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("ends_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("notes").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (RechargeBonusRule) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("payment_orders", PaymentOrder.Type),
	}
}

func (RechargeBonusRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "priority"),
		index.Fields("starts_at", "ends_at"),
	}
}
