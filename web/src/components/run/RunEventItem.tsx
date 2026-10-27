import type { RunStreamEvent } from "../../api/types";
import { formatDurationMs } from "../../lib/format";
import { JsonTree } from "../data/JsonTree";
import { MethodPill } from "../data/MethodPill";
import { StatusBadge } from "../data/StatusBadge";

export function RunEventItem({ event }: { event: RunStreamEvent }) {
  switch (event.type) {
    case "connected":
    case "idle":
      return null;
    case "start":
      return (
        <>
          <p className="st-timeline__title">Bắt đầu chạy suite</p>
          <p className="st-timeline__meta">
            {event.data.total_scenarios} kịch bản ·{" "}
            {(event.data.servers ?? []).join(", ")}
          </p>
        </>
      );
    case "scenario_start":
      return <p className="st-timeline__title">{event.data.scenario_name}</p>;
    case "step_result":
      return (
        <>
          <div className="st-timeline__line">
            <MethodPill method={event.data.method} />
            <code className="st-timeline__call">{event.data.call}</code>
          </div>
          <div className="st-timeline__line">
            <StatusBadge value={event.data.passed ? "passed" : "failed"} />
            <span className="st-run-event__status">
              <code>{event.data.status_code}</code>
              <span aria-hidden="true"> / </span>
              <code>{event.data.expect}</code>
            </span>
            <span className="st-timeline__meta">
              {formatDurationMs(event.data.latency_ms)}
            </span>
          </div>
          {event.data.error && (
            <p className="st-run-event__diagnosis">
              <strong>Lỗi:</strong> {event.data.error}
            </p>
          )}
          {event.data.ai_diagnosis && (
            <p className="st-run-event__diagnosis">
              <strong>Chẩn đoán AI:</strong> {event.data.ai_diagnosis}
            </p>
          )}
          <details className="st-run-event__details">
            <summary>Chi tiết bước</summary>
            <div className="st-run-event__details-body">
              <p className="st-run-event__url">
                <span>URL</span>
                <code>{event.data.url}</code>
              </p>
              {event.data.saved_vars &&
              Object.keys(event.data.saved_vars).length > 0 ? (
                <JsonTree value={event.data.saved_vars} name="saved_vars" />
              ) : (
                <p className="st-timeline__meta">Không có biến được lưu.</p>
              )}
            </div>
          </details>
        </>
      );
    case "scenario_complete": {
      const status = event.data.skipped
        ? "skipped"
        : event.data.passed
          ? "passed"
          : "failed";
      return (
        <>
          <div className="st-timeline__line">
            <p className="st-timeline__title">Kết thúc {event.data.scenario_id}</p>
            <StatusBadge value={status} />
          </div>
          {event.data.reason && (
            <p className="st-timeline__meta">{event.data.reason}</p>
          )}
        </>
      );
    }
    case "complete":
      return (
        <>
          <p className="st-timeline__title">
            Hoàn tất · {event.data.report.passed_scenarios}/
            {event.data.report.total_scenarios} kịch bản đạt
          </p>
          <p className="st-timeline__meta">
            {event.data.report.failed_scenarios} lỗi ·{" "}
            {event.data.report.skipped_scenarios} bỏ qua
          </p>
        </>
      );
    case "cancelled":
      return (
        <>
          <p className="st-timeline__title">Đã hủy phiên chạy</p>
          {event.message && <p className="st-timeline__meta">{event.message}</p>}
        </>
      );
    case "error":
      return (
        <>
          <p className="st-timeline__title">Lỗi phiên chạy</p>
          <p className="st-timeline__meta">{event.message}</p>
        </>
      );
  }
}
