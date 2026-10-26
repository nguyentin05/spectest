import type { PlanSetup } from "../../api/types";
import { JsonTree } from "../data/JsonTree";
import { Badge } from "../ui/Badge";
import { Card } from "../ui/Card";
import { CopyButton } from "../ui/CopyButton";
import { StepList } from "./StepList";

type SetupCardProps = {
  id: string;
  setup: PlanSetup;
};

export function SetupCard({ id, setup }: SetupCardProps) {
  return (
    <Card
      title={<code>{id}</code>}
      subtitle="Setup"
      actions={<CopyButton value={id} label={`ID setup ${id}`} />}
    >
      {setup.use && setup.use.length > 0 && (
        <div className="st-setup__chain">
          <span className="st-suite__hint">Kế thừa:</span>
          {setup.use.map((name) => (
            <Badge key={name} tone="brand">
              {name}
            </Badge>
          ))}
        </div>
      )}

      <div className="st-setup__vars">
        <p className="st-step__label">Biến</p>
        {setup.vars && Object.keys(setup.vars).length > 0 ? (
          <JsonTree value={setup.vars} name="vars" />
        ) : (
          <p className="st-suite__hint">Không có biến.</p>
        )}
      </div>

      <StepList steps={setup.steps ?? []} setupVars={setup.vars} />
    </Card>
  );
}
