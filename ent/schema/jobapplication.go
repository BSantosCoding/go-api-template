package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// JobApplication holds the schema definition for the JobApplication entity.
type JobApplication struct {
	ent.Schema
}

// Fields of the JobApplication.
func (JobApplication) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("id").Immutable(),

		field.UUID("job_id", uuid.UUID{}).StorageKey("job_id").Immutable(),
		field.UUID("contractor_id", uuid.UUID{}).StorageKey("contractor_id").Immutable(),

		// Map JobApplicationState enum to Ent's enum field
		field.Enum("state").
			Values("Waiting", "Accepted", "Rejected", "Withdrawn").
			Default("Waiting"),

		field.Time("created_at").Immutable().Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (JobApplication) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "job_application"},
	}
}

// Edges of the JobApplication.
func (JobApplication) Edges() []ent.Edge {
	return []ent.Edge{
		// Application belongs to a contractor (User). Required edge.
		edge.From("contractor", User.Type).
			Ref("applicationsAsContractor").
			Required().
			Unique().
			Immutable().
			Field("contractor_id"),

		// Application is for a specific Job. Required edge.
		edge.From("job", Job.Type).
			Ref("applications").
			Required().
			Unique().
			Immutable().
			Field("job_id"),
	}
}
