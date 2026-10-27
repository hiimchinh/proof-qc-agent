import { h } from "./dom";
import type { Decision, ModelScore, Review } from "./gql";

const decisionLabel: Record<Decision, string> = {
  ready_for_proof: "Ready for proof",
  recreate_artwork: "Recreate the artwork",
  ask_customer: "Ask the customer",
};

export const usd = (n: number) => `$${n.toFixed(n < 0.01 ? 4 : 2)}`;
const pct = (n: number) => `${Math.round(n * 100)}%`;
const seconds = (ms: number) => `${(ms / 1000).toFixed(1)} s`;

function list(items: string[], empty: string): HTMLElement {
  if (items.length === 0) return h("p", { class: "hint" }, empty);
  return h("ul", {}, ...items.map((i) => h("li", {}, i)));
}

export function renderReview(r: Review): Node[] {
  const pre = r.preflight;
  return [
    h("p", { class: `decision ${r.decision}` }, decisionLabel[r.decision] ?? r.decision),
    h("h3", {}, "Note to the customer"),
    h("p", { class: "note" }, r.customerNote),
    h("h3", {}, "Note to the designer"),
    h("p", { class: "note" }, r.designerNote || "Nothing to fix."),
    h("h3", {}, "Order changes (decided by code)"),
    list(r.orderChanges, "None: the comment fits the order."),
    h("h3", {}, "Preflight"),
    h("dl", { class: "facts" },
      h("dt", {}, "Effective dpi"), h("dd", {}, `${pre.effectiveDpi} at ${pre.widthPx}×${pre.heightPx} px`),
      h("dt", {}, "Model"), h("dd", {}, r.model),
      h("dt", {}, "Cost"), h("dd", {}, usd(r.costUsd)),
      h("dt", {}, "Latency"), h("dd", {}, seconds(r.latencyMs)),
    ),
    list(pre.findings.map((f) => `${f.flag}: ${f.message}`), "No preflight findings."),
    h("h3", {}, "Why"),
    list(r.reasons, "No reasons given."),
    ...(r.lint.length ? [h("h3", {}, "Support principles still broken"), list(r.lint.map((l) => `${l.rule}: ${l.detail}`), "")] : []),
  ];
}

export function renderLeaderboard(table: HTMLTableElement, rows: ModelScore[]): void {
  if (rows.length === 0) {
    table.replaceChildren(h("caption", {}, "No saved eval runs in this build yet."));
    return;
  }
  const head = h("tr", {},
    h("th", { scope: "col" }, "Reviewer"), h("th", { scope: "col" }, "Rules"),
    h("th", { scope: "col", class: "num" }, "Accuracy"), h("th", { scope: "col", class: "num" }, "Notes pass"),
    h("th", { scope: "col", class: "num" }, "$ / proof"), h("th", { scope: "col", class: "num" }, "p50"),
    h("th", { scope: "col" }, "Verdict"));
  const body = rows.map((s) => {
    const acc = s.runs > 1 ? `${pct(s.accuracy)} (${pct(s.accuracyLow)}–${pct(s.accuracyHigh)})` : pct(s.accuracy);
    return h("tr", { class: s.keep ? "keep" : "" },
      h("th", { scope: "row" }, s.model), h("td", {}, s.variant),
      h("td", { class: "num", title: `${s.runs} run(s), ${s.cases} answers` }, acc),
      h("td", { class: "num" }, pct(s.lintPass)), h("td", { class: "num" }, usd(s.costPerProof)),
      h("td", { class: "num" }, seconds(s.p50Ms)), h("td", {}, s.verdict));
  });
  table.replaceChildren(h("thead", {}, head), h("tbody", {}, ...body));
}

export function renderRecent(ol: HTMLOListElement, rows: Review[]): void {
  if (rows.length === 0) {
    ol.replaceChildren(h("li", { class: "hint" }, "No reviews yet today. Be the first."));
    return;
  }
  ol.replaceChildren(...rows.map((r) => h("li", {},
    h("span", { class: "tag" }, decisionLabel[r.decision] ?? r.decision),
    ` · ${r.model} · ${usd(r.costUsd)} · ${seconds(r.latencyMs)} · `,
    h("time", { datetime: r.createdAt }, new Date(r.createdAt).toLocaleString()),
    h("p", { class: "note" }, r.customerNote))));
}
