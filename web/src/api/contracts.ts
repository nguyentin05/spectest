import type {
  HistoryChange,
  HistoryRequirementsTrigger,
  HistoryResponse,
  HistoryUpdate,
  LLMConfig,
  MarkdownResponse,
  PlanResponse,
  PlanScenario,
  PlanSetup,
  PlanStats,
  PlanStep,
  PlanSeed,
  WorkspaceResponse,
} from "./types";

export type RawConfig = {
  config_file: string;
  default_provider: string;
  active_model: string;
  endpoint_url: string;
  api_key_masked: string;
  has_api_key: boolean;
};

export function toLLMConfig(raw: RawConfig): LLMConfig {
  return {
    provider: raw.default_provider,
    model: raw.active_model,
    endpoint: raw.endpoint_url,
    has_api_key: raw.has_api_key,
    api_key_masked: raw.api_key_masked,
  };
}

export type RawMarkdown = {
  path?: string;
  title: string;
  sections: string[];
  table_count: number;
  line_count: number;
  raw_content: string;
};

export function toMarkdownResponse(raw: RawMarkdown, fallbackPath: string): MarkdownResponse {
  const path = raw.path || fallbackPath;
  return {
    path,
    file_name: path.split(/[\\/]/).pop() ?? path,
    content: raw.raw_content,
  };
}

// Go omits optional YAML blocks (omitempty) and encodes nil slices as null,
// so hand-written suites routinely miss keys the UI expects to be present.
export type RawPlanScenario = Partial<Omit<PlanScenario, "steps">> & {
  steps?: PlanStep[] | null;
};

export type RawPlan = {
  path?: string;
  version?: number;
  servers?: string[] | null;
  apis?: string[] | null;
  analysis?: string;
  seed?: Record<string, PlanSeed> | null;
  setup?: Record<string, Partial<PlanSetup>> | null;
  scenarios?: RawPlanScenario[] | null;
  stats?: Partial<PlanStats> | null;
};

export function toPlanResponse(raw: RawPlan): PlanResponse {
  return {
    path: raw.path ?? "",
    version: raw.version ?? 0,
    servers: raw.servers ?? [],
    apis: raw.apis ?? [],
    analysis: raw.analysis ?? "",
    seed: raw.seed ?? {},
    setup: Object.fromEntries(
      Object.entries(raw.setup ?? {}).map(([id, setup]) => [
        id,
        { ...setup, steps: setup.steps ?? [] },
      ]),
    ),
    scenarios: (raw.scenarios ?? []).map((scenario) => ({
      id: scenario.id ?? "",
      name: scenario.name ?? "",
      setup: scenario.setup ?? [],
      steps: scenario.steps ?? [],
      locked: scenario.locked ?? false,
      skip: scenario.skip ?? "",
    })),
    stats: {
      total_scenarios: raw.stats?.total_scenarios ?? 0,
      total_setups: raw.stats?.total_setups ?? 0,
      total_steps: raw.stats?.total_steps ?? 0,
      locked_scenarios: raw.stats?.locked_scenarios ?? 0,
      skipped_scenarios: raw.stats?.skipped_scenarios ?? 0,
    },
  };
}

export type RawHistoryOpenApiTrigger = {
  added?: string[] | null;
  changed?: string[] | null;
  removed?: string[] | null;
  restored?: string[] | null;
};

export type RawHistoryTrigger = {
  openapi?: RawHistoryOpenApiTrigger | null;
  requirements?: Partial<HistoryRequirementsTrigger> | null;
};

export type RawHistoryCreate = {
  seed?: Record<string, unknown> | null;
  setup?: Record<string, unknown> | null;
  scenarios?: Array<{ id?: string; name?: string }> | null;
};

export type RawHistoryChange = Partial<HistoryChange>;

export type RawHistoryUpdate = {
  n?: number;
  at?: string;
  by?: string;
  note?: string;
  trigger?: RawHistoryTrigger | null;
  create?: RawHistoryCreate | null;
  change?: RawHistoryChange[] | null;
  after_hash?: string;
};

export type RawHistory = {
  path?: string;
  base?: {
    at?: string;
    by?: string;
    after_hash?: string;
    spectest?: {
      servers?: string[] | null;
      apis?: string[] | null;
      scenarios?: unknown[] | null;
    } | null;
  } | null;
  updates?: RawHistoryUpdate[] | null;
};

function toHistoryUpdate(raw: RawHistoryUpdate): HistoryUpdate {
  const openapi = raw.trigger?.openapi;
  const requirements = raw.trigger?.requirements;
  const create = raw.create ?? {};

  return {
    n: raw.n ?? 0,
    at: raw.at ?? "",
    by: raw.by ?? "",
    note: raw.note ?? "",
    trigger: {
      ...(openapi
        ? {
            openapi: {
              added: openapi.added ?? [],
              changed: openapi.changed ?? [],
              removed: openapi.removed ?? [],
              restored: openapi.restored ?? [],
            },
          }
        : {}),
      ...(requirements
        ? {
            requirements: {
              before_hash: requirements.before_hash ?? "",
              after_hash: requirements.after_hash ?? "",
            },
          }
        : {}),
    },
    create: {
      seed_added: Object.keys(create.seed ?? {}),
      setup_added: Object.keys(create.setup ?? {}),
      scenarios_added: (create.scenarios ?? [])
        .map((scenario) => scenario.id ?? scenario.name ?? "")
        .filter(Boolean),
    },
    change: (raw.change ?? []).map((change) => ({
      target: change.target ?? "",
      id: change.id ?? "",
      reason: change.reason ?? "",
      before: change.before,
      after: change.after,
    })),
    after_hash: raw.after_hash ?? "",
  };
}

// Go's workspace walk classifies by file name and misses `spectest.log.yaml`,
// so the evolution log shows up as "other" and never links to /history.
export function toWorkspaceResponse(raw: WorkspaceResponse): WorkspaceResponse {
  return {
    ...raw,
    items: raw.items.map((item) => ({
      ...item,
      type: /(^|[\\/])spectest\.log\.ya?ml$/i.test(item.name) ? "history" : item.type,
    })),
  };
}

export function toHistoryResponse(raw: RawHistory): HistoryResponse {
  const base = raw.base ?? {};
  const suite = base.spectest ?? {};

  return {
    base: {
      at: base.at ?? "",
      by: base.by ?? "",
      after_hash: base.after_hash ?? "",
      spectest_summary: {
        servers: suite.servers ?? [],
        apis_count: (suite.apis ?? []).length,
        scenarios_count: (suite.scenarios ?? []).length,
      },
    },
    updates: (raw.updates ?? []).map(toHistoryUpdate),
  };
}
