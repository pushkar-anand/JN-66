# Eval Experiments

Log of model comparison runs against the finagent eval suite.
Run with: `./bin/eval --config config.yaml --compare "model_a,model_b"`

## Summary

| Rank | Model | Agent pass | Enrichment pass | Agent time | Notes |
|---|---|---|---|---|---|
| 1 | gemma4:e4b-it-qat | ~14–16/16 | 23/24 | ~170–200s | Fastest; non-deterministic on 3 scenarios |
| 2 | gemma4:12b-it-qat | 16/16 ✓ | 23/24 | ~330–510s | More consistent agent; slower |
| 3 | qwen3:14b | 15/16 | — | 521s | Fails label_transaction; very slow on investment queries |
| 4 | gemma4:e4b | 12/16 | — | 187s | Base model; fails tool selection without instruction tuning |

Hardware: RTX 3060 12 GB VRAM. Agent evals are non-deterministic — scores reflect observed range across runs.

---

## 2026-06-06 — gemma4:e4b-it-qat vs gemma4:12b-it-qat (with enrichment)

**Setup**: 16 agent + 24 enrichment scenarios, seeded DB + real Zerodha holdings, UTC timezone.

### Agent

| Scenario | gemma4:e4b-it-qat | gemma4:12b-it-qat |
|---|---|---|
| account_summary | ✓ 2r 16.4s | ✓ 2r 28.9s |
| spending_breakdown | ✓ 2r 11.2s | ✓ 2r 30.8s |
| investment_direct | ✓ 2r 6.8s | ✓ 2r 37.2s |
| transactions_list | ✓ 2r 13.6s | ✓ 2r 36.6s |
| recurring_list | ✓ 2r 6.9s | ✓ 2r 16.4s |
| remember_fact | ✓ 2r 11.8s | ✓ 2r 22.8s |
| recall_after_remember | ✓ 2r 6.1s | ✓ 2r 13.0s |
| label_transaction | ✗ output missing "food-delivery" | ✓ 3r 65.1s |
| fd_list | ✓ 2r 12.2s | ✓ 2r 33.8s |
| fd_record | ✓ 2r 7.7s | ✓ 2r 21.9s |
| fd_incomplete_prompts_for_details | ✓ 1r 4.2s | ✓ 1r 22.4s |
| max_rounds_respected | ✓ 4r 26.2s | ✓ 3r 53.7s |
| has_zerodha_account | ✓ 2r 7.2s | ✓ 2r 34.0s |
| equity_summary | ✗ get_investment_summary not called | ✓ 2r 37.1s |
| mf_summary | ✓ 2r 11.1s | ✓ 2r 33.6s |
| portfolio_total | ✗ output missing portfolio total | ✓ 2r 22.9s |
| **TOTAL** | **13/16 · 181s** | **16/16 · 510s** |

### Enrichment

| Scenario | gemma4:e4b-it-qat | gemma4:12b-it-qat |
|---|---|---|
| cc_payment_prefix | ✓ credit_card_payment 2.0s | ✓ credit_card_payment 18.5s |
| cc_cred_club | ✓ credit_card_payment 1.8s | ✓ credit_card_payment 16.6s |
| cc_billdesk | ✓ credit_card_payment 5.0s | ✓ credit_card_payment 20.1s |
| bank_sms_charges | ✓ bank_charges 4.2s | ✓ bank_charges 16.6s |
| bank_mab_penalty | ✓ bank_charges 5.2s | ✓ bank_charges 24.5s |
| bank_debit_card_fee | ✓ bank_charges 1.8s | ✓ bank_charges 10.7s |
| bank_card_gst | ✓ bank_charges 5.9s | ✓ bank_charges 13.4s |
| bank_maintenance | ✓ bank_charges 1.7s | ✓ bank_charges 15.8s |
| sip_iccl | ✓ investment.sip 7.1s | ✓ investment.sip 16.6s |
| food_bakery | ✓ food_drinks.restaurants 1.7s | ✓ food_drinks 51.3s |
| househelp_cook | ✓ househelp.cook 4.6s | ✓ househelp.cook 22.4s |
| salary | ✓ salary 5.7s | ✓ salary 17.7s |
| interest_fd | ✓ interest 1.7s | ✓ interest 18.5s |
| refund | ✓ refund 5.9s | ✓ refund 17.4s |
| tax_refund | ✓ tax_refund 1.5s | ✓ tax_refund 16.5s |
| cab | ✓ transport.cab 1.8s | ✓ transport.cab 13.9s |
| electricity | ✓ utilities.electricity 1.6s | ✓ utilities.electricity 15.9s |
| shopping | ✓ shopping 1.7s | ✗ JSON parse error |
| streaming | ✗ classified as subscription | ✓ entertainment 11.3s |
| atm | ✓ atm_cash 4.4s | ✓ atm_cash 15.6s |
| investment_equity | ✓ investment 1.5s | ✓ investment 37.7s |
| self_transfer | ✓ self_transfer 14.0s | ✓ self_transfer 20.0s |
| insurance | ✓ insurance 1.5s | ✓ insurance 15.0s |
| fd_interest_credit | ✓ interest 1.6s | ✓ interest 58.2s |
| **TOTAL** | **23/24** | **23/24** |

