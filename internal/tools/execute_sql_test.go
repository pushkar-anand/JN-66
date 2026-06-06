package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRows is a hand-rolled pgx.Rows implementation for testing.
type fakeRows struct {
	headers []string
	data    [][]any
	pos     int
	err     error
}

func newFakeRows(headers []string, data [][]any) *fakeRows {
	return &fakeRows{headers: headers, data: data, pos: -1}
}

func (r *fakeRows) Close()                          {}
func (r *fakeRows) Err() error                      { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag   { return pgconn.CommandTag{} }
func (r *fakeRows) RawValues() [][]byte             { return nil }
func (r *fakeRows) Scan(dest ...any) error          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                 { return nil }

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	descs := make([]pgconn.FieldDescription, len(r.headers))
	for i, h := range r.headers {
		descs[i] = pgconn.FieldDescription{Name: h}
	}
	return descs
}

func (r *fakeRows) Next() bool {
	r.pos++
	return r.pos < len(r.data)
}

func (r *fakeRows) Values() ([]any, error) {
	if r.pos < 0 || r.pos >= len(r.data) {
		return nil, nil
	}
	return r.data[r.pos], nil
}

// fakeRawQuerier satisfies rawQuerier for testing.
type fakeRawQuerier struct {
	rows pgx.Rows
	err  error
}

func (q *fakeRawQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return q.rows, q.err
}

func TestExecuteSQL_Definition(t *testing.T) {
	def := NewExecuteSQL(&fakeRawQuerier{}).Definition()
	assert.Equal(t, "execute_sql", def.Name)
}

func TestExecuteSQL_InvalidJSON(t *testing.T) {
	_, err := NewExecuteSQL(&fakeRawQuerier{}).Execute(t.Context(), "", `{bad`)
	require.Error(t, err)
}

func TestExecuteSQL_RejectsWriteQueries(t *testing.T) {
	cases := []string{
		`{"query":"UPDATE transactions SET amount=0"}`,
		`{"query":"DELETE FROM transactions"}`,
		`{"query":"INSERT INTO transactions VALUES ()"}`,
		`{"query":"DROP TABLE transactions"}`,
		`{"query":"; SELECT 1"}`,
		`{"query":"TRUNCATE transactions"}`,
	}
	tool := NewExecuteSQL(&fakeRawQuerier{})
	for _, c := range cases {
		_, err := tool.Execute(t.Context(), "", c)
		require.Error(t, err, "expected rejection for %s", c)
	}
}

func TestExecuteSQL_AcceptsSelectAndWith(t *testing.T) {
	rows := newFakeRows([]string{"n"}, [][]any{{"1"}})
	q := &fakeRawQuerier{rows: rows}
	cases := []string{
		`{"query":"SELECT 1"}`,
		`{"query":"select 1"}`,
		`{"query":"  SELECT 1"}`,
		`{"query":"WITH x AS (SELECT 1) SELECT * FROM x"}`,
		`{"query":"-- comment\nSELECT 1"}`,
	}
	for _, c := range cases {
		rows.pos = -1 // reset
		_, err := NewExecuteSQL(q).Execute(t.Context(), "", c)
		require.NoError(t, err, "expected acceptance for %s", c)
	}
}

func TestExecuteSQL_DBError(t *testing.T) {
	q := &fakeRawQuerier{err: errors.New("db down")}
	_, err := NewExecuteSQL(q).Execute(t.Context(), "", `{"query":"SELECT 1"}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestExecuteSQL_NoRows(t *testing.T) {
	rows := newFakeRows([]string{"id"}, nil)
	q := &fakeRawQuerier{rows: rows}
	got, err := NewExecuteSQL(q).Execute(t.Context(), "", `{"query":"SELECT id FROM transactions"}`)
	require.NoError(t, err)
	assert.Equal(t, "No rows returned.", got)
}

func TestExecuteSQL_FormatsTable(t *testing.T) {
	rows := newFakeRows(
		[]string{"name", "amount"},
		[][]any{
			{"Zomato", int64(45000)},
			{"Swiggy", int64(32000)},
		},
	)
	q := &fakeRawQuerier{rows: rows}
	got, err := NewExecuteSQL(q).Execute(t.Context(), "", `{"query":"SELECT name, amount FROM transactions"}`)
	require.NoError(t, err)
	assert.Contains(t, got, "name")
	assert.Contains(t, got, "amount")
	assert.Contains(t, got, "Zomato")
	assert.Contains(t, got, "45000")
}

func TestExecuteSQL_NullValueRendered(t *testing.T) {
	rows := newFakeRows([]string{"col"}, [][]any{{nil}})
	q := &fakeRawQuerier{rows: rows}
	got, err := NewExecuteSQL(q).Execute(t.Context(), "", `{"query":"SELECT NULL"}`)
	require.NoError(t, err)
	assert.Contains(t, got, "NULL")
}

func TestExecuteSQL_TruncatesAt50Rows(t *testing.T) {
	data := make([][]any, maxSQLRows+1)
	for i := range data {
		data[i] = []any{"xyz"}
	}
	rows := newFakeRows([]string{"val"}, data)
	q := &fakeRawQuerier{rows: rows}
	got, err := NewExecuteSQL(q).Execute(t.Context(), "", `{"query":"SELECT val FROM t"}`)
	require.NoError(t, err)
	assert.Contains(t, got, "showing first 50 rows")
	// Exactly 50 data lines + header + separator + truncation notice.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	dataLines := 0
	for _, l := range lines {
		if strings.Contains(l, "xyz") {
			dataLines++
		}
	}
	assert.Equal(t, maxSQLRows, dataLines)
}
