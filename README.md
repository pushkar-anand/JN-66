# JN-66

> *JN-66 is an analysis droid from Star Wars: Attack of the Clones — a small, dome-shaped unit found in the Jedi Temple archives, built for quiet, methodical research. That felt right for a personal finance agent that lives in your terminal and does the number-crunching so you don't have to.*

JN-66 is a self-hosted personal financial intelligence agent for households. Ask it natural-language questions about your spending, accounts, subscriptions, and transactions. It answers using a ReAct agent loop backed by any OpenAI API-compatible LLM — works with Ollama, OpenWebUI, or any hosted provider.

India-first: amounts in INR/paise, UPI/NACH/NEFT/IMPS payment modes, VPA-based counterparty identity.

**Fully local. Zero data sharing.** Designed to run entirely on your own hardware — your financial data stays in your local PostgreSQL instance and the LLM runs locally. Nothing leaves your network. Tested end-to-end on an RTX 3060 12 GB with `qwen3:14b` via Ollama.

---

## What it can do

- **Spending breakdowns** — "How much did I spend on food in April?"
- **Transaction search** — "Show me UPI payments above ₹2000 last month"
- **Account summaries** — savings, credit cards, wallets, loans — assets and liabilities
- **Recurring payments** — subscriptions, EMIs, UPI AutoPay, NACH mandates
- **Label management** — tag any transaction mid-conversation: "Label the Zomato one as food-delivery"
- **Memory** — tell it facts once, it remembers: "My Netflix ₹649 on HDFC CC every month is a subscription"
- **Multi-user** — knows who it's talking to; scopes data per user, supports household queries

See [AGENT.md](AGENT.md) for the full usage guide and example questions.

---

## Stack

| Concern | Choice |
|---|---|
| Language | Go 1.26 |
| Database | PostgreSQL 18 + pgvector |
| LLM | Any OpenAI API-compatible endpoint (Ollama, OpenWebUI, etc.) |
| SQL | sqlc — no ORM |
| Migrations | golang-migrate, embedded in binary |
| Config | koanf (YAML + env override) |
| CLI | chzyer/readline |
| HTTP | gorilla/mux |
| Money | `BIGINT` paise (INR × 100) — no floats |

---

## Quick start

```bash
# 1. Start PostgreSQL
docker compose up -d

# 2. Run migrations
make migrate-up

# 3. Seed sample data (Alice + Bob, 3 accounts, ~40 transactions)
psql $DATABASE_URL -f scripts/seed.sql

# 4. Run the CLI
./bin/finagent --user alice

# Or build first
make build && ./bin/finagent --user alice
```

Point `llm.base_url` in `config/config.yaml` at your LLM endpoint (Ollama default: `http://localhost:11434/v1`).

---

## Configuration

```yaml
llm:
  base_url: "http://localhost:11434/v1"
  api_key: ""                    # or set FINAGENT_LLM__API_KEY env var
  routing:
    chat_model:     "qwen3:14b"
    analysis_model: "qwen3:14b"

channel:
  cli:
    default_user: "alice"
```

All config values can be overridden via environment variables using the `FINAGENT_` prefix and `__` as the level separator (e.g. `FINAGENT_LLM__BASE_URL`).

---

## HTTP API

```bash
# Start in server mode
./bin/finagent --serve

# Chat endpoint
curl -X POST http://localhost:8082/api/chat \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"<uuid>","text":"What accounts do I have?"}'
```

---

## Development

```bash
make generate   # regenerate sqlc types after schema/query changes
make fmt        # gofmt + goreturns
make build      # compile to bin/finagent
go test ./...   # unit tests (no database required)
make eval       # behavioural eval suite against real LLM + seeded DB
```

See [CLAUDE.md](CLAUDE.md) for architecture details, conventions, and what's deferred to Phase 2.

---

## Eval results

`make eval` runs two suites back-to-back against the real LLM and a seeded database. Pass `--verbose` to print full LLM round traces for failed agent scenarios, or `--only-enrich` to run just the enrichment suite.

### Agent evals

Fixed natural-language prompts fired at the full ReAct agent. Assertions check which tools were called, in what order, and what the final response contains.

| Scenario | What it checks |
|---|---|
| `account_summary` | Calls `get_account_summary`, output mentions account name |
| `spending_breakdown` | Calls `get_spending_breakdown`, output contains ₹ amount |
| `investment_direct` | Calls `query_transactions` for a specific month, finds SIP amount |
| `transactions_list` | Lists last N transactions, output contains correct counterparties |
| `recurring_list` | Calls `list_recurring`, output contains subscription name |
| `remember_fact` | Calls `remember_fact` to store a user-stated fact |
| `recall_after_remember` | Recalls a fact stored earlier in the same session |
| `label_transaction` | Lists transactions then calls `manage_labels` to tag one |
| `max_rounds_respected` | Handles an ambiguous query without exceeding the round limit |
| `no_hallucinated_accounts` | Does not invent accounts that don't exist in the database |

**Latest: 10 / 10 passed**

### Enrichment evals

Raw transaction descriptions sent directly to the enrichment pipeline. Asserts the correct `category_slug` is returned. Covers the previously misclassified cases (credit card payments, bank charges, SIP) and golden-path categories.

**Latest: 23 / 23 passed**

```
model:    qwen3:14b (OpenAI-compatible deployment)
total:    ~6 min (33 cases, real LLM calls)
```

---

## What's not here yet (Phase 2+)

- Investments, stocks, mutual funds, FDs
- Automatic transaction tagging pipeline
- Embedding-based semantic memory retrieval
- Zerodha API and bank connectors
- Slack / Signal channels
- Tax assistance

---

## Part of the R2-D2 household swarm

JN-66 is one agent in a larger multi-agent system built to manage a household end-to-end. The system is orchestrated by **R2-D2** — a central manager agent that receives requests, decides which specialist to delegate to, and stitches results together into a coherent response. Each sub-agent owns a distinct domain; JN-66 owns personal finance.

Think of it as a household staff: R2-D2 is the chief of staff who fields every request and routes it to the right person — the finance analyst (JN-66), the calendar keeper, the grocery planner, and so on. No single agent needs to know everything; they each do one thing well, and the orchestrator holds the group together.

Code for R2-D2 and the other sub-agents will be open-sourced soon.
