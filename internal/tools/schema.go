package tools

import (
	"context"
	"fmt"
	"strings"
)

// BuildSchemaString queries the live database and returns a formatted schema
// description for use in the get_schema tool. Called once at startup.
func BuildSchemaString(ctx context.Context, db rawQuerier) (string, error) {
	cols, err := queryColumns(ctx, db)
	if err != nil {
		return "", fmt.Errorf("query columns: %w", err)
	}
	enums, err := queryEnums(ctx, db)
	if err != nil {
		return "", fmt.Errorf("query enums: %w", err)
	}

	var b strings.Builder
	b.WriteString(`CONVENTIONS
  - Monetary amounts are stored as BIGINT in paise (INR × 100). Never floats.
  - Always filter by user_id unless intentionally querying across all users.
  - Enrichment data (category, transfer_id, notes, tagging_status) lives in
    transaction_enrichments joined to transactions; v_transactions is a view
    that combines both.
  - account_class is a GENERATED column derived from account_type — never set it manually.
  - idempotency_key on transactions is SHA256(account_id||txn_date||amount||description).

`)

	var curTable string
	for _, c := range cols {
		if c.table != curTable {
			if curTable != "" {
				b.WriteByte('\n')
			}
			b.WriteString("TABLE " + c.table + "\n")
			curTable = c.table
		}
		nullable := ""
		if c.nullable == "NO" {
			nullable = " NOT NULL"
		}
		generated := ""
		if c.generated != "" {
			generated = " GENERATED"
		}
		b.WriteString(fmt.Sprintf("  %-30s %s%s%s\n", c.column, c.typeName, nullable, generated))
	}

	if len(enums) > 0 {
		b.WriteByte('\n')
		for name, vals := range enums {
			b.WriteString("ENUM " + name + ": " + strings.Join(vals, ", ") + "\n")
		}
	}

	return b.String(), nil
}

type colRow struct {
	table     string
	column    string
	typeName  string
	nullable  string
	generated string
}

func queryColumns(ctx context.Context, db rawQuerier) ([]colRow, error) {
	const q = `
SELECT
    c.table_name,
    c.column_name,
    COALESCE(c.udt_name, c.data_type) AS type_name,
    c.is_nullable,
    COALESCE(c.is_generated, '') AS is_generated
FROM information_schema.columns c
WHERE c.table_schema = 'public'
ORDER BY c.table_name, c.ordinal_position`

	rows, err := db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []colRow
	for rows.Next() {
		var r colRow
		if err := rows.Scan(&r.table, &r.column, &r.typeName, &r.nullable, &r.generated); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func queryEnums(ctx context.Context, db rawQuerier) (map[string][]string, error) {
	const q = `
SELECT t.typname, e.enumlabel
FROM pg_type t
JOIN pg_enum e ON t.oid = e.enumtypid
ORDER BY t.typname, e.enumsortorder`

	rows, err := db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var name, label string
		if err := rows.Scan(&name, &label); err != nil {
			return nil, err
		}
		result[name] = append(result[name], label)
	}
	return result, rows.Err()
}
