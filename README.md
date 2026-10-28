# proof-qc-agent
[![Live desk: what customers got from the latest 100 simulated orders, then the model behind it](docs/live-desk.png)](https://proof-qc.hiimchinh.com)

Live: **https://proof-qc.hiimchinh.com**. Agents work a simulated order stream every 10 minutes with nobody clicking. [Tool trace for one order](docs/tool-trace.png).

An agent that does the first pass on sticker artwork before a designer makes the proof.

It measures the file against the order, reads the customer's comment, and decides one of three things:

- `ready_for_proof`: a designer can start now. Any fixes go in a short designer note.
- `recreate_artwork`: the art is under 150 dpi at the ordered size, so a designer redraws it first.
- `ask_customer`: only the customer can unblock the proof (missing file, size or material change, conflicting instructions).

It also writes the note the customer reads. Every note is checked against the support principles in the [Sticker Mule Guide](https://www.stickermule.com/guide). If a note says "sorry" or forgets the customer's name, the agent gets one chance to rewrite it.

## Why it matters

Every Sticker Mule order gets a free proof shortly after checkout and unlimited free changes ([proofing FAQ](https://www.stickermule.com/stickermule/faqs/proofing)). The card is charged only when the customer approves ([FAQ](https://www.stickermule.com/faqs/stickermule/can-i-see-a-proof-before-ordering)), and the delivery date moves if the proof is not approved by 5pm EST the next business day ([FAQ](https://www.stickermule.com/faqs/stickermule/how-long-to-approve-proof)). So every revision round costs designer time, delays the charge and can push a delivery date.

This agent reads every order before the proof. Clean orders go straight to a designer. Problems (low-resolution art, a size or material change hidden in a comment, conflicting instructions) are caught before the customer sees a proof, which saves a free revision round. The live desk counts this for the latest 100 orders: sent straight to proof, issues caught, issues missed (each costs the customer an extra round) and unneeded questions or rework. A person still approves every proof.

## The hard parts

The sticker checks are the easy part. This is a proof of concept for running agents unattended:

- **Models extract, code decides.** The model reads the size, shape and material a customer asks for; [code](internal/agent/order_rules.go) decides. Moving those rules out of the prompt took accuracy from 89–93% to 97–100%.
- **Bounded tool loop.** [Each agent](internal/agent/desk.go) gets at most six calls, never retries, and a tool result cannot bypass the rules. Every step is traced.
- **A fleet that manages itself.** [The policy](internal/fleet/fleet.go) promotes a challenger that beats the champion, retires models that miss accuracy, note quality, cost or latency, retires a model early after five consecutive failed calls, and seats the next candidate. Credit, billing and rate-limit failures pause model scoring instead of retiring models; a challenger the provider rate-limits on 12 calls in a row is skipped for the next candidate. Every change is stored with the numbers behind it.
- **Money is a hard limit.** One [daily ledger](internal/api/budget.go) covers scheduled agents and visitors; a cap leaves work queued and resumes it without paying twice.
- **Safe to run more than once.** Ticks are serialized with a [Postgres advisory lock](internal/store/coordination.go); abandoned orders are recovered.

## Runs on its own

A scheduled `POST /internal/tick` enqueues one **simulated order** from the 106 labelled cases embedded in the binary. The champion handles it; the challenger runs in shadow on the same artwork and instructions. Both choose tools: `measure_artwork` runs the exact Go preflight, `customer_history` reads earlier orders from Postgres, and terminal `route_order` submits a decision. Code applies the order rules and note checks. Each tool step, override, failed call, cost and latency is saved.

After at least 30 orders per model, the fleet applies the [eval policy](internal/eval/eval.go): ≥90% accuracy, ≥10 points over rules measured on the stream, ≥95% clean notes, ≤$0.01/order, p50 ≤30 s. A passing challenger replaces the champion for higher accuracy, then lower cost. Failing models are retired. Either model is retired early after 5 consecutive failed calls, even before the 30-order minimum; any answered call resets the streak. A challenger that its provider rate-limits on 12 consecutive calls (about two hours of ticks) is skipped with a `skipped` event, unscored, so a free model that is never served cannot hold the seat. An empty challenger seat takes the next never-tried model from `CHALLENGER_QUEUE` (comma-separated OpenRouter IDs; default `google/gemma-4-31b-it,qwen/qwen3-vl-235b-a22b-instruct,x-ai/grok-4.7`); persisted membership history prevents retries across restarts, and the desk shows the remaining queue or “Queue empty”. State changes and their comparison numbers are committed together. If neither model nor an untried queued candidate remains, orders stay queued. These are synthetic labels, not production quality estimates.

```mermaid
flowchart LR
  Scheduler[Scheduled POST /internal/tick] --> Queue[(Postgres order queue)]
  Queue --> Champion[Champion acts]
  Queue --> Challenger[Challenger shadows]
  Champion --> Tools[Measure / customer history / route]
  Challenger --> Tools
  Tools --> Rules[Order rules and note checks]
  Rules --> Runs[(Runs, traces, spend)]
  Runs --> Policy[Accuracy, notes, cost, latency]
  Policy --> Fleet[Promote or retire + event evidence]
  Fleet --> Champion
  Runs --> Desk[GraphQL live desk]
```

In the live demo, Cloud Scheduler calls the tick every 10 minutes. Ticks are serialized across instances with a Postgres advisory lock. Each tick processes at most two queued orders, each agent gets at most six calls, and calls never retry. The shared daily USD ledger includes both agents and manual reviews. A cap leaves unfinished orders queued; a subsequent tick resumes saved roles without calling them twice. A call already in flight can cross the cap; its cost still counts.

`CHAMPION_MODEL` defaults to `openai/gpt-6.1-terra`; `CHALLENGER_MODEL` defaults to `anthropic/claude-haiku-5.5`. They seed the fleet only on first boot; subsequent boots use its persisted state. An empty `TICK_TOKEN` disables the endpoint. Configure the scheduler to send the token in `X-Tick-Token`; never expose it to the browser. The page refreshes persisted results every 10 seconds and never starts paid work on a read.

## Live demo

https://proof-qc.hiimchinh.com (Cloud Run, us-central1; Postgres on Cloud SQL)

The first view is the live desk: what customers got over the latest 100 orders, the model behind it, then orders, tool traces, the champion/challenger scoreboard and fleet events. The separate “Try your own artwork” form still accepts a PNG or JPEG, a model and order instructions. The page is TypeScript, the API is GraphQL in Go (`internal/api`), and every review is stored in Postgres. Each visitor gets 20 reviews an hour, and the demo stops calling models once the day's spend reaches a cap.

## Results

106 synthetic cases: 26 hand-written cases plus 80 generated by `cmd/genfixtures` from a comment bank with hand-labelled effects. The set includes French, German, Vietnamese and Spanish comments and non-ASCII customer names. Labels: 54 `ready_for_proof`, 43 `ask_customer`, 9 `recreate_artwork`. The rules-only baseline gets 64/106 (60%).

The numbers below are from the committed structured-output runs. They do **not** measure the new tool-loop agent. Tool-loop results use separate `*.tools.*` files. A [three-order Haiku smoke run](results/tools-smoke/leaderboard.tools.limit3.md) recorded 3/3 correct decisions, six model calls and $0.0021647 total spend. All three trace checks passed. These first three cases are simple; the sample is below the fleet’s 30-order minimum and is not evidence of general accuracy.

### Models extract, code decides

The model extracts the requested size, shape and material. Code compares them with the order. Any order change forces `ask_customer` and a templated customer note.

On the same 106 cases, moving order rules from the prompt into code changed the results:

| Model | Prompt only, one run | Rules in code, mean (range), three runs |
|---|---:|---:|
| Sonnet 5.5 | 93% | 100% (100–100) |
| GPT-6.1 Terra | 93% | 100% (99–100) |
| Haiku 5.5 | 89%, retire | 97% (96–98), keep |

Most prompt-only misses were material or shape changes: holographic, clear or glitter vinyl, or die cut versus rectangle. See the [prompt-only results](results/v2-prompt-only/leaderboard.md) and [rules-in-code results](results/v2/leaderboard.md) for every miss and raw run.

### Keep or retire

Accuracy is rounded to whole percentages. Ranges show the lowest and highest run. Models have three runs each, except Grok with two and Opus with one.

| Model | Accuracy, mean (range) | Notes pass | $ / proof | p50 | Verdict |
|---|---:|---:|---:|---:|---|
| Sonnet 5.5 | 100% (100–100) | 100% | $0.0078 | 4.3s | keep |
| GPT-6.1 Terra | 100% (99–100) | 100% | $0.0016 | 3.7s | keep |
| Haiku 5.5 | 97% (96–98) | 99% | $0.0005 | 4.8s | keep |
| Grok 4.7 | 100% (100–100) | 99% | $0.0102 | 11.9s | retire: over cost cap |
| Opus 5.5 | 99%, one run | 100% | $0.0162 | 8s | retire: over cost cap |
| Qwen3-VL 30B | 80% (79–80) | 100% | $0.0002 | 5.6s | retire: below accuracy floor |
| Gemma 3 27B | 86% (81–91) | 99% | $0.0002 | 5.2s | retire: below accuracy floor |

An agent stays only if accuracy is at least 90%, gain over the rules baseline is at least 10 percentage points, at least 95% of answered customer notes pass the support checks, cost is at most $0.01 per proof, and p50 is at most 30 s. I added the 90% floor after the larger set pushed the baseline down to 60%. The old +10-point rule was too easy: Qwen at 80% would have passed.

I would run GPT-6.1 Terra by default: 100% rounded mean accuracy at $0.0016 per proof, about 5x cheaper than Sonnet. Haiku is the cheapest option that passes; I would watch its misses in shadow mode.

### Limits and next step

The synthetic set is now too easy for the top models, which reach 100% rounded mean accuracy. The next step is real orders in shadow mode, not a claim of production accuracy. GPT-6 Astra was dropped because the OpenRouter balance ran out mid-run. Grok has only two runs and Opus only one.

### Earlier finding

In the [original 26-case run](results/v1-26-cases/leaderboard.md), every model accepted a square sticker for a die-cut order. That failure led to moving order-change rules into code.

## How it works

```
artwork + order ──▶ preflight (Go, exact) ──▶ model (any, via OpenRouter) ──▶ note lint ──▶ decision + notes
                     dpi, background,          reads comment + image           support      │
                     safe margin, thin lines                                   principles   └─ one repair if a rule breaks
```

1. **Preflight** (`internal/artwork`) measures what code can measure exactly: effective dpi at the ordered size, a solid background on a die cut, art inside the 1/16 in safe margin, strokes under 1/32 in, white art on a transparent background. Models are not asked to guess numbers.
2. **The model** (`internal/agent`) reads the comment and image, extracts requests, and weighs them against the preflight. **Order rules in code** compare the requested size, shape and material with the order. Any change overrides the model's decision with `ask_customer` and a templated customer note.
3. **Note lint** (`internal/notelint`) enforces the Guide: no sorry, can't, won't or unfortunately; no "appreciate"; at most one exclamation point; use the customer's name; under 60 words.
4. **Eval** (`internal/eval`) scores every model against the rules-only baseline on accuracy, note quality, cost and latency, and returns keep or retire with the reasons.

The model never acts on an order. The worst a wrong answer can do is send a designer a suggestion that they decline.

## Labelling rules

The original cases were labelled by hand before any model ran. Generated cases use the comment bank's hand-labelled effects and the artwork's dpi. Both follow these rules:

1. `ask_customer` when the comment refers to a file that is not attached, asks for a size, shape or material different from the order, gives instructions that contradict each other, or there is no artwork. This wins over the other two.
2. `recreate_artwork` when the art is under 150 dpi at the ordered size. The Guide says to always try to recreate low-resolution artwork rather than ask for a new file.
3. `ready_for_proof` otherwise, including fixes a designer can make alone: remove a background, thicken lines, move art off the edge, edit text the customer wrote out.

The artwork is synthetic and drawn by `cmd/genfixtures`, so the set is reproducible and contains no customer data. CI regenerates it and fails if a byte changes.

## Run it

```sh
go test ./...                                   # no network, no key
go run ./cmd/proofqc eval                       # rules baseline only
go run ./cmd/proofqc eval -tools                 # tool agent; Haiku by default, requires a key

export OPENROUTER_API_KEY=...
go run ./cmd/proofqc check -model x-ai/grok-4.7 -product die_cut -w 3 -h 3 \
  -customer "Brian Smith" -comment "Remove the white box please" art.png
go run ./cmd/proofqc eval -max-usd 2 \
  -models anthropic/claude-sonnet-5.5,openai/gpt-6.1-terra,x-ai/grok-4.7,qwen/qwen3-vl-30b-a3b-instruct
```

To evaluate the tool agent on all 106 cases, run `go run ./cmd/proofqc eval -tools -models anthropic/claude-haiku-5.5 -workers 1 -max-usd 0.10 -out results/tools`. Add `-limit 3` for a small smoke run. Limited runs use `.limit3` filenames so they cannot replace full-dataset outcomes or reports. It reports decision accuracy plus measured-before-route, override consistency and call bounds. Use `-reuse` to rescore saved tool runs without a key. A partial run is marked incomplete and cannot pass policy.

`-max-usd` stops new calls once the spend passes the cap. The client never retries on its own, so every billed attempt is counted.

## Run the demo locally

```sh
docker run -d --rm --name proofqc-pg -p 5432:5432 \
  -e POSTGRES_USER=proofqc -e POSTGRES_PASSWORD=proofqc -e POSTGRES_DB=proofqc postgres:17
export DATABASE_URL='postgres://proofqc:proofqc@localhost:5432/proofqc?sslmode=disable'
export OPENROUTER_API_KEY=...
DAILY_USD_CAP=1 go run ./cmd/proofqc-server       # http://localhost:8080
go test -race ./...                              # store and API tests use the same DATABASE_URL
```

The server applies its migration at startup. `PORT` defaults to 8080 and `DAILY_USD_CAP` to 3 (US dollars per UTC day; `0` blocks every model call). The page is built from `web/src` with `corepack pnpm build` in `web/`; the built `web/dist/app.js` is committed, so `go run` and `docker build` need no Node.

## How I would roll this out at Sticker Mule

1. **Baseline, one week.** From existing order data, per new order: time from checkout to first proof, revision rounds before approval, share of proofs approved first time, and delivery dates pushed because approval missed the cutoff.
2. **Shadow, one to two weeks.** Replace the simulated queue with the real order feed. The agent routes every new order; designers work as usual and nothing reaches a customer. Compare its routing with what designers did, and add every order a designer sent back to the labelled set.
3. **Turn it on for one product line**, for example die-cut stickers, only if shadow routing passes the gate: missed issues below an agreed rate, since each one costs a customer an extra round.
4. **Keep a kill switch.** If revision rounds per order or pushed delivery dates rise against the baseline for two days, route that line back to designers and retire the version. Prompt and model changes stay gated on the labelled set in CI, and the fleet keeps a challenger in shadow.

## License

[PolyForm Noncommercial 1.0.0](LICENSE). You may read, run and evaluate the code; commercial use needs written permission.
