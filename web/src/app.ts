import { startDesk } from "./desk";
import { byId, h } from "./dom";
import { gql, GraphQLError, queries, type ModelScore, type Review, type ReviewInput, type Sample } from "./gql";
import { renderLeaderboard, renderRecent, renderReview, usd } from "./render";

const maxUploadBytes = 4 * 1024 * 1024;

const form = byId<HTMLFormElement>("review");
const fileInput = byId<HTMLInputElement>("file");
const choice = byId<HTMLParagraphElement>("artwork-choice");
const errorBox = byId<HTMLParagraphElement>("error");
const submit = byId<HTMLButtonElement>("submit");
const result = byId<HTMLElement>("result");
const field = (id: string) => byId<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>(id);

// The artwork is either a sample id or an uploaded file's base64, never both.
let artwork: { sampleId: string } | { imageBase64: string } | null = null;

function showSpent(usdToday: number): void {
  byId("spent").textContent = `Today's model spend: ${usd(usdToday)}`;
}

function pressOnly(button: HTMLButtonElement | null): void {
  for (const b of document.querySelectorAll<HTMLButtonElement>("#samples button")) {
    b.setAttribute("aria-pressed", String(b === button));
  }
}

function selectSample(s: Sample, button: HTMLButtonElement): void {
  pressOnly(button);
  artwork = { sampleId: s.id };
  fileInput.value = "";
  errorBox.textContent = "";
  field("customer").value = s.customer;
  field("product").value = s.product;
  field("width").value = String(s.widthIn);
  field("height").value = String(s.heightIn);
  field("comment").value = s.comment;
  choice.textContent = `Sample "${s.id}". Hand label: ${s.want.replaceAll("_", " ")} (${s.why}).`;
}

function renderSamples(samples: Sample[]): void {
  byId("samples").replaceChildren(...samples.map((s) => {
    const label = `${s.id}: ${s.product.replace("_", " ")} ${s.widthIn}×${s.heightIn} in${s.comment ? `, comment: ${s.comment}` : ""}`;
    const b = h("button", { type: "button", "aria-pressed": "false", title: label, "aria-label": label },
      h("img", { src: s.imageUrl, alt: "", loading: "lazy", width: "72", height: "72" }));
    b.addEventListener("click", () => selectSample(s, b));
    return b;
  }));
}

fileInput.addEventListener("change", () => {
  const file = fileInput.files?.[0];
  if (!file) return;
  if (!["image/png", "image/jpeg"].includes(file.type) || file.size > maxUploadBytes) {
    errorBox.textContent = "Choose a PNG or JPEG of at most 4 MB.";
    fileInput.value = "";
    return;
  }
  const reader = new FileReader();
  reader.onload = () => {
    const url = String(reader.result);
    artwork = { imageBase64: url.slice(url.indexOf(",") + 1) };
    pressOnly(null);
    errorBox.textContent = "";
    choice.textContent = `Upload "${file.name}" (${Math.round(file.size / 1024)} KB).`;
  };
  reader.onerror = () => {
    errorBox.textContent = "That file could not be read.";
  };
  reader.readAsDataURL(file);
});

function readInput(): ReviewInput | string {
  if (!artwork) return "Pick a sample or upload artwork first.";
  const customer = field("customer").value.trim();
  const widthIn = Number(field("width").value);
  const heightIn = Number(field("height").value);
  if (!customer) return "Enter the customer's name.";
  if (!(widthIn > 0 && heightIn > 0)) return "Width and height must be positive numbers of inches.";
  return {
    customer, widthIn, heightIn, ...artwork,
    product: field("product").value,
    comment: field("comment").value.trim(),
    model: field("model").value,
  };
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const input = readInput();
  if (typeof input === "string") {
    errorBox.textContent = input;
    return;
  }
  errorBox.textContent = "";
  submit.disabled = true;
  submit.textContent = "Reviewing…";
  result.setAttribute("aria-busy", "true");
  try {
    const { reviewArtwork } = await gql<{ reviewArtwork: Review }>(queries.review, { in: input });
    const title = byId("result-title");
    result.replaceChildren(title, ...renderReview(reviewArtwork));
    title.focus();
    void refresh();
  } catch (err) {
    errorBox.textContent = err instanceof GraphQLError ? err.message : "Something went wrong. Try again.";
  } finally {
    submit.disabled = false;
    submit.textContent = "Review artwork";
    result.removeAttribute("aria-busy");
  }
});

async function refresh(): Promise<void> {
  try {
    const d = await gql<{ recentReviews: Review[]; spentToday: number }>(queries.refresh);
    renderRecent(byId("recent"), d.recentReviews);
    showSpent(d.spentToday);
  } catch {
    // The review itself succeeded; a stale list is not worth an error message.
  }
}

async function load(): Promise<void> {
  try {
    const d = await gql<{ samples: Sample[]; leaderboard: ModelScore[]; recentReviews: Review[]; spentToday: number }>(queries.page);
    renderSamples(d.samples);
    renderLeaderboard(byId("leaderboard"), d.leaderboard);
    renderRecent(byId("recent"), d.recentReviews);
    showSpent(d.spentToday);
  } catch (err) {
    errorBox.textContent = err instanceof GraphQLError ? err.message : "The page could not load its data.";
  }
}

startDesk();
void load();
