package postgres

import (
	"fmt"
	"strings"
)

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