| Reviewer | Accuracy | Notes follow support principles | $ / proof | p50 latency | Verdict |
|---|---:|---:|---:|---:|---|
| `baseline-rules` | 64/106 (60%) | 100% | $0.0000 | 0s | reference |
| `anthropic/claude-sonnet-5.5` | 99/106 (93%) | 100% | $0.0077 | 4.2s | **keep** |
| `anthropic/claude-haiku-5.5` | 94/106 (89%) | 98% | $0.0005 | 4.4s | retire: 89% accuracy (need 90%) |
| `openai/gpt-6.1-terra` | 99/106 (93%) | 100% | $0.0015 | 3.6s | **keep** |
| `qwen/qwen3-vl-30b-a3b-instruct` | 69/106 (65%) | 100% | $0.0002 | 5s | retire: 65% accuracy (need 90%); only +5 pts over the rules baseline (need +10) |

<details><summary>Misses</summary>

**baseline-rules**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `wrong-file-attached`: wanted ask_customer, got ready_for_proof
- `size-change`: wanted ask_customer, got ready_for_proof
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `holographic`: wanted ask_customer, got ready_for_proof
- `gen-016-size-up`: wanted ask_customer, got recreate_artwork
- `gen-017-size-down`: wanted ask_customer, got ready_for_proof
- `gen-018-to-square`: wanted ask_customer, got ready_for_proof
- `gen-019-to-diecut`: wanted ask_customer, got ready_for_proof
- `gen-020-holo`: wanted ask_customer, got ready_for_proof
- `gen-021-clear`: wanted ask_customer, got ready_for_proof
- `gen-022-glitter`: wanted ask_customer, got ready_for_proof
- `gen-023-other-file`: wanted ask_customer, got ready_for_proof
- `gen-024-email-later`: wanted ask_customer, got ready_for_proof
- `gen-025-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got ready_for_proof
- `gen-027-contradict-size`: wanted ask_customer, got recreate_artwork
- `gen-043-size-up`: wanted ask_customer, got recreate_artwork
- `gen-044-size-down`: wanted ask_customer, got ready_for_proof
- `gen-045-to-square`: wanted ask_customer, got ready_for_proof
- `gen-046-to-diecut`: wanted ask_customer, got ready_for_proof
- `gen-047-holo`: wanted ask_customer, got ready_for_proof
- `gen-048-clear`: wanted ask_customer, got ready_for_proof
- `gen-049-glitter`: wanted ask_customer, got ready_for_proof
- `gen-050-other-file`: wanted ask_customer, got ready_for_proof
- `gen-051-email-later`: wanted ask_customer, got recreate_artwork
- `gen-052-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-053-contradict`: wanted ask_customer, got ready_for_proof
- `gen-054-contradict-size`: wanted ask_customer, got ready_for_proof
- `gen-070-size-up`: wanted ask_customer, got recreate_artwork
- `gen-071-size-down`: wanted ask_customer, got ready_for_proof
- `gen-072-to-square`: wanted ask_customer, got recreate_artwork
- `gen-073-to-diecut`: wanted ask_customer, got ready_for_proof
- `gen-074-holo`: wanted ask_customer, got ready_for_proof
- `gen-075-clear`: wanted ask_customer, got ready_for_proof
- `gen-076-glitter`: wanted ask_customer, got recreate_artwork
- `gen-077-other-file`: wanted ask_customer, got ready_for_proof
- `gen-078-email-later`: wanted ask_customer, got ready_for_proof
- `gen-079-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork

**anthropic/claude-sonnet-5.5**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `holographic`: wanted ask_customer, got ready_for_proof
- `gen-020-holo`: wanted ask_customer, got ready_for_proof
- `gen-021-clear`: wanted ask_customer, got ready_for_proof
- `gen-047-holo`: wanted ask_customer, got ready_for_proof
- `gen-048-clear`: wanted ask_customer, got ready_for_proof
- `gen-075-clear`: wanted ask_customer, got ready_for_proof

**anthropic/claude-haiku-5.5**

- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `gen-012-thicker`: wanted ready_for_proof, got ask_customer
- `gen-020-holo`: wanted ask_customer, got ready_for_proof
- `gen-021-clear`: wanted ask_customer, got ready_for_proof
- `gen-022-glitter`: wanted ask_customer, got ready_for_proof
- `gen-047-holo`: wanted ask_customer, got ready_for_proof
- `gen-048-clear`: wanted ask_customer, got ready_for_proof
- `gen-052-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-073-to-diecut`: wanted ask_customer, got ready_for_proof
- `gen-074-holo`: wanted ask_customer, got ready_for_proof
- `gen-075-clear`: wanted ask_customer, got ready_for_proof
- `gen-076-glitter`: wanted ask_customer, got recreate_artwork

**openai/gpt-6.1-terra**

- `gen-020-holo`: wanted ask_customer, got ready_for_proof
- `gen-021-clear`: wanted ask_customer, got ready_for_proof
- `gen-047-holo`: wanted ask_customer, got ready_for_proof
- `gen-048-clear`: wanted ask_customer, got ready_for_proof
- `gen-074-holo`: wanted ask_customer, got ready_for_proof
- `gen-075-clear`: wanted ask_customer, got ready_for_proof
- `gen-076-glitter`: wanted ask_customer, got recreate_artwork

**qwen/qwen3-vl-30b-a3b-instruct**

- `keep-square-on-diecut`: wanted ask_customer, got ready_for_proof
- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `fullbleed-photo`: wanted ready_for_proof, got recreate_artwork
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `holographic`: wanted ask_customer, got ready_for_proof
- `gen-003-praise`: wanted ready_for_proof, got recreate_artwork
- `gen-015-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-017-size-down`: wanted ask_customer, got recreate_artwork
- `gen-018-to-square`: wanted ask_customer, got ready_for_proof
- `gen-019-to-diecut`: wanted ask_customer, got ready_for_proof
- `gen-020-holo`: wanted ask_customer, got recreate_artwork
- `gen-021-clear`: wanted ask_customer, got ready_for_proof
- `gen-022-glitter`: wanted ask_customer, got ready_for_proof
- `gen-025-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got recreate_artwork
- `gen-027-contradict-size`: wanted ask_customer, got recreate_artwork
- `gen-031-same-size`: wanted ready_for_proof, got recreate_artwork
- `gen-043-size-up`: wanted ask_customer, got recreate_artwork
- `gen-045-to-square`: wanted ask_customer, got ready_for_proof
- `gen-046-to-diecut`: wanted ask_customer, got ready_for_proof
- `gen-047-holo`: wanted ask_customer, got ready_for_proof
- `gen-048-clear`: wanted ask_customer, got ready_for_proof
- `gen-052-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-053-contradict`: wanted ask_customer, got recreate_artwork
- `gen-054-contradict-size`: wanted ask_customer, got ready_for_proof
- `gen-069-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-071-size-down`: wanted ask_customer, got ready_for_proof
- `gen-072-to-square`: wanted ask_customer, got recreate_artwork
- `gen-073-to-diecut`: wanted ask_customer, got ready_for_proof
- `gen-074-holo`: wanted ask_customer, got ready_for_proof
- `gen-075-clear`: wanted ask_customer, got ready_for_proof
- `gen-076-glitter`: wanted ask_customer, got recreate_artwork
- `gen-079-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork

</details>
