export type HealthResponse = {
  status: string;
  time: string;
  work_dir: string;
};

export type LogsResponse = {
  logs: string[];
};

export type LLMConfig = {
  provider: string;
  model: string;
  endpoint: string;
  has_api_key: boolean;
  api_key_masked: string;
};

export type ConfigUpdateRequest = {
  provider: string;
  endpoint: string;
  model: string;
  api_key?: string;
};

export type ConfigTestRequest = {
  provider: string;
  endpoint: string;
  model: string;
  api_key?: string;
};

export type ConfigTestResponse = {
  success: boolean;
  provider?: string;
  model?: string;
  message: string;
  latency_ms?: number;
};

export type HealthcheckRequest = {
  target_url: string;
  timeout_seconds?: number;
};

export type HealthcheckResponse = {
  ready: boolean;
  target_url: string;
  latency_ms: number;
  message: string;
};

export type WorkspaceItemType = "openapi" | "prd" | "suite" | "history" | "matrix" | "other";

export type WorkspaceItem = {
  name: string;
  path: string;
  type: WorkspaceItemType;
  size: number;
};

export type WorkspaceResponse = {
  current_dir: string;
  items: WorkspaceItem[];
};

export type UploadResponse = {
  success: boolean;
  file_name: string;
  path: string;
  type: WorkspaceItemType;
  size: number;
};

export type UploadTextRequest = {
  file_name: string;
  content: string;
};

export type PlanStep = {
  call: string;
  headers?: Record<string, string>;
  body?: unknown;
  expect: number;
  save?: Record<string, string>;
};

export type PlanSetup = {
  vars?: Record<string, string>;
  use?: string[];
  steps: PlanStep[];
};

export type PlanScenario = {
  id: string;
  name: string;
  setup: string[];
  steps: PlanStep[];
  locked: boolean;
  skip: string;
};

export type PlanStats = {
  total_scenarios: number;
  total_setups: number;
  total_steps: number;
  locked_scenarios: number;
  skipped_scenarios: number;
};

export type PlanSeed = {
  vars: Record<string, string>;
};

export type PlanResponse = {
  path: string;
  version: number;
  servers: string[];
  apis: string[];
  analysis: string;
  seed: Record<string, PlanSeed>;
  setup: Record<string, PlanSetup>;
  scenarios: PlanScenario[];
  stats: PlanStats;
};

export type HistoryOpenApiTrigger = {
  added: string[];
  changed: string[];
  removed: string[];
};

export type HistoryRequirementsTrigger = {
  before_hash: string;
  after_hash: string;
};

export type HistoryTrigger = {
  openapi?: HistoryOpenApiTrigger;
  requirements?: HistoryRequirementsTrigger;
};

export type HistoryCreate = {
  setup_added?: string[];
  scenarios_added?: string[];
};

export type HistoryChange = {
  target: string;
  id: string;
  reason: string;
  before: unknown;
  after: unknown;
};

export type HistoryUpdate = {
  n: number;
  at: string;
  by: string;
  note: string;
  trigger?: HistoryTrigger;
  create?: HistoryCreate;
  change?: HistoryChange[];
  after_hash: string;
};

export type HistoryBase = {
  at: string;
  by: string;
  after_hash: string;
  spectest_summary?: {
    servers: string[];
    apis_count: number;
    scenarios_count: number;
  };
};

export type HistoryResponse = {
  base: HistoryBase;
  updates: HistoryUpdate[];
};

export type GenerateRequest = {
  apis: { server: string; path: string }[];
  req_path: string;
  output_dir: string;
  dry_run: boolean;
};

export type GenerateResponse = {
  success: boolean;
  suite_path: string;
  log_path: string;
  servers: string[];
  apis_count: number;
  scenarios_generated: number;
  message: string;
};

export type GeneratePendingResponse = {
  success: boolean;
  status: "pending_synthesis";
  message: string;
};

export type RunRequest = {
  suite_path: string;
  target_url?: string;
  targets?: Record<string, string>;
  timeout_seconds?: number;
};

export type RunStartResponse = {
  run_id: string;
  status: string;
  message: string;
  target_url: string;
  suite_path: string;
};

export type RunStreamEvent =
  | {
      type: "connected";
      timestamp: string;
      message?: string;
      data?: { run_id: string; target_url?: string; suite_path?: string };
    }
  | { type: "idle"; timestamp: string; message?: string }
  | {
      type: "start";
      timestamp: string;
      message: string;
      data: {
        total_scenarios: number;
        servers: string[];
        targets?: Record<string, string>;
      };
    }
  | {
      type: "scenario_start";
      timestamp: string;
      message: string;
      data: { scenario_id: string; scenario_name: string };
    }
  | {
      type: "step_result";
      timestamp: string;
      data: {
        scenario_id: string;
        setup?: string;
        call: string;
        method: string;
        url: string;
        status_code: number;
        expect: number;
        latency_ms: number;
        passed: boolean;
        error?: string;
        saved_vars?: Record<string, unknown>;
        ai_diagnosis?: string;
      };
    }
  | {
      type: "scenario_complete";
      timestamp: string;
      data: {
        scenario_id: string;
        passed: boolean;
        skipped?: boolean;
        reason?: string;
      };
    }
  | {
      type: "complete";
      timestamp: string;
      message: string;
      data: { report: RunReport };
    }
  | { type: "error"; timestamp: string; message: string; data?: unknown }
  | { type: "cancelled"; timestamp: string; message?: string; data?: unknown };

export type RunReportStatus = "passed" | "failed" | "cancelled" | "running";

export type RunCaseResult = {
  scenario_id: string;
  step_id?: string;
  case_id?: string;
  call?: string;
  endpoint?: string;
  url?: string;
  method: string;
  status: "PASS" | "FAIL" | "SKIPPED" | "CANCELLED";
  http_status: number;
  expected_http_status: number;
  latency_ms: number;
  error?: string;
  saved_vars?: Record<string, unknown>;
};

export type RunReport = {
  id: string;
  target_url?: string;
  targets?: Record<string, string>;
  suite_path: string;
  status: RunReportStatus;
  total_scenarios: number;
  passed_scenarios: number;
  failed_scenarios: number;
  skipped_scenarios: number;
  total_cases: number;
  passed: number;
  failed: number;
  skipped: number;
  start_time: string;
  end_time: string;
  duration_ms: number;
  results: RunCaseResult[];
  selection_mode?: string;
  update_n?: number;
};

export type MarkdownResponse = {
  path: string;
  file_name: string;
  content: string;
};

export type ActionResponse = {
  success: boolean;
  message: string;
};
