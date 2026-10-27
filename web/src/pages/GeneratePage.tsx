import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import type {
  GeneratePendingResponse,
  GenerateRequest,
  GenerateResponse,
  WorkspaceItem,
} from "../api/types";
import { api } from "../api/endpoints";
import { isMissingEndpoint } from "../api/errors";
import { useApi } from "../hooks/useApi";
import { isOpenApiDocument } from "../lib/openapi";
import { withQuery } from "../lib/url";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { CapabilityNotice } from "../components/ui/CapabilityNotice";
import { Card } from "../components/ui/Card";
import { Checkbox } from "../components/ui/Checkbox";
import { ErrorState } from "../components/ui/ErrorState";
import { Field } from "../components/ui/Field";
import { LoadingBlock } from "../components/ui/LoadingBlock";
import { useToast } from "../components/ui/Toast";

function findAliasErrors(
  items: WorkspaceItem[],
  aliases: Record<string, string>,
): Record<string, string> {
  const errors: Record<string, string> = {};
  const seen = new Map<string, string>();

  for (const item of items) {
    const alias = (aliases[item.name] ?? "").trim();
    if (!alias) {
      errors[item.name] = "Nhập alias cho server.";
      continue;
    }

    const key = alias.toLowerCase();
    const owner = seen.get(key);
    if (owner) {
      errors[item.name] = "Alias đã được dùng cho server khác.";
      errors[owner] ??= "Alias đã được dùng cho server khác.";
    } else {
      seen.set(key, item.name);
    }
  }

  return errors;
}

