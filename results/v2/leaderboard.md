| Reviewer | Accuracy | Notes follow support principles | $ / proof | p50 latency | Verdict |
|---|---:|---:|---:|---:|---|
| `baseline-rules` | 64/106 (60%) | 100% | $0.0000 | 0s | reference |
| `anthropic/claude-sonnet-5.5` | 100% (100–100) | 100% | $0.0078 | 4.3s | **keep** |
| `anthropic/claude-haiku-5.5` | 97% (96–98) | 99% | $0.0005 | 4.8s | **keep** |
| `openai/gpt-6.1-terra` | 100% (99–100) | 100% | $0.0016 | 3.7s | **keep** |
| `x-ai/grok-4.7` | 100% (100–100) | 99% | $0.0102 | 11.9s | retire: $0.0102 per proof (max $0.01) |
| `qwen/qwen3-vl-30b-a3b-instruct` | 80% (79–80) | 100% | $0.0002 | 5.6s | retire: 80% accuracy (need 90%) |
| `google/gemma-3-27b-it` | 86% (81–91) | 99% | $0.0002 | 5.2s | retire: 86% accuracy (need 90%) |
| `anthropic/claude-opus-5.5` | 105/106 (99%) | 100% | $0.0162 | 8s | retire: $0.0162 per proof (max $0.01) |

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

**anthropic/claude-haiku-5.5**

- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `gen-012-thicker`: wanted ready_for_proof, got ask_customer
- `gen-079-hires-later`: wanted ask_customer, got recreate_artwork
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `gen-012-thicker`: wanted ready_for_proof, got ask_customer
- `gen-066-thicker`: wanted ready_for_proof, got ask_customer
- `gen-079-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-012-thicker`: wanted ready_for_proof, got ask_customer
- `gen-066-thicker`: wanted ready_for_proof, got ask_customer

**openai/gpt-6.1-terra**

- `conflicting-background`: wanted ask_customer, got ready_for_proof

**qwen/qwen3-vl-30b-a3b-instruct**

- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `fullbleed-photo`: wanted ready_for_proof, got recreate_artwork
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `gen-003-praise`: wanted ready_for_proof, got recreate_artwork
- `gen-006-remove-bg`: wanted ready_for_proof, got recreate_artwork
- `gen-015-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-025-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got recreate_artwork
- `gen-027-contradict-size`: wanted ask_customer, got recreate_artwork
- `gen-031-same-size`: wanted ready_for_proof, got recreate_artwork
- `gen-036-remove-bg-vi`: wanted ready_for_proof, got recreate_artwork
- `gen-047-holo`: error: qwen/qwen3-vl-30b-a3b-instruct: answer cut off at the token limit
- `gen-052-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-053-contradict`: wanted ask_customer, got recreate_artwork
- `gen-061-remove-bg-fr`: wanted ready_for_proof, got recreate_artwork
- `gen-068-bigger-text`: error: qwen/qwen3-vl-30b-a3b-instruct: answer cut off at the token limit
- `gen-069-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-079-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork
- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `fullbleed-photo`: wanted ready_for_proof, got recreate_artwork
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `gen-003-praise`: wanted ready_for_proof, got recreate_artwork
- `gen-006-remove-bg`: wanted ready_for_proof, got recreate_artwork
- `gen-025-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got recreate_artwork
- `gen-027-contradict-size`: wanted ask_customer, got recreate_artwork
- `gen-029-rush`: error: qwen/qwen3-vl-30b-a3b-instruct: answer cut off at the token limit
- `gen-031-same-size`: wanted ready_for_proof, got recreate_artwork
- `gen-046-to-diecut`: error: qwen/qwen3-vl-30b-a3b-instruct: answer cut off at the token limit
- `gen-052-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-053-contradict`: wanted ask_customer, got recreate_artwork
- `gen-054-contradict-size`: wanted ask_customer, got ready_for_proof
- `gen-061-remove-bg-fr`: wanted ready_for_proof, got recreate_artwork
- `gen-069-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-079-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork
- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `lowres-sending-hires`: wanted ask_customer, got recreate_artwork
- `fullbleed-photo`: wanted ready_for_proof, got recreate_artwork
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `gen-003-praise`: wanted ready_for_proof, got recreate_artwork
- `gen-006-remove-bg`: wanted ready_for_proof, got recreate_artwork
- `gen-015-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-025-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got recreate_artwork
- `gen-027-contradict-size`: wanted ask_customer, got recreate_artwork
- `gen-031-same-size`: wanted ready_for_proof, got recreate_artwork
- `gen-047-holo`: error: qwen/qwen3-vl-30b-a3b-instruct: answer cut off at the token limit
- `gen-052-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-053-contradict`: wanted ask_customer, got recreate_artwork
- `gen-054-contradict-size`: wanted ask_customer, got ready_for_proof
- `gen-061-remove-bg-fr`: wanted ready_for_proof, got recreate_artwork
- `gen-068-bigger-text`: wanted ready_for_proof, got recreate_artwork
- `gen-069-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-079-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork

**google/gemma-3-27b-it**

- `medium-res`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `medium-thin-whitebox`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-025-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got ready_for_proof
- `gen-051-email-later`: wanted ask_customer, got recreate_artwork
- `gen-053-contradict`: wanted ask_customer, got ready_for_proof
- `gen-069-redraw-ok`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork
- `lowres-diecut`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got ready_for_proof
- `gen-027-contradict-size`: wanted ask_customer, got recreate_artwork
- `gen-029-rush`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-031-same-size`: wanted ready_for_proof, got recreate_artwork
- `gen-034-remove-bg-fr`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-035-remove-bg-de`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-053-contradict`: wanted ask_customer, got ready_for_proof
- `gen-055-none`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-061-remove-bg-fr`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-069-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork
- `lowres-diecut`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `medium-res`: wanted ready_for_proof, got recreate_artwork
- `missing-blue-version`: wanted ask_customer, got ready_for_proof
- `conflicting-background`: wanted ask_customer, got ready_for_proof
- `medium-thin-whitebox`: wanted ready_for_proof, got recreate_artwork
- `gen-003-praise`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-018-to-square`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-025-hires-later`: wanted ask_customer, got recreate_artwork
- `gen-026-contradict`: wanted ask_customer, got ready_for_proof
- `gen-027-contradict-size`: wanted ask_customer, got recreate_artwork
- `gen-029-rush`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-030-praise`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-033-remove-bg`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-035-remove-bg-de`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-043-size-up`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-053-contradict`: wanted ask_customer, got ready_for_proof
- `gen-055-none`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-058-same-size`: error: google/gemma-3-27b-it: answer cut off at the token limit
- `gen-069-redraw-ok`: wanted ready_for_proof, got recreate_artwork
- `gen-080-contradict`: wanted ask_customer, got recreate_artwork

**anthropic/claude-opus-5.5**

- `gen-054-contradict-size`: wanted ask_customer, got ready_for_proof

</details>
