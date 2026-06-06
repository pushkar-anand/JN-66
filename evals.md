# Eval Experiments

Log of model comparison runs against the finagent eval suite.
Run with: `./bin/eval --config config.yaml --compare "model_a,model_b"`

---

## 2026-06-06 — qwen3:14b vs gemma4:12b-it-qat

**Setup**: 16 agent scenarios, seeded DB + real Zerodha holdings imported, UTC timezone.

| Scenario | qwen3:14b | gemma4:12b-it-qat |
|---|---|---|
| account_summary | ✓ 2r 31.3s | ✓ 2r 29.2s |
| spending_breakdown | ✓ 2r 33.2s | ✓ 2r 20.6s |
| investment_direct | ✓ 2r 71.6s | ✓ 2r 18.6s |
| transactions_list | ✓ 2r 34.2s | ✓ 2r 25.4s |
| recurring_list | ✓ 2r 23.6s | ✓ 2r 11.8s |
| remember_fact | ✓ 2r 17.6s | ✓ 2r 14.7s |
| recall_after_remember | ✓ 2r 26.6s | ✓ 2r 12.6s |
| label_transaction | ✗ missing "food-delivery" | ✓ 4r 52.0s |
| fd_list | ✓ 2r 20.1s | ✓ 2r 10.8s |
| fd_record | ✓ 2r 23.5s | ✓ 2r 17.0s |
| fd_incomplete_prompts_for_details | ✓ 1r 9.1s | ✓ 1r 14.5s |
| max_rounds_respected | ✓ 2r 43.3s | ✓ 4r 32.1s |
| has_zerodha_account | ✓ 2r 22.9s | ✓ 2r 20.8s |
| equity_summary | ✓ 2r 23.4s | ✓ 2r 10.1s |
| mf_summary | ✓ 2r 51.7s | ✓ 2r 21.9s |
| portfolio_total | ✓ 2r 29.4s | ✓ 2r 18.2s |
| **TOTAL** | **15/16 · 521s** | **16/16 · 330s** |

**Winner: gemma4:12b-it-qat** — 1.6× faster overall, higher pass rate.

**Notes:**
- `investment_direct` largest gap: qwen3 71.6s vs gemma4 18.6s — qwen3 likely spending tokens on chain-of-thought for the complex multi-holding query.
- `label_transaction`: qwen3 failed outright; gemma4 passed but hallucinated a truncated UUID on first attempt, then self-corrected in round 2 (4 total rounds).
- `max_rounds_respected`: gemma4 needed 4 rounds vs qwen3's 2 — less efficient on this scenario.
- `fd_incomplete_prompts_for_details`: only scenario where qwen3 was faster (9.1s vs 14.5s).
- Config at time of run: all routing slots set to the model under test; embed_model unchanged (`nomic-embed-text`).
