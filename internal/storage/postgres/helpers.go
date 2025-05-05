package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier defines the methods needed from pgxpool.Pool or pgx.Tx
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// buildJobListQuery constructs the SQL query for listing jobs based on filters.
func (r *JobRepo) buildJobListQuery(baseQuery string, conditions []string, args *[]interface{}, reqOffset, reqLimit int) string {
	var queryBuilder strings.Builder
	queryBuilder.WriteString(baseQuery)

	if len(conditions) > 0 {
		queryBuilder.WriteString(" WHERE ")
		queryBuilder.WriteString(strings.Join(conditions, " AND "))
	}

	queryBuilder.WriteString(" ORDER BY created_at DESC") // Default ordering

	// Add LIMIT and OFFSET
	*args = append(*args, reqLimit)
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", len(*args)))
	*args = append(*args, reqOffset)
	queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", len(*args)))

	return queryBuilder.String()
}