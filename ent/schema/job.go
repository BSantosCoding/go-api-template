package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Job holds the schema definition for the Job entity.
type Job struct {
	ent.Schema
}

// Fields of the Job.
func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("id").Immutable(),

		field.Float("rate").Positive(), // Rate should be non-negative
		field.Int("duration"),          // Duration in hours

		field.UUID("employer_id", uuid.UUID{}).StorageKey("employer_id").Immutable(),
		field.UUID("contractor_id", uuid.UUID{}).StorageKey("contractor_id").Optional(),

		// Map JobState enum to Ent's enum field
		field.Enum("state").
			Values("Waiting", "Ongoing", "Complete", "Archived").
			Default("Waiting"),

		field.Int("invoice_interval"), // In hours

		field.Time("created_at").Immutable().Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Job.
func (Job) Edges() []ent.Edge {
	return []ent.Edge{
		// Job belongs to an employer (User). Required edge.
		edge.From("employer", User.Type).
			Ref("jobsAsEmployer").
			Required().
			Unique().
			Immutable().
			Field("employer_id"),

		// Job may have a contractor (User). Optional edge.
		edge.From("contractor", User.Type).
			Ref("jobsAsContractor").
			Unique().
			Field("contractor_id"),

		// Job has multiple invoices.
		edge.To("invoices", Invoice.Type).Annotations(entsql.OnDelete(entsql.Cascade)), // Corrected line
		// Job has multiple applications.
		edge.To("applications", JobApplication.Type).Annotations(entsql.OnDelete(entsql.Cascade)), // Corrected line
	}
}
