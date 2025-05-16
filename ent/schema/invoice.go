package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Invoice holds the schema definition for the Invoice entity.
type Invoice struct {
	ent.Schema
}

// Fields of the Invoice.
func (Invoice) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("id").Immutable(),

		field.Float("value").Positive(), // Value should be non-negative

		// Map InvoiceState enum to Ent's enum field
		field.Enum("state").
			Values("Waiting", "Complete").
			Default("Waiting"),

		field.UUID("job_id", uuid.UUID{}).StorageKey("job_id").Immutable(),

		field.Int("interval_number"),

		field.Time("created_at").Immutable().Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Invoice.
func (Invoice) Edges() []ent.Edge {
	return []ent.Edge{
		// Invoice belongs to a Job. Required edge.
		edge.From("job", Job.Type).
			Ref("invoices").
			Required().
			Unique().
			Immutable().
			Field("job_id"), // Maps to the foreign key column
	}
}
