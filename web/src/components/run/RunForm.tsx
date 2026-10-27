import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import type { HealthcheckResponse, RunRequest } from "../../api/types";
import { api } from "../../api/endpoints";
import { ApiError, isMissingEndpoint } from "../../api/errors";
import { useApi } from "../../hooks/useApi";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { CapabilityNotice } from "../ui/CapabilityNotice";
import { Card } from "../ui/Card";
import { Checkbox } from "../ui/Checkbox";
import { EmptyState } from "../ui/EmptyState";
import { ErrorState } from "../ui/ErrorState";
import { Field } from "../ui/Field";
import { LoadingBlock } from "../ui/LoadingBlock";

type TargetMode = "single" | "servers";

function isValidTarget(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}

function errorMessage(cause: unknown, fallback: string): string {
  return cause instanceof Error ? cause.message : fallback;
}

export function RunForm({ onStarted }: { onStarted: (runId: string) => void }) {
  const workspace = useApi(api.workspace);
  const plan = useApi(api.plan);

  const [suitePath, setSuitePath] = useState("");
  const [mode, setMode] = useState<TargetMode>("single");
  const [targetUrl, setTargetUrl] = useState("");
  const [perServer, setPerServer] = useState<Record<string, string>>({});
  const [timeoutSeconds, setTimeoutSeconds] = useState("30");
  const [triage, setTriage] = useState(false);
  const [onlyChanged, setOnlyChanged] = useState(false);
  const [logPath, setLogPath] = useState("");
  const [checking, setChecking] = useState(false);
  const [starting, setStarting] = useState(false);
  const [health, setHealth] = useState<HealthcheckResponse[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadError = workspace.error ?? plan.error;

  if (workspace.loading || plan.loading) {
    return <LoadingBlock label="Đang tải cấu hình chạy…" rows={3} />;
  }
  if (loadError) {
    if (plan.error instanceof ApiError && plan.error.status === 404) {
      return (
        <EmptyState
          title="Chưa có spectest.yaml"
          description="Hãy sinh kịch bản đầu tiên rồi quay lại chạy kiểm thử."
          action={
            <Link className="st-btn st-btn--secondary" to="/generate">
              Sinh kịch bản
            </Link>
          }
        />
      );
    }
    return isMissingEndpoint(loadError) ? (
      <CapabilityNotice feature="chạy kiểm thử" error={loadError} />
    ) : (
      <ErrorState
        error={loadError}
        onRetry={() => {
          workspace.reload();
          plan.reload();
        }}
      />
    );
  }

  const suiteItems = (workspace.data?.items ?? []).filter((item) => item.type === "suite");
  const servers = plan.data?.servers ?? [];
  const planPath = plan.data?.path ?? "";
  const selectedSuite = suitePath || planPath || suiteItems[0]?.name || "";

  function currentTargets(): string[] {
    if (mode === "single") return [targetUrl.trim()];
    return servers.map((server) => (perServer[server] ?? "").trim());
  }

  function validateTargets(targets: string[]): string | null {
    if (targets.length === 0 || targets.some((target) => !target)) {
      return "Nhập địa chỉ máy chủ đích.";
    }
    if (targets.some((target) => !isValidTarget(target))) {
      return "Địa chỉ máy chủ phải bắt đầu bằng http:// hoặc https://.";
    }
    return null;
  }

  async function checkTargets() {
    const targets = currentTargets();
    const invalid = validateTargets(targets);
    if (invalid) {
      setError(invalid);
      return;
    }

    setError(null);
    setHealth(null);
    setChecking(true);
    try {
      const results = await Promise.all(
        targets.map((target) =>
          api.healthcheck({ target_url: target, timeout_seconds: 5 }),
        ),
      );
      setHealth(results);
    } catch (cause) {
      setError(errorMessage(cause, "Kiểm tra máy chủ thất bại"));
    } finally {
      setChecking(false);
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const targets = currentTargets();
    const invalid = validateTargets(targets);
    if (invalid) {
      setError(invalid);
      return;
    }

    const seconds = Number(timeoutSeconds);
    if (!Number.isFinite(seconds) || seconds <= 0) {
      setError("Thời gian chờ phải lớn hơn 0.");
      return;
    }

    const body: RunRequest = {
      suite_path: selectedSuite,
      timeout_seconds: seconds,
      triage,
      only_changed: onlyChanged,
      ...(logPath.trim() ? { log_path: logPath.trim() } : {}),
      ...(mode === "single"
        ? { target_url: targets[0] }
        : {
            targets: Object.fromEntries(
              servers.map((server, index) => [server, targets[index] ?? ""]),
            ),
          }),
    };

    setError(null);
    setStarting(true);
    try {
      const response = await api.run(body);
      onStarted(response.run_id);
    } catch (cause) {
      setError(errorMessage(cause, "Không bắt đầu được phiên chạy"));
    } finally {
      setStarting(false);
    }
  }

  return (
    <Card
      title="Cấu hình lần chạy"
      subtitle="Chọn suite, máy chủ đích và thời gian chờ trước khi chạy."
    >
      <form className="st-run-form" onSubmit={(event) => void submit(event)}>
        <Field id="run-suite" label="Suite">
          <select
            value={selectedSuite}
            onChange={(event) => setSuitePath(event.target.value)}
          >
            {planPath && !suiteItems.some((item) => item.name === planPath) && (
              <option value={planPath}>{planPath}</option>
            )}
            {suiteItems.map((item) => (
              <option key={item.path} value={item.name}>
                {item.name}
              </option>
            ))}
          </select>
        </Field>

        <div
          className="st-run-form__modes"
          role="radiogroup"
          aria-label="Cách cấu hình máy chủ đích"
        >
          <label className="st-radio">
            <input
              type="radio"
              name="run-target-mode"
              checked={mode === "single"}
              onChange={() => setMode("single")}
            />
            <span>Một địa chỉ chung</span>
          </label>
          <label className="st-radio">
            <input
              type="radio"
              name="run-target-mode"
              checked={mode === "servers"}
              disabled={servers.length === 0}
              onChange={() => setMode("servers")}
            />
            <span>Riêng từng server</span>
          </label>
        </div>

        {mode === "single" ? (
          <Field id="run-target-url" label="Địa chỉ máy chủ đích">
            <input
              value={targetUrl}
              onChange={(event) => setTargetUrl(event.target.value)}
              placeholder="http://localhost:8080"
            />
          </Field>
        ) : (
          <div className="st-run-form__servers">
            {servers.map((server, index) => (
              <Field
                key={server}
                id={`run-target-${index}`}
                label={`Địa chỉ cho ${server}`}
              >
                <input
                  value={perServer[server] ?? ""}
                  onChange={(event) =>
                    setPerServer((current) => ({
                      ...current,
                      [server]: event.target.value,
                    }))
                  }
                  placeholder="http://localhost:8080"
                />
              </Field>
            ))}
          </div>
        )}

        <div className="st-run-form__row">
          <Field id="run-timeout" label="Thời gian chờ (giây)">
            <input
              type="number"
              min={1}
              value={timeoutSeconds}
              onChange={(event) => setTimeoutSeconds(event.target.value)}
            />
          </Field>
          <Button variant="secondary" loading={checking} onClick={() => void checkTargets()}>
            Kiểm tra máy chủ
          </Button>
        </div>

        <div className="st-run-form__options">
          <Checkbox checked={triage} onChange={(event) => setTriage(event.target.checked)}>
            Chẩn đoán AI khi lỗi
          </Checkbox>
          <Checkbox
            checked={onlyChanged}
            onChange={(event) => setOnlyChanged(event.target.checked)}
          >
            Chỉ chạy thay đổi
          </Checkbox>
        </div>

        <Field id="run-log-path" label="Tệp log lịch sử (tuỳ chọn)">
          <input
            value={logPath}
            onChange={(event) => setLogPath(event.target.value)}
            placeholder="tests/spectest.log.yaml"
          />
        </Field>

        {health && health.length > 0 && (
          <div className="st-run-form__health">
            {health.map((result, index) => (
              <p key={`${result.target_url}-${index}`}>
                <Badge tone={result.ready ? "pass" : "fail"}>
                  {result.ready ? "Sẵn sàng" : "Không phản hồi"}
                </Badge>
                <code>{result.target_url}</code>
                <span>{result.latency_ms} ms</span>
              </p>
            ))}
          </div>
        )}

        {error && (
          <p className="st-run-form__error" role="alert">
            {error}
          </p>
        )}

        <div className="st-run-form__actions">
          <Button type="submit" loading={starting}>
            Bắt đầu chạy
          </Button>
        </div>
      </form>
    </Card>
  );
}
