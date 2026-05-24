# Contributing to JN-66

## Prerequisites

Install the following tools before starting:

| Tool | Install |
|------|---------|
| Go 1.26+ | https://go.dev/dl/ |
| Docker | https://docs.docker.com/get-docker/ |
| sqlc | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |
| golang-migrate | `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` |
| mockgen | `go install go.uber.org/mock/mockgen@latest` |
| goreturns | `go install github.com/sqs/goreturns@latest` |

## Dev setup

```bash
git clone https://github.com/pushkaranand/finagent.git
cd finagent

# Copy and edit config — set llm.base_url, users, api_keys at minimum
cp config.yaml.example config.yaml

# Start PostgreSQL 18 + pgvector
docker compose up -d

# Run all migrations
make migrate-up

# (Optional) seed test data: Alice + Bob, 3 accounts, ~40 transactions
make seed

# Generate sqlc types from schema + queries
make generate

# Build and run
make build
./bin/finagent --user alice
```

## Project structure

| Path | What lives here |
|------|----------------|
| `cmd/finagent/` | CLI entry point; subcommand dispatch (`import`, `account`, `user`, `enrich`, `zerodha`) |
| `config/` | koanf config loading and slog setup |
| `internal/agent/` | ReAct loop, system prompt, model routing |
| `internal/api/` | HTTP server and endpoint handlers |
| `internal/channel/` | Transport abstractions (CLI; future Slack/Signal) |
| `internal/llm/` | LLM provider interface and OpenAI-compatible client |
| `internal/tools/` | Agent tool implementations and registry |
| `internal/store/` | Typed data access wrappers over sqlc-generated code |
| `internal/importer/` | Bank statement parsers and LLM enrichment pipeline |
| `internal/eval/` | Behavioral test framework |
| `internal/db/` | SQL migrations and sqlc query files |
| `internal/sqlc/` | **AUTO-GENERATED** by sqlc — never edit directly |
| `internal/model/` | Shared domain types (`Money`, sentinel errors) |
| `internal/zerodha/` | Zerodha Kite Connect API client |

## Adding a bank parser

Each bank/format combination is a small struct that implements `parser.Parser`.

1. **Create** `internal/importer/parser/<bank>.go`.

2. **Implement** the interface:

```go
type Parser interface {
    Bank() string          // short bank identifier, e.g. "axis"
    FormatVersion() string // e.g. "v1"
    CanParse(header []string) bool
    Parse(r io.Reader) (ParseResult, error)
}
```

   `CanParse` receives a lower-cased tokenised row from the statement header and returns true if this parser owns the format. `Parse` reads the full `io.Reader` and returns a `ParseResult` containing `StatementMeta` and a slice of `RawTransaction`.

   See `axis.go` for a CSV example and `hdfc.go` for an XLS example.

3. **Register** in `internal/importer/parser/registry.go` inside `NewRegistry()`:

```go
r.Register(&YourBankV1{})
```

   Parsers are tried newest-first; register newer format versions before older ones.

4. **Add anonymised testdata** in `internal/importer/parser/testdata/<bank>_v1.<ext>` — a small sample with no real account numbers or names.

5. **Write tests** in `internal/importer/parser/<bank>_test.go` following the pattern in `axis_test.go`.

## Adding an agent tool

Each tool is a struct that the LLM can call during a ReAct loop iteration.

1. **Create** `internal/tools/<tool_name>.go`.

2. **Implement** the interface:

```go
type Tool interface {
    Name() string
    Definition() llm.ToolDefinition
    Execute(ctx context.Context, userID string, argsJSON string) (string, error)
}
```

   `Definition()` returns the JSON Schema descriptor shown to the LLM. `Execute()` receives raw JSON args (unmarshal them yourself) and returns a plain-text result string that the model reads back.

3. **Add a dependency interface** in `internal/tools/queriers.go` if your tool needs DB access. This keeps the tool testable without a real database.

4. **Add a sqlc query** if needed:
   - Write SQL in `internal/db/queries/<table>.sql`
   - Run `make generate` to regenerate `internal/sqlc/`
   - Wrap the generated method in `internal/store/<table>_store.go`

5. **Register** in `internal/app/registry.go` inside `BuildToolRegistry`. If the tool depends on the Zerodha service (which requires per-environment querier wiring), register it in `cmd/finagent/main.go` after `BuildToolRegistry` returns — see the existing investment tools for the pattern.

6. **Write tests** in `internal/tools/<tool_name>_test.go` using the mock querier. Run `make mocks` after changing a querier interface to regenerate mocks.

## Adding a database migration

1. **Create** a numbered file pair in `internal/db/migrations/`:

   ```
   000014_your_feature.up.sql
   000014_your_feature.down.sql
   ```

2. **Apply** with `make migrate-up` (or `make migrate-down` to roll back one step).

3. **Write sqlc queries** in `internal/db/queries/<table>.sql`, then run `make generate` to update `internal/sqlc/`.

4. **Wrap** the generated methods in a new or existing `internal/store/<table>_store.go`.

Migrations are embedded into the binary via `//go:embed` — no separate migration binary is needed in production. Auto-migration on startup is controlled by `database.auto_migrate` in config.

## Writing eval test cases

Eval cases live in `internal/eval/scenarios.go`. Each `EvalCase` fires a natural-language prompt at the full agent stack and asserts on tool calls and output content.

```go
{
    Name:              "spending_breakdown_april",
    Input:             "How much did I spend on food in April?",
    MustCallTools:     []string{"get_spending_breakdown"},
    OutputMustContain: []string{"food"},
}
```

For assertions that depend on live DB state, use `ComputeExpected` to compute expected values at runtime:

```go
ComputeExpected: func(ctx context.Context, db *pgxpool.Pool, userID string) (map[string]string, error) {
    // query the seeded DB and return key→value pairs
    return map[string]string{"total": "₹4,500"}, nil
},
// then reference them in assertions:
OutputMustContainOneOf: [][]string{{"₹4,500", "$expected.total"}},
```

Use `PreambleInputs` to warm conversation context before the main assertion input:

```go
PreambleInputs: []string{
    "Remember: my Netflix charge is a subscription.",
},
Input: "What subscriptions do I have?",
```

Run the full suite with `make eval` (requires `make seed` and a running LLM).

## Code conventions

**Money**: always `int64` paise (INR × 100). Use `model.Money` and `model.FromRupees()`. Never use `float64` for currency arithmetic.

**Transactions are immutable**: never `UPDATE transactions`. All enrichments (category, labels, counterparty) live in `transaction_enrichments`.

**Idempotency key**: `SHA256(account_id || txn_date || amount || description)` — import pipelines use this to skip duplicates.

**`context.Context`** is always the first parameter of every function that does I/O.

**No global state** — inject all dependencies via `New*(...)` constructors.

**Interfaces defined in the consuming package**, not the implementing one.

Run `make fmt` before every commit.

## Testing

```bash
go test ./...   # all unit tests — no database or LLM required
make eval       # full behavioural eval suite — requires seeded DB and running LLM
```

Unit tests use mock queriers generated by `mockgen`. If you add or change a querier interface, run `make mocks` to regenerate them.

The eval suite requires:
1. A running database with migrations applied and test data seeded: `docker compose up -d && make migrate-up && make seed`
2. A running LLM at the configured `llm.base_url`
3. Correct `users` and `api_keys` entries in `config.yaml`
