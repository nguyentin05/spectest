import { useEffect, useRef, useState, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import type { RunReport, RunStreamEvent } from "../api/types";
import { api } from "../api/endpoints";
import { isMissingEndpoint, translateBackendError } from "../api/errors";
import { useRunStream } from "../hooks/useRunStream";
import { formatDurationMs } from "../lib/format";
import { Badge } from "../components/ui/Badge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { ErrorState } from "../components/ui/ErrorState";
import { LoadingBlock } from "../components/ui/LoadingBlock";
import { Modal } from "../components/ui/Modal";
import { Tabs } from "../components/ui/Tabs";
import { ReportTable } from "../components/run/ReportTable";
import { RunForm } from "../components/run/RunForm";
import { RunSummary } from "../components/run/RunSummary";
import { RunTimeline } from "../components/run/RunTimeline";

type StartEvent = Extract<RunStreamEvent, { type: "start" }>;
type EndEvent = Extract<RunStreamEvent, { type: "complete" | "cancelled" | "error" }>;

type ReportState = {
  report: RunReport | null;
  error: Error | null;
  loading: boolean;
  settled: boolean;
  reload: () => void;
};

function findEndEvent(events: RunStreamEvent[]): EndEvent | undefined {
  return [...events]
    .reverse()
    .find(
      (event): event is EndEvent =>
        event.type === "complete" ||
        event.type === "cancelled" ||
        event.type === "error",
    );
}

function useElapsed(events: RunStreamEvent[]): string {
  const startEvent = events.find(
    (event): event is StartEvent => event.type === "start",
  );
  const endEvent = findEndEvent(events);
  const [now, setNow] = useState(() => Date.now());
  const running = Boolean(startEvent && !endEvent);

  useEffect(() => {
    if (!running) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [running]);

  if (!startEvent) return "0 ms";
  const start = Date.parse(startEvent.timestamp);
  const end = endEvent ? Date.parse(endEvent.timestamp) : now;
  if (Number.isNaN(start) || Number.isNaN(end)) return "0 ms";
  return formatDurationMs(Math.max(0, end - start));
}

function scenarioCounts(events: RunStreamEvent[]): {
  done: number;
  passed: number;
  failed: number;
} {
  let done = 0;
  let passed = 0;

  for (const event of events) {
    if (event.type !== "scenario_complete") continue;
    done += 1;
    if (event.data.passed) passed += 1;
  }

  return { done, passed, failed: done - passed };
}

function useRunReport(runId: string, endEvent: EndEvent | undefined): ReportState {
  const [report, setReport] = useState<RunReport | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [loading, setLoading] = useState(false);
  const [settled, setSettled] = useState(false);
  const [version, setVersion] = useState(0);
  const key = runId ? `${runId}:${endEvent?.timestamp ?? "start"}` : "";

  useEffect(() => {
    if (!key) {
      setReport(null);
      setError(null);
      setLoading(false);
      setSettled(false);
      return;
    }

    let active = true;
    setLoading(true);
    setError(null);
    api.report().then(
      (result) => {
        if (!active) return;
        setReport(result);
        setLoading(false);
        setSettled(true);
      },
      (cause: unknown) => {
        if (!active) return;
        setError(cause instanceof Error ? cause : new Error(String(cause)));
        setLoading(false);
        setSettled(true);
      },
    );

    return () => {
      active = false;
    };
  }, [key, version]);

  return {
    report,
    error,
    loading,
    settled,
    reload: () => setVersion((current) => current + 1),
  };
}

export function RunPage() {
  const [params, setParams] = useSearchParams();
  const runId = params.get("run") ?? "";
  const { events, connected, error, start, stop, clear } = useRunStream();
  const streamRef = useRef<HTMLDivElement>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [cancelRequested, setCancelRequested] = useState(false);

  const startEvent = events.find(
    (event): event is StartEvent => event.type === "start",
  );
  const endEvent = findEndEvent(events);
  const reportState = useRunReport(runId, endEvent);
  const currentReport = reportState.report?.id === runId ? reportState.report : null;
  const restored = Boolean(
    runId && currentReport && currentReport.status !== "running",
  );

  useEffect(() => {
    setConfirmOpen(false);
    setCancelError(null);
    setCancelRequested(false);
    if (!runId || !reportState.settled) return;
    clear();
    if (restored) return;
    start();
    return () => stop();
  }, [runId, reportState.settled, restored, clear, start, stop]);

  useEffect(() => {
    const element = streamRef.current;
    if (!element) return;
    const distance = element.scrollHeight - element.scrollTop - element.clientHeight;
    if (distance < 120) {
      element.scrollTop = element.scrollHeight;
    }
  }, [events]);

  const total = startEvent?.data.total_scenarios ?? 0;
  const { done, passed, failed } = scenarioCounts(events);
  const elapsed = useElapsed(events);
  async function confirmCancel() {
    setCancelling(true);
    setCancelError(null);
    try {
      const result = await api.cancelRun();
      setConfirmOpen(false);
      if (!result.success) {
        setCancelError(
          translateBackendError(result.message || "Không hủy được phiên chạy"),
        );
        return;
      }
      setCancelRequested(true);
    } catch (cause) {
      setConfirmOpen(false);
      setCancelError(
        cause instanceof Error ? cause.message : "Không hủy được phiên chạy",
      );
    } finally {
      setCancelling(false);
    }
  }

  const finishedStatus =
    endEvent?.type === "cancelled"
      ? "cancelled"
      : endEvent?.type === "error"
        ? "failed"
        : endEvent
          ? "passed"
          : currentReport && currentReport.status !== "running"
            ? currentReport.status
            : null;

  const statusLabel = error
    ? "Mất kết nối"
    : finishedStatus === "cancelled"
      ? "Đã hủy"
      : finishedStatus === "failed"
        ? "Lỗi"
        : finishedStatus
          ? "Hoàn tất"
          : connected
            ? "Đang chạy"
            : "Đã dừng";
  const statusTone = error
    ? "fail"
    : finishedStatus === "cancelled"
      ? "skip"
      : finishedStatus === "failed"
        ? "fail"
        : finishedStatus
          ? "pass"
          : connected
            ? "live"
            : "skip";

  let reportContent: ReactNode;
  if (reportState.loading && !currentReport) {
    reportContent = <LoadingBlock label="Đang tải báo cáo…" rows={4} />;
  } else if (reportState.error) {
    reportContent = isMissingEndpoint(reportState.error) ? (
      <EmptyState
        title="Chưa có báo cáo chi tiết"
        description={
          endEvent || restored
            ? "Máy chủ chưa trả về báo cáo cho lần chạy này."
            : "Báo cáo sẽ có sau khi phiên chạy kết thúc."
        }
      />
    ) : (
      <ErrorState
        title="Không tải được báo cáo"
        error={reportState.error}
        onRetry={reportState.reload}
      />
    );
  } else if (currentReport) {
    reportContent = <ReportTable results={currentReport.results} />;
  } else {
    reportContent = (
      <EmptyState
        title="Chưa có báo cáo chi tiết"
        description={
          endEvent
            ? "Máy chủ chưa trả về báo cáo cho lần chạy này."
            : "Báo cáo sẽ có sau khi phiên chạy kết thúc."
        }
      />
    );
  }

  let live: ReactNode = null;
  if (runId) {
    live = (
      <section className="st-run-live" aria-label="Phiên chạy trực tiếp">
        <header className="st-run-live__head">
          <div>
            <h2 className="st-run-live__title">Phiên chạy trực tiếp</h2>
            <p className="st-run-live__id">
              <code>{runId}</code>
            </p>
          </div>
          <div className="st-run-live__actions">
            <Badge tone={statusTone}>{statusLabel}</Badge>
            {!endEvent && !restored && !cancelRequested && (
              <Button
                variant="danger"
                size="sm"
                onClick={() => {
                  setCancelError(null);
                  setConfirmOpen(true);
                }}
              >
                Hủy phiên chạy
              </Button>
            )}
          </div>
        </header>

        <div className="st-run-live__stats" aria-label="Tiến độ phiên chạy">
          <span>
            <strong>
              {done}/{total}
            </strong>{" "}
            kịch bản
          </span>
          <span>
            Đạt <strong>{passed}</strong>
          </span>
          <span>
            Lỗi <strong>{failed}</strong>
          </span>
          <span>
            Thời gian <strong>{elapsed}</strong>
          </span>
        </div>

        {cancelError && (
          <p className="st-run-live__cancel-error" role="alert">
            {cancelError}
          </p>
        )}

        {cancelRequested && (
          <div className="st-run-live__cancelled" role="status">
            <strong>Phiên chạy đã dừng theo yêu cầu.</strong>
            <span className="st-run-live__hint">
              Báo cáo một phần đã sẵn sàng bên dưới.
            </span>
          </div>
        )}

        {error && (
          <div className="st-run-live__reconnect" role="alert">
            <div>
              <p>Mất kết nối với phiên chạy</p>
              <p className="st-run-live__hint">
                Các sự kiện đã nhận vẫn được giữ lại.
              </p>
            </div>
            <Button variant="secondary" size="sm" onClick={start}>
              Kết nối lại
            </Button>
          </div>
        )}

        {currentReport && <RunSummary report={currentReport} />}

        <Tabs
          label="Nội dung phiên chạy"
          defaultValue="stream"
          items={[
            {
              id: "stream",
              label: "Diễn tiến",
              content: (
                <div className="st-run-live__stream" ref={streamRef}>
                  <RunTimeline events={events} live={connected && !error} />
                </div>
              ),
            },
            {
              id: "report",
              label: "Báo cáo chi tiết",
              content: reportContent,
            },
          ]}
        />
      </section>
    );
  }

  return (
    <section className="st-page">
      <header className="st-page__head">
        <h1>Chạy thử</h1>
        <p className="st-page__subtitle">
          Chạy suite kiểm thử và theo dõi từng bước theo thời gian thực.
        </p>
      </header>

      <RunForm
        onStarted={(id) => setParams({ run: id }, { replace: true })}
      />

      {live}

      <Modal
        open={confirmOpen}
        title="Hủy phiên chạy?"
        onClose={() => setConfirmOpen(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirmOpen(false)}>
              Giữ lại
            </Button>
            <Button
              variant="danger"
              loading={cancelling}
              onClick={() => void confirmCancel()}
            >
              Xác nhận hủy
            </Button>
          </>
        }
      >
        <p>
          Phiên chạy sẽ dừng ngay. Các bước đã chạy vẫn được lưu trong báo cáo.
        </p>
      </Modal>
    </section>
  );
}
