import { byId, h } from "./dom";
import { gql } from "./gql";
import { deskQuery, type Desk, type DeskOrder, type AgentRun, type FleetScore } from "./desk-gql";
import { usd } from "./render";

const label = (v: string) => v.replaceAll("_", " ");
const pct = (n: number) => `${Math.round(n * 100)}%`;
const elapsed = (ms: number) => `${(ms / 1000).toFixed(1)} s`;
let previous = "";

function detail(key: string, title: string, ...children: Node[]): HTMLDetailsElement {
  return h("details", { "data-key": key }, h("summary", {}, title), ...children);
}
// A provider outage (rate limit, account credit) is not the model's failure and is not scored.
const notScored = (run: AgentRun) => run.error !== "" && !run.scored;
function runView(order: DeskOrder, run: AgentRun): Node {
  const outcome = run.error ? "Failed" : run.correct ? "Matches label" : "Misses label";
  const title = notScored(run) ? `${run.role} not scored: ${run.errorCause}` :
    run.error ? `${run.role} call failed: ${run.errorCause}` : `${run.role} · ${label(run.decision || "no decision")} · ${outcome}`;
  return detail(`${order.id}-${run.role}`, title,
    h("p", { class: "hint" }, `${run.model} · ${usd(run.costUsd)} · ${elapsed(run.latencyMs)} · Notes ${run.notesPass ? "pass" : "fail"}${run.overriddenByRules ? " · Code overrode routing" : ""}`),
    ...(run.error ? [h("p", { class: run.role === "challenger" || notScored(run) ? "hint" : "error" }, run.error)] : []),
    h("p", { class: "note" }, run.customerNote || "No customer note."),
    h("p", {}, `Designer: ${run.designerNote || "No designer changes."}`),
    h("ol", { class: "tool-trace" }, ...run.trace.map((s) => h("li", {},
      h("strong", {}, s.tool), ...(s.overridden ? [h("span", { class: "badge" }, "Code override")] : []),
      h("pre", {}, s.args), h("p", {}, s.result)))),
  );
}
function orderView(o: DeskOrder): HTMLElement {
  const champion = o.runs.find((r) => r.role === "champion");
  const state = champion ? (notScored(champion) ? "Not scored" : champion.error ? "Failed" : champion.correct ? "Match" : "Miss") : o.status;
  return h("li", { class: "desk-order" },
    h("div", { class: "order-heading" }, h("h3", {}, `#${o.id} · ${o.customer}`),
      h("span", { class: `badge ${champion?.correct ? "match" : ""}` }, state)),
    h("p", { class: "hint" }, `${o.product.replaceAll("_", " ")} · ${o.widthIn} × ${o.heightIn} in · ${o.caseId}`),
    ...(o.comment ? [h("p", { class: "order-comment" }, o.comment)] : []),
    h("p", {}, `Decision: ${label(champion?.decision || "pending")} · Label: ${label(o.expected)}`),
    ...o.runs.map((r) => runView(o, r)),
    ...(o.runs.length ? [] : [h("p", { class: "hint" }, `Order ${o.status}. No model run yet.`)]),
    h("time", { datetime: o.createdAt, class: "hint" }, new Date(o.createdAt).toLocaleString()));
}
function scoreView(s: FleetScore): HTMLElement {
  const metric = (name: string, value: string) => [h("dt", {}, name), h("dd", {}, value)];
  return h("article", { class: "fleet-model" },
    h("p", { class: "eyebrow" }, s.role), h("h3", {}, s.model || "No active model"),
    h("dl", { class: "score-metrics" },
      ...metric("Orders", String(s.n)), ...metric("Accuracy", s.n ? pct(s.accuracy) : "—"),
      ...metric("Notes pass", s.n ? pct(s.notesPassRate) : "—"),
      ...metric("$ / order", s.n ? usd(s.costPerOrder) : "—"),
      ...metric("p50 latency", s.n ? elapsed(s.p50Ms) : "—")),
    h("p", { class: "hint" }, s.n < 30 ? `${30 - s.n} more orders before quality evaluation; 5 consecutive failed calls retire a model early.` : "Eligible for policy evaluation."));
}
function kpiViews(d: Desk): HTMLElement[] {
  const champion = d.scoreboard.find((s) => s.model === d.champion);
  const measured = champion && champion.n > 0;
  const retired = d.scoreboard.filter((s) => s.role === "retired").length;
  const kpi = (name: string, value: string, note: string) => h("div", { class: "kpi" }, h("dt", {}, name), h("dd", {}, value), h("p", { class: "hint" }, note));
  const [vendor, model] = d.champion.includes("/") ? d.champion.split("/", 2) : ["", d.champion];
  return [
    kpi("Champion", model || "None", d.champion ? `${vendor ? `${vendor} · ` : ""}acts on every order` : "Orders wait for a model"),
    kpi("Accuracy", measured ? pct(champion.accuracy) : "—", measured ? `over ${champion.n} orders` : "No scored orders yet"),
    kpi("Cost per order", measured ? usd(champion.costPerOrder) : "—", measured ? `p50 ${elapsed(champion.p50Ms)}` : "No scored orders yet"),
    kpi("Models measured", String(d.scoreboard.length), `${retired} retired by the policy`),
  ];
}
function render(d: Desk): void {
  byId("desk-kpis").replaceChildren(...kpiViews(d));
  const open = new Set(Array.from(document.querySelectorAll<HTMLDetailsElement>("#live-desk details[open]")).map((el) => el.dataset.key));
  const focused = document.activeElement?.closest<HTMLDetailsElement>("details")?.dataset.key;
  byId("desk-orders").replaceChildren(...(d.orders.length ? d.orders.map(orderView) : [
    h("li", { class: "empty-state" }, "Waiting for the first scheduled tick. Orders will appear here automatically."),
  ]));
  const scores = [...d.scoreboard];
  for (const [role, model] of [["champion", d.champion], ["challenger", d.challenger]] as const) {
    if (model && !scores.some((s) => s.model === model)) scores.push({ model, role, n: 0, accuracy: 0, notesPassRate: 0, costPerOrder: 0, p50Ms: 0 });
  }
  byId("fleet-scoreboard").replaceChildren(...scores.map(scoreView));
  byId("fleet-queue").textContent = d.challengerQueue.length ? `Next to trial: ${d.challengerQueue.join(" → ")}` : "Queue empty";
  byId("fleet-events").replaceChildren(...(d.events.length ? d.events.map((e) => h("li", {},
    h("span", { class: "hint" }, `${e.kind} · ${e.model}`), h("p", { class: "fleet-reason" }, e.reason),
    detail(`event-${e.id}`, "Decision evidence", h("pre", {}, e.numbers)),
    h("time", { datetime: e.createdAt, class: "hint" }, new Date(e.createdAt).toLocaleString()))) : [h("li", { class: "hint" }, "No fleet events yet.")]));
  for (const el of document.querySelectorAll<HTMLDetailsElement>("#live-desk details")) {
    if (open.has(el.dataset.key)) el.open = true;
    if (focused && el.dataset.key === focused) el.querySelector("summary")?.focus();
  }
  const capped = d.spentToday >= d.dailyCapUsd;
  byId("desk-budget").textContent = `${usd(d.spentToday)} / ${usd(d.dailyCapUsd)} today (UTC)${capped ? " · Budget spent; new orders stay queued." : ""}`;
  byId("desk-state").textContent = !d.champion && !d.challenger ? "Fleet empty · Orders stay queued" :
    !d.champion ? "Champion retired · Shadow evaluation only" :
    !d.challenger ? "Champion acts · Challenger seat empty" : "Champion acts · Challenger shadows";
}
export function startDesk(): void {
  let loading = false;
  const refresh = async () => {
    if (loading || document.hidden) return;
    loading = true;
    try {
      const { desk } = await gql<{ desk: Desk }>(deskQuery);
      const next = JSON.stringify(desk);
      if (next !== previous) { render(desk); previous = next; }
      byId("desk-updated").textContent = `Updated ${new Date().toLocaleTimeString()} · Refreshes every 10 s`;
    } catch {
      byId("desk-updated").textContent = "Desk could not refresh. Retrying in 10 s; displayed data may be stale.";
    } finally { loading = false; }
  };
  void refresh();
  window.setInterval(() => { void refresh(); }, 10_000);
  document.addEventListener("visibilitychange", () => { if (!document.hidden) void refresh(); });
}
