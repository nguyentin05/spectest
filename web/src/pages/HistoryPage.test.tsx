import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { createHistoryFixture } from "../mocks/fixtures";
import { server } from "../mocks/server";
import { renderAt } from "../test/render";

it("renders an update with its trigger and before/after change", async () => {
  renderAt("/history");
  expect(
    await screen.findByText("API added: POST {order}/api/orders/{id}/cancel"),
  ).toBeInTheDocument();
  expect(screen.getByText("POST {auth}/api/register")).toBeInTheDocument();
  expect(screen.getByText("0912345678")).toBeInTheDocument();
  expect(screen.getByRole("heading", { name: "Lần cập nhật #1" })).toBeInTheDocument();
  expect(screen.getByText("guest_session")).toBeInTheDocument();
  expect(screen.getByText("order-cancel-pending")).toBeInTheDocument();
  expect(screen.getByText("Seed mới")).toBeInTheDocument();
  expect(screen.getByText("admin")).toBeInTheDocument();
  expect(screen.getByText("Khôi phục")).toBeInTheDocument();
  expect(screen.getByText("GET {auth}/api/v1/me")).toBeInTheDocument();
});

it("renders a log that has no updates yet", async () => {
  server.use(
    http.get("/api/specs/history", () =>
      HttpResponse.json({
        path: "tests/spectest.log.yaml",
        base: {
          at: "2026-10-26T19:31:42+07:00",
          by: "ai",
          after_hash: "7ae58395aa4d",
          spectest: {
            version: 1,
            servers: ["auth"],
            apis: [
              "GET {auth}/api/v1/me",
              "GET {auth}/health",
              "POST {auth}/api/v1/login",
              "POST {auth}/api/v1/register",
            ],
            scenarios: [{ id: "health-check" }, { id: "login-success" }],
          },
        },
        updates: null,
      }),
    ),
  );

  renderAt("/history");

  expect(await screen.findByText("Điểm xuất phát")).toBeInTheDocument();
  expect(screen.getByText("auth")).toBeInTheDocument();
  expect(screen.getByText("4 API")).toBeInTheDocument();
  expect(screen.getByText("2 kịch bản")).toBeInTheDocument();
  expect(screen.getByText("Chưa có cập nhật nào.")).toBeInTheDocument();
});

it("shows an empty state when the log file is missing", async () => {
  server.use(
    http.get("/api/specs/history", () =>
      HttpResponse.json({ success: false, error: "File not found" }, { status: 404 }),
    ),
  );
  renderAt("/history");
  expect(await screen.findByText("Chưa có lịch sử thay đổi")).toBeInTheDocument();
});

it("loads history from the path query parameter", async () => {
  let requested: URL | undefined;
  server.use(
    http.get("/api/specs/history", ({ request }) => {
      requested = new URL(request.url);
      return HttpResponse.json(createHistoryFixture());
    }),
  );
  renderAt("/history?path=tests/spectest.log.yaml");
  expect(
    await screen.findByText("API added: POST {order}/api/orders/{id}/cancel"),
  ).toBeInTheDocument();
  expect(requested?.searchParams.get("path")).toBe("tests/spectest.log.yaml");
  expect(requested?.searchParams.has("dir")).toBe(false);
});

it("lets the backend discover the log when no path is provided", async () => {
  let requested: URL | undefined;
  server.use(
    http.get("/api/specs/history", ({ request }) => {
      requested = new URL(request.url);
      return HttpResponse.json(createHistoryFixture());
    }),
  );
  renderAt("/history");
  expect(
    await screen.findByText("API added: POST {order}/api/orders/{id}/cancel"),
  ).toBeInTheDocument();
  expect(requested?.searchParams.has("path")).toBe(false);
  expect(requested?.searchParams.has("dir")).toBe(false);
});
