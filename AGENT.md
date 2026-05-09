# finagent — User Guide

## What the agent knows

finagent is a personal financial intelligence agent for your household. It can answer questions about:

- **Transactions** — what you spent, where, when, how much
- **Spending breakdowns** — category totals for any date range
- **Account balances and summaries** — savings, credit cards, wallets
- **Recurring payments** — subscriptions, EMIs, UPI AutoPay, NACH
- **Labels** — custom tags you or the agent has applied to transactions
- **Household facts** — anything you've asked the agent to remember

## Example questions

```
What did I spend on food in April?
Show me all UPI transactions above ₹2000 last month.
How much have I paid towards my HDFC credit card this year?
What are my active subscriptions?
Which account has the highest balance?
Did I get a refund from Zomato recently?
Show me transfers between me and my partner this month.
What's my total spending by category for Q1?
```

## Teaching the agent

You can tell the agent facts and it will remember them:

```
My Netflix ₹649 charge on HDFC CC every month is a subscription.
Zomato@axisbank is Zomato — always tag it as food delivery.
The ₹50,000 transfer to my partner's SBI account on the 1st is rent share.
```

The agent stores these as memories and uses them when tagging future transactions and answering questions.

## Tools the agent has

| Tool | What it does |
|---|---|
| `query_transactions` | Filter transactions by date, account, category, label, amount, payment mode, counterparty |
| `get_account_summary` | List accounts with type, class (asset/liability), and recent activity |
| `get_spending_breakdown` | Grouped spend by category or sub-category for a date range |
| `manage_labels` | Add or remove labels on a transaction or a split |
| `list_recurring` | Show active recurring payments and upcoming expected charges |
| `remember_fact` | Store a fact in memory; can also create a recurring payment rule |
| `recall_facts` | Search memories by topic tags |

## Multi-user

The agent knows who it's talking to. Run the CLI with `--user <name>` to identify yourself. By default it scopes all data to your accounts. You can ask about your partner's accounts explicitly:

```
What did my partner spend on transport this month?
Show me our combined household spending in March.
```

## Data model (brief)

- **Transactions** — immutable records exactly as the bank reported them
- **Enrichments** — category, labels, notes, transfer links (added by you or the agent)
- **Categories** — system-defined hierarchy (food & drinks → delivery / restaurants / groceries)
- **Labels** — flat, user-defined tags (e.g. "vacation", "reimbursable", "split with partner")
- **Recurring payments** — known recurring charges matched against future transactions
- **Memories** — facts the agent has been told or inferred

## Importing transactions

Phase 1 supports CSV import. Place your bank CSV in the import directory and run:

```bash
./bin/finagent import --account <account-id> --file transactions.csv
```

(Zerodha API import and other bank connectors are planned for Phase 2.)

## Limitations (Phase 1)

- Investments, stocks, mutual funds — not yet tracked
- Physical assets (car, gold, property) — not yet tracked
- Automatic transaction tagging — not yet active (manual category assignment only)
- Tax calculations — not yet supported
- Slack / Signal channels — CLI only for now
