export interface ToolStep { tool: string; args: string; result: string; overridden: boolean }
export interface AgentRun {
  role: string; model: string; decision: string; designerNote: string; customerNote: string;
  correct: boolean; notesPass: boolean; costUsd: number; latencyMs: number;
  overriddenByRules: boolean; error: string; trace: ToolStep[];
}
export interface DeskOrder {
  id: string; caseId: string; customer: string; product: string; widthIn: number; heightIn: number;
  comment: string; expected: string; status: string; createdAt: string; runs: AgentRun[];
}
export interface FleetScore {
  model: string; role: string; n: number; accuracy: number; notesPassRate: number; costPerOrder: number; p50Ms: number;
}
export interface FleetEvent { id: string; kind: string; model: string; reason: string; numbers: string; createdAt: string }
export interface Desk {
  challengerQueue: string[];
  simulated: boolean; champion: string; challenger: string; spentToday: number; dailyCapUsd: number;
  orders: DeskOrder[]; scoreboard: FleetScore[]; events: FleetEvent[];
}
export const deskQuery = `{ desk {
  simulated champion challenger challengerQueue spentToday dailyCapUsd
  orders { id caseId customer product widthIn heightIn comment expected status createdAt
    runs { role model decision designerNote customerNote correct notesPass costUsd latencyMs overriddenByRules error
      trace { tool args result overridden } } }
  scoreboard { model role n accuracy notesPassRate costPerOrder p50Ms }
  events { id kind model reason numbers createdAt }
} }`;
