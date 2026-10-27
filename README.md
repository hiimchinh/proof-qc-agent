# proof-qc-agent

An agent that does the first pass on sticker artwork before a designer makes the proof.

It measures the file against the order, reads the customer's comment, and decides one of three things:

- `ready_for_proof`: a designer can start now. Any fixes go in a short designer note.
- `recreate_artwork`: the art is under 150 dpi at the ordered size, so a designer redraws it first.
- `ask_customer`: only the customer can unblock the proof (missing file, size or material change, conflicting instructions).

It also writes the note the customer reads. Every note is checked against the support principles in the [Sticker Mule Guide](https://www.stickermule.com/guide). If a note says "sorry" or forgets the customer's name, the agent gets one chance to rewrite it.

The goal is a shorter time to proof without letting a bad proof reach a customer. A person still approves every proof.

## Results

26 hand-labelled orders, one run, October 2026. Every model gets the same prompt and the same preflight.

| Reviewer | Accuracy | Notes follow support principles | $ / proof | p50 latency | Verdict |
|---|---:|---:|---:|---:|---|
| `baseline-rules` | 19/26 (73%) | 100% | $0.0000 | 0s | reference |
| `anthropic/claude-sonnet-5.5` | 25/26 (96%) | 100% | $0.0063 | 4.2s | **keep** |
| `anthropic/claude-haiku-5.5` | 23/26 (88%) | 100% | $0.0003 | 3.9s | **keep** |
| `openai/gpt-6.1-terra` | 24/26 (92%) | 100% | $0.0015 | 4.6s | **keep** |
| `x-ai/grok-4.7` | 25/26 (96%) | 100% | $0.0076 | 10.7s | **keep** |
| `qwen/qwen3-vl-30b-a3b-instruct` | 18/26 (69%) | 100% | $0.0002 | 4.6s | retire: only -4 pts over the rules baseline (need +10) |
| `google/gemma-3-27b-it` | 20/26 (77%) | 100% | $0.0001 | 4.1s | retire: only +4 pts over the rules baseline (need +10) |

What the numbers say:

- **Reading the comment is the whole job.** The rules-only baseline gets 73%; it misses all seven cases where the comment changes the answer. Sonnet 5.5 and Grok 4.7 get 25 of 26.
- **Haiku 5.5 is the one I would try first.** It costs $0.0003 per proof, 21x less than Sonnet, at 88%. All three of its misses are orders that should have gone back to the customer, so in shadow mode I would watch that class first.
- **The two open-weight models are retired.** They treat 160 to 200 dpi as "recreate" and miss comments that point to a file that is not attached. Cheap and wrong is not cheap.
- **Every model failed the same case.** A customer on a die-cut order asks to "keep the cream background as a square". Every model agreed to cut a square. A square sticker is a different product, so the order has to change first. Models lean toward pleasing the customer; product rules like this belong in code, not in the prompt. That is the next check I would add.
- **The lint earns its place.** GPT-6.1 Terra left out the customer's name in 10 of 26 first drafts. The repair loop fixed every one, and the result records which rule each draft broke.

The whole eval, including one rerun, cost under $0.50.

An agent stays only if it beats the rules-only baseline by 10 points, 95% of its notes follow the support principles, it costs under $0.01 per proof and its p50 latency is under 30 s. Everything else is retired. Raw answers for every case are in [`results/`](results).

## How it works

```
artwork + order ──▶ preflight (Go, exact) ──▶ model (any, via OpenRouter) ──▶ note lint ──▶ decision + notes
                     dpi, background,          reads comment + image           support      │
                     safe margin, thin lines                                   principles   └─ one repair if a rule breaks
```

1. **Preflight** (`internal/artwork`) measures what code can measure exactly: effective dpi at the ordered size, a solid background on a die cut, art inside the 1/16 in safe margin, strokes under 1/32 in, white art on a transparent background. Models are not asked to guess numbers.
2. **The model** (`internal/agent`) does the part that needs judgment: reading the comment ("I'll email the blue version", "make it 5 inches", "Por favor quiten el fondo blanco") and weighing it against the preflight.
3. **Note lint** (`internal/notelint`) enforces the Guide: no sorry, can't, won't or unfortunately; no "appreciate"; at most one exclamation point; use the customer's name; under 60 words.
4. **Eval** (`internal/eval`) scores every model against the rules-only baseline on accuracy, note quality, cost and latency, and returns keep or retire with the reasons.

The model never acts on an order. The worst a wrong answer can do is send a designer a suggestion that they decline.

## Labelling rules

Each case's expected decision was written by hand from these rules, before any model ran:

1. `ask_customer` when the comment refers to a file that is not attached, asks for a size, shape or material different from the order, gives instructions that contradict each other, or there is no artwork. This wins over the other two.
2. `recreate_artwork` when the art is under 150 dpi at the ordered size. The Guide says to always try to recreate low-resolution artwork rather than ask for a new file.
3. `ready_for_proof` otherwise, including fixes a designer can make alone: remove a background, thicken lines, move art off the edge, edit text the customer wrote out.

The artwork is synthetic and drawn by `cmd/genfixtures`, so the set is reproducible and contains no customer data. CI regenerates it and fails if a byte changes.

## Run it

```sh
go test ./...                                   # no network, no key
go run ./cmd/proofqc eval                       # rules baseline only

export OPENROUTER_API_KEY=...
go run ./cmd/proofqc check -model x-ai/grok-4.7 -product die_cut -w 3 -h 3 \
  -customer "Brian Smith" -comment "Remove the white box please" art.png
go run ./cmd/proofqc eval -max-usd 2 \
  -models anthropic/claude-sonnet-5.5,openai/gpt-6.1-terra,x-ai/grok-4.7,qwen/qwen3-vl-30b-a3b-instruct
```

`-max-usd` stops new calls once the spend passes the cap. The client never retries on its own, so every billed attempt is counted.

## What I would do next in production

1. Run it in shadow mode on real orders for two weeks: the agent decides, designers work as usual, and the eval compares the two.
2. Grow the labelled set from orders designers had to send back, and gate prompt and model changes on it in CI.
3. Put it behind the order queue: a Cloud Run job reads new orders from Postgres, writes its decision through the GraphQL API as a suggestion on the order, and records cost and latency per call.
4. Track the metric it exists for, time to proof, plus the share of proofs approved without edits. If those do not move, retire it.

## License

MIT
