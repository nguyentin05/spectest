import { useId, useState } from "react";
import type { PlanScenario } from "../../api/types";
import { Badge } from "../ui/Badge";
import { CopyButton } from "../ui/CopyButton";
import { StepList } from "./StepList";

export function ScenarioCard({ scenario }: { scenario: PlanScenario }) {
  const [expanded, setExpanded] = useState(false);
  const panelId = useId();

  return (
    <article className={`st-scenario ${expanded ? "is-open" : ""}`.trim()}>
      <div className="st-scenario__head">
        <h3 className="st-scenario__title">
          <button
            type="button"
            className="st-scenario__toggle"
            aria-expanded={expanded}
            aria-controls={panelId}
            onClick={() => setExpanded((current) => !current)}
          >
            <span className="st-scenario__chevron" aria-hidden="true">
              ▸
            </span>
            <span className="st-scenario__name">{scenario.name}</span>
          </button>
        </h3>
        <div className="st-scenario__badges">
          {scenario.locked && <Badge tone="neutral">Đã khóa</Badge>}
          {scenario.skip && (
            <Badge tone="skip" title={scenario.skip}>
              Bỏ qua
            </Badge>
          )}
        </div>
      </div>

      <div className="st-scenario__meta">
        <code className="st-scenario__id">{scenario.id}</code>
        <CopyButton value={scenario.id} label={`ID ${scenario.id}`} />
        {(scenario.setup && scenario.setup.length > 0) ? (
          scenario.setup.map((name) => (
            <Badge key={name} tone="brand">
              {name}
            </Badge>
          ))
        ) : (
          <span className="st-suite__hint">Không dùng setup</span>
        )}
      </div>

      {expanded && (
        <div className="st-scenario__steps" id={panelId}>
          <StepList steps={scenario.steps} />
        </div>
      )}
    </article>
  );
}
