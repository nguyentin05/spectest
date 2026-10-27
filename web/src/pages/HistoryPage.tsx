import { useState, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import type { HistoryChange, HistoryUpdate } from "../api/types";
import { api } from "../api/endpoints";
import { ApiError, isMissingEndpoint } from "../api/errors";
import { useApi } from "../hooks/useApi";
import { useDebouncedValue } from "../hooks/useDebouncedValue";
import { formatDateTime } from "../lib/format";
import { DiffView } from "../components/data/DiffView";
import { Timeline } from "../components/data/Timeline";
import { Badge, type BadgeTone } from "../components/ui/Badge";
import { CapabilityNotice } from "../components/ui/CapabilityNotice";
import { Card } from "../components/ui/Card";
import { CopyButton } from "../components/ui/CopyButton";
import { EmptyState } from "../components/ui/EmptyState";
import { ErrorState } from "../components/ui/ErrorState";
import { LoadingBlock } from "../components/ui/LoadingBlock";
import { SearchInput } from "../components/ui/SearchInput";

function authorLabel(by: string): string {
  return by === "ai" ? "AI" : "Người";
}

function TriggerGroup({
  label,
  values,
  tone = "neutral",
}: {
  label: string;
  values: string[];
  tone?: BadgeTone;
}) {
  if (values.length === 0) return null;

  return (
    <div className="st-history__trigger-group">
      <span className="st-history__label">{label}</span>
      {values.map((value) => (
        <Badge key={value} mono tone={tone}>
          {value}
        </Badge>
      ))}
    </div>
  );
}

function ChangeCard({ change }: { change: HistoryChange }) {
  return (
    <section className="st-history__change">
      <div className="st-history__change-head">
        <Badge mono>{change.target}</Badge>
        <code>{change.id}</code>
      </div>
      <p className="st-history__reason">{change.reason}</p>
      <DiffView before={change.before} after={change.after} />
    </section>
  );
}

function UpdateItem({ update }: { update: HistoryUpdate }) {
  const { openapi, requirements } = update.trigger ?? {};

  return (
    <article className="st-history__update">
      <div className="st-history__head">
        <h2 className="st-history__title">Lần cập nhật #{update.n}</h2>
        <Badge tone={update.by === "ai" ? "brand" : "neutral"} title={update.by}>
          {authorLabel(update.by)}
        </Badge>
        <span className="st-history__muted">{formatDateTime(update.at)}</span>
      </div>

      <p className="st-history__note">{update.note}</p>

      <p className="st-history__meta">
        Mã băm <code>{update.after_hash}</code>
        <CopyButton value={update.after_hash} label={`mã băm cập nhật ${update.n}`} />
      </p>

      {(openapi || requirements) && (
        <div className="st-history__triggers">
          {openapi && (
            <>
              <TriggerGroup label="Thêm" values={openapi.added} tone="brand" />
              <TriggerGroup label="Thay đổi" values={openapi.changed} />
              <TriggerGroup label="Xóa" values={openapi.removed} tone="fail" />
              <TriggerGroup label="Khôi phục" values={openapi.restored} tone="brand" />
            </>
          )}
          {requirements && (
            <div className="st-history__trigger-group">
              <span className="st-history__label">Yêu cầu</span>
              <code className="st-history__hash">{requirements.before_hash}</code>
              <span className="st-history__muted" aria-hidden="true">
                →
              </span>
              <code className="st-history__hash">{requirements.after_hash}</code>
            </div>
          )}
        </div>
      )}

      {((update.create?.seed_added?.length ?? 0) > 0 ||
        (update.create?.setup_added?.length ?? 0) > 0 ||
        (update.create?.scenarios_added?.length ?? 0) > 0) && (
        <div className="st-history__created">
          <TriggerGroup
            label="Seed mới"
            values={update.create.seed_added}
            tone="brand"
          />
          <TriggerGroup
            label="Setup mới"
            values={update.create?.setup_added ?? []}
            tone="brand"
          />
          <TriggerGroup
            label="Kịch bản mới"
            values={update.create?.scenarios_added ?? []}
            tone="brand"
          />
        </div>
      )}

      {(update.change ?? []).map((change, index) => (
        <ChangeCard key={`${change.target}-${change.id}-${index}`} change={change} />
      ))}
    </article>
  );
}

function matches(update: HistoryUpdate, query: string): boolean {
  if (!query) return true;

  const text = [
    update.note,
    ...(update.create?.seed_added ?? []),
    ...(update.create?.setup_added ?? []),
    ...(update.create?.scenarios_added ?? []),
    ...(update.change ?? []).flatMap((change) => [change.target, change.id, change.reason]),
    ...(update.trigger?.openapi
      ? [
          ...update.trigger.openapi.added,
          ...update.trigger.openapi.changed,
          ...update.trigger.openapi.removed,
          ...update.trigger.openapi.restored,
        ]
      : []),
  ]
    .join(" ")
    .toLowerCase();

  return text.includes(query);
}

export function HistoryPage() {
  const [params] = useSearchParams();
  const path = params.get("path")?.trim();
  const { data, error, loading, reload } = useApi(() => api.history(path), [path]);
  const [search, setSearch] = useState("");
  const query = useDebouncedValue(search.trim().toLowerCase());

  let content: ReactNode = null;

  if (loading) {
    content = <LoadingBlock label="Đang đọc lịch sử thay đổi…" rows={3} />;
  } else if (error instanceof ApiError && error.status === 404) {
    content = (
      <EmptyState
        title="Chưa có lịch sử thay đổi"
        description="Lịch sử sẽ xuất hiện sau lần cập nhật đặc tả đầu tiên."
      />
    );
  } else if (error) {
    content = isMissingEndpoint(error) ? (
      <CapabilityNotice feature="lịch sử thay đổi" error={error} />
    ) : (
      <ErrorState error={error} onRetry={reload} />
    );
  } else if (data) {
    const updates = [...data.updates]
      .sort((left, right) => right.n - left.n)
      .filter((update) => matches(update, query));

    content = (
      <>
        <Card title="Điểm xuất phát" subtitle={formatDateTime(data.base.at)}>
          <div className="st-history__base">
            <Badge tone={data.base.by === "ai" ? "brand" : "neutral"} title={data.base.by}>
              {authorLabel(data.base.by)}
            </Badge>
            <code className="st-history__hash">{data.base.after_hash}</code>
            <CopyButton value={data.base.after_hash} label="mã băm gốc" />
            {data.base.spectest_summary?.servers?.map((server) => (
              <Badge key={server} mono>
                {server}
              </Badge>
            ))}
            <Badge>{data.base.spectest_summary?.apis_count ?? 0} API</Badge>
            <Badge>{data.base.spectest_summary?.scenarios_count ?? 0} kịch bản</Badge>
          </div>
        </Card>

        <div className="st-history__toolbar">
          <SearchInput
            aria-label="Tìm trong lịch sử"
            placeholder="Tìm theo ghi chú, API, setup, kịch bản…"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <span className="st-history__muted">{updates.length} cập nhật</span>
        </div>

        {data.updates.length === 0 ? (
          <p className="st-history__empty">Chưa có cập nhật nào.</p>
        ) : updates.length === 0 ? (
          <p className="st-history__empty">Không có cập nhật phù hợp.</p>
        ) : (          <Timeline
            label="Lịch sử cập nhật"
            items={updates.map((update) => ({
              id: String(update.n),
              children: <UpdateItem update={update} />,
            }))}
          />
        )}
      </>
    );
  }

  return (
    <section className="st-page">
      <h1>Lịch sử</h1>
      {content}
    </section>
  );
}
