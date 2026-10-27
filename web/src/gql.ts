// Types mirror internal/api/schema.graphql.
export type Decision = "ready_for_proof" | "recreate_artwork" | "ask_customer";

export interface Review {
  decision: Decision;
  reasons: string[];
  designerNote: string;
  customerNote: string;
  orderChanges: string[];
  preflight: { widthPx: number; heightPx: number; effectiveDpi: number; findings: { flag: string; message: string }[] };
  lint: { rule: string; detail: string }[];
  model: string;
  costUsd: number;
  latencyMs: number;
  createdAt: string;
}

export interface ModelScore {
  model: string; variant: string; runs: number; cases: number;
  accuracy: number; accuracyLow: number; accuracyHigh: number;
  lintPass: number; costPerProof: number; p50Ms: number; keep: boolean; verdict: string;
}

export interface Sample {
  id: string; imageUrl: string; customer: string; product: string;
  widthIn: number; heightIn: number; comment: string; want: string; why: string;
}

export interface ReviewInput {
  customer: string; product: string; widthIn: number; heightIn: number; comment: string; model: string;
  imageBase64?: string; sampleId?: string;
}

/** GraphQLError carries the server's message and its extensions.code. */
export class GraphQLError extends Error {
  constructor(message: string, readonly code: string) {
    super(message);
  }
}

export async function gql<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  let res: Response;
  try {
    res = await fetch("/graphql", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query, variables }),
    });
  } catch {
    throw new GraphQLError("The server could not be reached. Check your connection and try again.", "NETWORK");
  }
  let body: { data?: T | null; errors?: { message: string; extensions?: { code?: string } }[] };
  try {
    body = await res.json();
  } catch {
    throw new GraphQLError(`The server answered ${res.status} without JSON.`, "NETWORK");
  }
  const first = body.errors?.[0];
  if (first) throw new GraphQLError(first.message, first.extensions?.code ?? "ERROR");
  if (!body.data) throw new GraphQLError("The server returned no data.", "ERROR");
  return body.data;
}

const reviewFields = `decision reasons designerNote customerNote orderChanges
  preflight { widthPx heightPx effectiveDpi findings { flag message } }
  lint { rule detail } model costUsd latencyMs createdAt`;

export const queries = {
  page: `{ samples { id imageUrl customer product widthIn heightIn comment want why }
    leaderboard { model variant runs cases accuracy accuracyLow accuracyHigh lintPass costPerProof p50Ms keep verdict }
    recentReviews(limit: 10) { ${reviewFields} }
    spentToday }`,
  refresh: `{ recentReviews(limit: 10) { ${reviewFields} } spentToday }`,
  review: `mutation($in: ReviewInput!) { reviewArtwork(input: $in) { ${reviewFields} } }`,
};
