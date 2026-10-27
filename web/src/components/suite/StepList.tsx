import type { PlanStep } from "../../api/types";
import { JsonTree } from "../data/JsonTree";
import { MethodPill } from "../data/MethodPill";
import { Badge } from "../ui/Badge";

type StepListProps = {
  steps: PlanStep[];
  setupVars?: Record<string, unknown>;
};

export function StepList({ steps, setupVars }: StepListProps) {
  return (
    <div className="st-step-list">
      {setupVars && Object.keys(setupVars).length > 0 && (
        <div className="st-step-list__chain">
          <span className="st-suite__hint">Biến kế thừa:</span>
          {Object.keys(setupVars).map((name) => (
            <Badge key={name} mono>
              {name}
            </Badge>
          ))}
        </div>
      )}

      <ol className="st-step-list__items">
        {(steps ?? []).map((step, index) => {
          const method = (step.call ?? "").split(" ")[0] ?? "";

          return (
            <li key={`${step.call}-${index}`} className="st-step">
              <span className="st-step__index" aria-hidden="true">
                {index + 1}
              </span>
              <div className="st-step__body">
                <div className="st-step__head">
                  <MethodPill method={method} />
                  <code className="st-step__call">{step.call}</code>
                  <span className="st-step__expect">
                    <span className="st-suite__hint">Mong đợi</span>
                    <Badge mono>{step.expect}</Badge>
                  </span>
                </div>

                {step.headers && Object.keys(step.headers).length > 0 && (
                  <dl className="st-step__headers">
                    {Object.entries(step.headers).map(([key, value]) => (
                      <div key={key} className="st-step__header-row">
                        <dt>{key}</dt>
                        <dd>
                          <code>{value}</code>
                        </dd>
                      </div>
                    ))}
                  </dl>
                )}

                {step.body !== undefined && (
                  <div className="st-step__section">
                    <p className="st-step__label">Yêu cầu</p>
                    <JsonTree value={step.body} />
                  </div>
                )}

                {step.save && Object.keys(step.save).length > 0 && (
                  <div className="st-step__saves">
                    <span className="st-suite__hint">Lưu biến:</span>
                    {Object.entries(step.save).map(([key, value]) => (
                      <Badge key={key} tone="brand" mono>
                        {key} ← {value}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