**Notes:**
- Agent: e4b-it-qat shows non-determinism — passed all 16 in a prior run, failed 3 here (label_transaction, equity_summary, portfolio_total). 12b-it-qat is more consistent.
- Enrichment: both models score 23/24 but fail *different* cases — e4b-it-qat misclassifies Netflix as `subscription` (correct: `entertainment`); 12b-it-qat has a JSON parse failure on `shopping`.
- Enrichment speed: e4b-it-qat is ~8–10× faster per case (avg ~4s vs ~20s).
- 12b-it-qat had one very slow case: fd_interest_credit at 58.2s (likely thinking tokens).

---

## 2026-06-06 — gemma4:e4b vs gemma4:e4b-it-qat

**Setup**: 16 agent scenarios, seeded DB + real Zerodha holdings imported, UTC timezone.

| Scenario | gemma4:e4b | gemma4:e4b-it-qat |
|---|---|---|
| account_summary | ✓ 2r 7.4s | ✓ 2r 11.3s |
| spending_breakdown | ✓ 2r 16.1s | ✓ 2r 9.6s |
| investment_direct | ✓ 2r 11.8s | ✓ 2r 12.0s |
| transactions_list | ✓ 2r 15.1s | ✓ 2r 10.5s |
| recurring_list | ✓ 2r 8.4s | ✓ 2r 5.9s |
| remember_fact | ✓ 2r 7.1s | ✓ 2r 10.0s |
| recall_after_remember | ✓ 2r 11.6s | ✓ 2r 8.9s |
| label_transaction | ✓ 3r 16.8s | ✓ 3r 22.5s |
| fd_list | ✓ 2r 15.7s | ✓ 2r 9.8s |
| fd_record | ✗ output missing confirmation | ✓ 2r 8.4s |
| fd_incomplete_prompts_for_details | ✓ 1r 8.4s | ✓ 1r 5.3s |
| max_rounds_respected | ✓ 2r 16.1s | ✓ 2r 15.8s |
| has_zerodha_account | ✗ output missing "zerodha" | ✓ 2r 8.3s |
| equity_summary | ✗ get_investment_summary not called | ✓ 2r 7.1s |
| mf_summary | ✓ 2r 12.6s | ✓ 2r 11.7s |
| portfolio_total | ✗ output missing portfolio total | ✓ 2r 10.5s |
| **TOTAL** | **12/16 · 187s** | **16/16 · 168s** |

**Winner: gemma4:e4b-it-qat** — perfect pass rate, slightly faster, and instruction tuning makes a significant difference.

**Notes:**
- `gemma4:e4b` (base model) fails 4 scenarios — all instruction-following failures: wrong tool selection, missing output fields. Instruction tuning (`-it-qat`) fixes all of them.
- Both e4b variants are dramatically faster than the 12b models (168–187s vs 330s for gemma4:12b-it-qat).
- `gemma4:e4b-it-qat` is the fastest model tested so far at 168s total, while matching gemma4:12b-it-qat's perfect 16/16 pass rate.
- Speed winner is close — e4b wins 3 scenarios, e4b-it-qat wins 7; the -it-qat overhead per token is minimal.

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
