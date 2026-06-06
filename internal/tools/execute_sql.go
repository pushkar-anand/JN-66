package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pushkaranand/finagent/internal/llm"
)

const maxSQLRows = 50

// ExecuteSQL runs a read-only SELECT query against the database via the
// finagent_ro role. Two-layer protection: syntactic check (must start with
// SELECT or WITH) plus the DB role itself which only has SELECT privileges.
type ExecuteSQL struct {
	db rawQuerier
}

// NewExecuteSQL creates an ExecuteSQL tool backed by a read-only connection pool.
func NewExecuteSQL(db rawQuerier) *ExecuteSQL {
	return &ExecuteSQL{db: db}
}

// Definition returns the tool's name, description, and parameter schema.
func (t *ExecuteSQL) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        "execute_sql",
		Description: "Executes a read-only SELECT query against the database and returns the results. Only SELECT and WITH (CTE) queries are allowed — any attempt to run INSERT, UPDATE, DELETE, DROP, or other write operations will be rejected. Call get_schema first to know the table and column names.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "A SQL SELECT or WITH query to execute. Must not contain any write operations.",
				},
			},
			"required": []string{"query"},
		},
	}
}

type executeSQLArgs struct {
	Query string `json:"query"`
}

// Execute validates and runs the query, returning results as a text table.
func (t *ExecuteSQL) Execute(ctx context.Context, _ string, argsJSON string) (string, error) {
	var args executeSQLArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	if err := validateSelectQuery(args.Query); err != nil {
		return "", err
	}

	rows, err := t.db.Query(ctx, args.Query)
	if err != nil {
		return "", fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	descs := rows.FieldDescriptions()
	headers := make([]string, len(descs))
	for i, d := range descs {
		headers[i] = string(d.Name)
	}

	var result [][]string
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return "", fmt.Errorf("scan row: %w", err)
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		result = append(result, row)
		if len(result) >= maxSQLRows+1 {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("read rows: %w", err)
	}

	truncated := len(result) > maxSQLRows
	if truncated {
		result = result[:maxSQLRows]
	}

	if len(result) == 0 {
		return "No rows returned.", nil
	}

	return formatTable(headers, result, truncated), nil
}

// validateSelectQuery rejects anything that doesn't start with SELECT or WITH.
func validateSelectQuery(query string) error {
	trimmed := strings.TrimSpace(query)
	// Strip leading line comments.
	for strings.HasPrefix(trimmed, "--") {
		rest := strings.IndexByte(trimmed, '\n')
		if rest < 0 {
			trimmed = ""
			break
		}
		trimmed = strings.TrimSpace(trimmed[rest+1:])
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return fmt.Errorf("only SELECT and WITH queries are allowed; got: %.40s", trimmed)
	}
	return nil
}

// formatTable renders headers and rows as a pipe-delimited text table.
func formatTable(headers []string, rows [][]string, truncated bool) string {
	// Compute column widths.
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	pad := func(s string, w int) string {
		if len(s) >= w {
			return s
		}
		return s + strings.Repeat(" ", w-len(s))
	}

	var b strings.Builder
	// Header row.
	for i, h := range headers {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(pad(h, widths[i]))
	}
	b.WriteByte('\n')
	// Separator.
	for i, w := range widths {
		if i > 0 {
			b.WriteString("-+-")
		}
		b.WriteString(strings.Repeat("-", w))
	}
	b.WriteByte('\n')
	// Data rows.
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(pad(cell, widths[i]))
		}
		b.WriteByte('\n')
	}

	if truncated {
		b.WriteString(fmt.Sprintf("\n(showing first %d rows — add a LIMIT clause to your query to reduce results)\n", maxSQLRows))
	}
	return b.String()
}
