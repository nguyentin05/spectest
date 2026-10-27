import type { RawConfig, RawHistory } from "../api/contracts";
import type {
  PlanResponse,
  RunReport,
  RunStreamEvent,
  WorkspaceItem,
} from "../api/types";
import {
  createConfigFixture,
  createHistoryFixture,
  createLogsFixture,
  createMarkdownFixtures,
  createPlanFixture,
  createWorkspaceFixture,
} from "./fixtures";

export type MockRun = {
  id: string;
  status: "running" | "completed" | "cancelled";
  suitePath: string;
  targetUrl: string;
  startedAt: number;
  events: RunStreamEvent[];
  subscribers: Set<(event: RunStreamEvent) => void>;
};

export type MockState = {
  config: RawConfig;
  workspace: WorkspaceItem[];
  plan: PlanResponse;
  history: RawHistory;
  markdown: Record<string, string>;
  logs: string[];
  run: MockRun | null;
  report: RunReport | null;
  runDelayMs: number;
};

function createState(): MockState {
  return {
    config: createConfigFixture(),
    workspace: createWorkspaceFixture(),
    plan: createPlanFixture(),
    history: createHistoryFixture(),
    markdown: createMarkdownFixtures(),
    logs: createLogsFixture(),
    run: null,
    report: null,
    runDelayMs: 250,
  };
}

export const mockState: MockState = createState();

export function resetMockStore(): void {
  Object.assign(mockState, createState());
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

const MAX_LOGS = 200;

export function appendLog(message: string): void {
  const now = new Date();
  const stamp = [now.getHours(), now.getMinutes(), now.getSeconds()]
    .map((part) => String(part).padStart(2, "0"))
    .join(":");
  mockState.logs.push(`[${stamp}] ${message}`);
  if (mockState.logs.length > MAX_LOGS) {
    mockState.logs.splice(0, mockState.logs.length - MAX_LOGS);
  }
}
