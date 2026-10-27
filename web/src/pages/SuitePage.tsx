import { useState, type ReactNode } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { PlanResponse } from "../api/types";
import { api } from "../api/endpoints";
import { ApiError, isMissingEndpoint } from "../api/errors";
import { useApi } from "../hooks/useApi";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { JsonTree } from "../components/data/JsonTree";
import { CapabilityNotice } from "../components/ui/CapabilityNotice";
import { Checkbox } from "../components/ui/Checkbox";
import { EmptyState } from "../components/ui/EmptyState";
import { ErrorState } from "../components/ui/ErrorState";
import { LoadingBlock } from "../components/ui/LoadingBlock";
import { SearchInput } from "../components/ui/SearchInput";
import { Tabs } from "../components/ui/Tabs";
import { PlanHeader } from "../components/suite/PlanHeader";
import { ScenarioCard } from "../components/suite/ScenarioCard";
import { SetupCard } from "../components/suite/SetupCard";

function ScenarioList({ plan }: { plan: PlanResponse }) {
  const [search, setSearch] = useState("");
  const [lockedOnly, setLockedOnly] = useState(false);
  const query = useDebouncedValue(search.trim().toLowerCase());

  const scenarios = (plan.scenarios ?? []).filter((scenario) => {
    if (lockedOnly && !scenario.locked) return false;
    if (!query) return true;
    return (
      scenario.name.toLowerCase().includes(query) || scenario.id.toLowerCase().includes(query)
    );
  });

  return (
    <div className="st-suite">
      <div className="st-suite__filters">
        <SearchInput
          aria-label="Tìm kịch bản"
          placeholder="Tìm theo tên hoặc ID…"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <Checkbox
          checked={lockedOnly}
          onChange={(event) => setLockedOnly(event.target.checked)}
        >
          Chỉ kịch bản bị khóa
        </Checkbox>
        <span className="st-suite__hint">{scenarios.length} kịch bản</span>
      </div>

      {scenarios.length === 0 ? (
        <p className="st-suite__empty">Không có kịch bản phù hợp.</p>
      ) : (
        <div className="st-suite__list">
          {scenarios.map((scenario) => (
            <ScenarioCard key={scenario.id} scenario={scenario} />
          ))}
        </div>
      )}
    </div>
  );
}

function SuiteContent({ plan }: { plan: PlanResponse }) {
  const setupEntries = Object.entries(plan.setup ?? {});

  return (
    <>
      <PlanHeader plan={plan} />
      <Tabs
        label="Nội dung suite"
        items={[
          {
            id: "scenarios",
            label: "Kịch bản",
            content: <ScenarioList plan={plan} />,
          },
          {
            id: "setup",
            label: "Setup",
            content:
              setupEntries.length === 0 ? (
                <p className="st-suite__empty">Chưa có setup.</p>
              ) : (
                <div className="st-suite__setups">
                  {setupEntries.map(([id, setup]) => (
                    <SetupCard key={id} id={id} setup={setup} />
                  ))}
                </div>
              ),
          },
          {
            id: "data",
            label: "Dữ liệu",
            content: <JsonTree value={plan} name="Dữ liệu suite" />,
          },
        ]}
      />
    </>
  );
}

export function SuitePage() {
  const [params] = useSearchParams();
  const path = params.get("path") ?? undefined;
  const { data, error, loading, reload } = useApi(() => api.plan(path), [path]);

  let content: ReactNode = null;

  if (loading) {
    content = <LoadingBlock label="Đang đọc spectest.yaml…" rows={3} />;
  } else if (error instanceof ApiError && error.status === 404) {
    content = (
      <EmptyState
        title="Chưa có spectest.yaml"
        description="Hãy sinh kịch bản đầu tiên hoặc chọn file khác."
        action={
          <Link className="st-btn st-btn--secondary" to="/generate">
            Sinh kịch bản đầu tiên
          </Link>
        }
      />
    );
  } else if (error) {
    content = isMissingEndpoint(error) ? (
      <CapabilityNotice feature="kịch bản" error={error} />
    ) : (
      <ErrorState error={error} onRetry={reload} />
    );
  } else if (data) {
    content = <SuiteContent plan={data} />;
  }

  return (
    <section className="st-page">
      <h1>Kịch bản</h1>
      {content}
    </section>
  );
}
