package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// Use field.UUID for the primary key, Ent handles the default generation if not provided
		field.UUID("id", uuid.UUID{}).Default(uuid.New).StorageKey("id").Immutable(),

		// Corresponds to Name string in models.User
		field.String("name").NotEmpty(),

		// Corresponds to Email string in models.User, add Unique constraint
		field.String("email").Unique().NotEmpty(),

		// Corresponds to PasswordHash string in models.User. Use Text for potentially long hashes.
		field.Text("password_hash").Sensitive().NotEmpty(), // Mark as Sensitive to prevent logging

		// Corresponds to CreatedAt time.Time. Immutable with default.
		field.Time("created_at").Immutable().Default(time.Now),

		// Corresponds to UpdatedAt time.Time. Default and update default.
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the User. Define relationships here.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// User can be an employer for multiple jobs.
		edge.To("jobsAsEmployer", Job.Type),

		// User can be a contractor for multiple jobs (optional).
		edge.To("jobsAsContractor", Job.Type),

		// User can submit multiple job applications.
		edge.To("applicationsAsContractor", JobApplication.Type),
	}
}