function GenerateWizard({
  items,
  onGenerated,
}: {
  items: WorkspaceItem[];
  onGenerated: () => void;
}) {
  const navigate = useNavigate();
  const { push } = useToast();
  const [selected, setSelected] = useState<string[]>([]);
  const [aliases, setAliases] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [reqPath, setReqPath] = useState("");
  const [outputDir, setOutputDir] = useState("./tests");
  const [dryRun, setDryRun] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [result, setResult] = useState<GenerateResponse | null>(null);
  const [pending, setPending] = useState<GeneratePendingResponse | null>(null);
  const [sniffedSpecs, setSniffedSpecs] = useState<string[]>([]);

  useEffect(() => {
    const candidates = items.filter(
      (item) => item.type === "other" && /\.(ya?ml|json)$/i.test(item.name),
    );
    if (candidates.length === 0) return;

    let active = true;
    // shortcut: the workspace walk mislabels specs by name, so reuse
    // /markdown/read to sniff the first bytes; drop this when Go classifies by content.
    void Promise.all(
      candidates.map(async (item) => {
        try {
          const document = await api.markdown(item.path);
          return isOpenApiDocument(document.content) ? item.name : null;
        } catch {
          return null;
        }
      }),
    ).then((names) => {
      if (active) {
        setSniffedSpecs(names.filter((name): name is string => name !== null));
      }
    });

    return () => {
      active = false;
    };
  }, [items]);

  const openapiItems = items.filter(
    (item) => item.type === "openapi" || sniffedSpecs.includes(item.name),
  );
  const prdItems = items.filter((item) => item.type === "prd");
  const selectedItems = openapiItems.filter((item) => selected.includes(item.name));
  const aliasErrors = findAliasErrors(selectedItems, aliases);
  const hasAliasErrors = Object.keys(aliasErrors).length > 0;

  const reason =
    selected.length === 0
      ? "Chọn ít nhất một đặc tả API."
      : hasAliasErrors
        ? "Hoàn tất alias cho các server đã chọn."
        : !outputDir.trim()
          ? "Nhập thư mục đầu ra."
          : null;

  function toggle(name: string) {
    setSelected((current) =>
      current.includes(name)
        ? current.filter((item) => item !== name)
        : [...current, name],
    );
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (reason) {
      setError(new Error(reason));
      return;
    }

    const body: GenerateRequest = {
      apis: selectedItems.map((item) => ({
        server: (aliases[item.name] ?? "").trim(),
        path: item.name,
      })),
      req_path: reqPath,
      output_dir: outputDir.trim(),
      dry_run: dryRun,
    };

    setError(null);
    setResult(null);
    setPending(null);
    setSubmitting(true);
    try {
      const response = await api.generate(body);
      if ("status" in response) {
        setPending(response);
      } else {
        setResult(response);
        push("Sinh kịch bản thành công", "success");
        onGenerated();
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause : new Error(String(cause)));
    } finally {
      setSubmitting(false);
    }
  }

  async function recheck() {
    setChecking(true);
    setError(null);
    try {
      const plan = await api.plan();
      navigate(withQuery("/suite", { path: plan.path }));
    } catch (cause) {
      setError(cause instanceof Error ? cause : new Error(String(cause)));
    } finally {
      setChecking(false);
    }
  }

  return (
    <form className="st-generate" onSubmit={(event) => void submit(event)}>
      <Card
        title="Chọn đặc tả API"
        subtitle="Mỗi server cần một alias dùng trong lời gọi của kịch bản."
      >
        {openapiItems.length === 0 ? (
          <p className="st-generate__empty">
            Chưa có đặc tả API. Tải tệp YAML hoặc JSON trong workspace.
          </p>
        ) : (
          <ul className="st-generate__apis">
            {openapiItems.map((item, index) => {
              const checked = selected.includes(item.name);
              const alias = aliases[item.name] ?? "";
              const showError =
                touched[item.name] || alias.trim().length > 0;

              return (
                <li key={item.name} className="st-generate__api">
                  <Checkbox checked={checked} onChange={() => toggle(item.name)}>
                    {item.name}
                  </Checkbox>

                  {checked && (
                    <Field
                      id={`generate-alias-${index}`}
                      label={`Alias cho ${item.name}`}
                      className="st-generate__alias"
                      error={showError ? aliasErrors[item.name] : undefined}
                    >
                      <input
                        value={alias}
                        placeholder="server"
                        onChange={(event) =>
                          setAliases((current) => ({
                            ...current,
                            [item.name]: event.target.value,
                          }))
                        }
                        onBlur={() =>
                          setTouched((current) => ({ ...current, [item.name]: true }))
                        }
                      />
                    </Field>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </Card>

      <Card
        title="Tài liệu yêu cầu"
        subtitle="Thông tin nghiệp vụ giúp LLM hiểu ngữ cảnh khi sinh kịch bản."
      >
        <div className="st-generate__options">
          <Field id="generate-req" label="Tài liệu yêu cầu">
            <select value={reqPath} onChange={(event) => setReqPath(event.target.value)}>
              <option value="">Không dùng tài liệu yêu cầu</option>
              {prdItems.map((item) => (
                <option key={item.name} value={item.name}>
                  {item.name}
                </option>
              ))}
            </select>
          </Field>

          <Field id="generate-output" label="Thư mục đầu ra">
            <input
              value={outputDir}
              onChange={(event) => setOutputDir(event.target.value)}
              placeholder="./tests"
            />
          </Field>

          <Checkbox checked={dryRun} onChange={(event) => setDryRun(event.target.checked)}>
            Chạy thử, không ghi tệp
          </Checkbox>
        </div>
      </Card>

      {error &&
        (isMissingEndpoint(error) ? (
          <CapabilityNotice feature="sinh kịch bản" error={error} />
        ) : (
          <p className="st-generate__alert" role="alert">
            {error.message}
          </p>
        ))}

      <div className="st-generate__summary">
        <div>
          <p className="st-generate__count">{selected.length} đặc tả API đã chọn</p>
          {reason && <p className="st-generate__reason">{reason}</p>}
        </div>
        <Button type="submit" loading={submitting} disabled={reason !== null}>
          Sinh kịch bản
        </Button>
      </div>

      {result && (
        <Card
          className="st-generate__result"
          title={`Đã sinh ${result.scenarios_generated} kịch bản`}
          actions={<Badge tone="pass">Hoàn tất</Badge>}
        >
          <p className="st-generate__message">{result.message}</p>

          <dl className="st-generate__paths">
            <div>
              <dt>Suite</dt>
              <dd>
                <code>{result.suite_path}</code>
              </dd>
            </div>
            <div>
              <dt>Log</dt>
              <dd>
                <code>{result.log_path}</code>
              </dd>
            </div>
          </dl>

          <div className="st-generate__servers">
            <span>Server:</span>
            {result.servers.map((server) => (
              <Badge key={server} mono>
                {server}
              </Badge>
            ))}
          </div>

          <div className="st-generate__result-action">
            <Link
              className="st-btn st-btn--secondary st-btn--sm"
              to={withQuery("/suite", { path: result.suite_path })}
            >
              Xem kịch bản
            </Link>
          </div>
        </Card>
      )}

      {pending && (
        <Card
          className="st-generate__pending"
          title="Đang tổng hợp kịch bản"
          actions={<Badge tone="live">Chờ xử lý</Badge>}
        >
          <p className="st-generate__message">{pending.message}</p>
          <Button
            variant="secondary"
            loading={checking}
            onClick={() => void recheck()}
          >
            Kiểm tra lại
          </Button>
        </Card>
      )}
    </form>
  );
}

export function GeneratePage() {
  const { data, error, loading, reload } = useApi(api.workspace);

  let content: ReactNode = null;

  if (data) {
    content = <GenerateWizard items={data.items} onGenerated={reload} />;
  } else if (loading) {
    content = <LoadingBlock label="Đang quét workspace…" rows={3} />;
  } else if (error) {
    content = isMissingEndpoint(error) ? (
      <CapabilityNotice feature="sinh kịch bản" error={error} />
    ) : (
      <ErrorState error={error} onRetry={reload} />
    );
  }

  return (
    <section className="st-page">
      <header className="st-page__head">
        <h1>Sinh kịch bản</h1>
        <p className="st-page__subtitle">
          Sinh suite kiểm thử từ đặc tả API và tài liệu yêu cầu.
        </p>
      </header>
      {content}
    </section>
  );
}
