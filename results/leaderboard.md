| Reviewer | Accuracy | Notes follow support principles | $ / proof | p50 latency | Verdict |
|---|---:|---:|---:|---:|---|
| `baseline-rules` | 19/26 (73%) | 100% | $0.0000 | 0s | reference |
| `anthropic/claude-sonnet-5.5` | 25/26 (96%) | 100% | $0.0063 | 4.2s | **keep** |
| `anthropic/claude-haiku-5.5` | 23/26 (88%) | 100% | $0.0003 | 3.9s | **keep** |
| `openai/gpt-6.1-terra` | 24/26 (92%) | 100% | $0.0015 | 4.6s | **keep** |
| `x-ai/grok-4.7` | 25/26 (96%) | 100% | $0.0076 | 10.7s | **keep** |
| `qwen/qwen3-vl-30b-a3b-instruct` | 18/26 (69%) | 100% | $0.0002 | 4.6s | retire: only -4 pts over the rules baseline (need +10) |
| `google/gemma-3-27b-it` | 20/26 (77%) | 100% | $0.0001 | 4.1s | retire: only +4 pts over the rules baseline (need +10) |

<details><summary>Misses</summary>

**baseline-rules**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `wrong-file-attached`: wanted ask_customer, got ready_for_proof
- `size-change`: wanted ask_customer, got ready_for_proof
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `holographic`: wanted ask_customer, got ready_for_proof

**anthropic/claude-sonnet-5.5**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof

**anthropic/claude-haiku-5.5**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `holographic`: wanted ask_customer, got ready_for_proof

**openai/gpt-6.1-terra**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof

**x-ai/grok-4.7**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof

**qwen/qwen3-vl-30b-a3b-instruct**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `fullbleed-photo`: wanted ready_for_proof, got ask_customer
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `holographic`: wanted ask_customer, got ready_for_proof

**google/gemma-3-27b-it**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `holographic`: wanted ask_customer, got ready_for_proof

</details>
