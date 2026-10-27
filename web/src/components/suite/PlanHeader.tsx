import type { PlanResponse } from "../../api/types";
import { MethodPill } from "../data/MethodPill";
import { StatGrid } from "../data/StatGrid";
import { Badge } from "../ui/Badge";

function splitApi(call: string): { method: string; path: string } {
  const [method, ...rest] = call.split(" ");
  return { method: method ?? "", path: rest.join(" ") };
}

export function PlanHeader({ plan }: { plan: PlanResponse }) {
  return (
    <header className="st-plan-header">
      <div className="st-plan-header__top">
        <div className="st-plan-header__id">
          <code className="st-plan-header__path">{plan.path}</code>
          <Badge>v{plan.version}</Badge>
          <Badge tone="brand">Chỉ đọc</Badge>
        </div>
        <div className="st-plan-header__servers">
          <span className="st-suite__hint">Máy chủ:</span>
          {(plan.servers ?? []).map((server) => (
            <Badge key={server} mono>
              {server}
            </Badge>
          ))}
        </div>
      </div>

      <StatGrid
        items={[
          { id: "scenarios", label: "Kịch bản", value: plan.stats?.total_scenarios ?? 0 },
          { id: "setups", label: "Setup", value: plan.stats?.total_setups ?? 0 },
          { id: "steps", label: "Bước", value: plan.stats?.total_steps ?? 0 },
          { id: "locked", label: "Khóa", value: plan.stats?.locked_scenarios ?? 0 },
        ]}
      />

      <div className="st-plan-header__cols">
        <div>
          <p className="st-step__label">API trong suite</p>
          <ul className="st-plan-header__apis">
            {(plan.apis ?? []).map((call) => {
              const { method, path } = splitApi(call);
              return (
                <li key={call}>
                  <MethodPill method={method} />
                  <code>{path}</code>
                </li>
              );
            })}
          </ul>
        </div>

        {plan.analysis && (
          <div className="st-plan-header__analysis" role="note">
            <p className="st-step__label">Phân tích</p>
            {plan.analysis
              .split("\n")
              .filter(Boolean)
              .map((line) => (
                <p key={line}>{line.replace(/^-\s*/, "")}</p>
              ))}
          </div>
        )}
      </div>
    </header>
  );
}
