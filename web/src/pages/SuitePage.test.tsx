import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import type { PlanResponse } from "../api/types";
import { server } from "../mocks/server";
import { renderAt } from "../test/render";

it("renders a suite whose optional collections are omitted", async () => {
  server.use(
    http.get("/api/specs/plan", () =>
      HttpResponse.json({
        path: "spectest.yaml",
        version: 1,
        servers: null,
        apis: [],
        stats: {
          total_scenarios: 1,
          total_setups: 0,
          total_steps: 1,
          locked_scenarios: 0,
          skipped_scenarios: 0,
        },
        scenarios: [
          {
            id: "sparse",
            name: "Thiếu setup",
            steps: [{ call: "GET {api}/health", expect: 200 }],
          },
        ],
      }),
    ),
  );

  renderAt("/suite");
  expect(await screen.findByRole("heading", { name: "Thiếu setup" })).toBeInTheDocument();
  expect(screen.getByText("Không dùng setup")).toBeInTheDocument();
});

it("renders scenarios visually and exposes steps on demand", async () => {
  renderAt("/suite");
  expect(
    await screen.findByRole("heading", { name: "Đặt hàng vượt tồn kho" }),
  ).toBeInTheDocument();
  expect(screen.getByText("Đã khóa")).toBeInTheDocument();
  await userEvent.click(
    screen.getByRole("button", { name: /Đặt hàng vượt tồn kho/ }),
  );
  expect(screen.getByText("POST {order}/api/orders")).toBeInTheDocument();
  expect(screen.getByText("422")).toBeInTheDocument();
  expect(screen.getByText("Bearer {{user_token}}")).toBeInTheDocument();
});

it("renders non-string suite variables as JSON", async () => {
  const plan: PlanResponse = {
    path: "spectest.yaml",
    version: 1,
    servers: ["auth"],
    apis: [],
    analysis: "",
    seed: { admin: { vars: { retries: 3 } } },
    setup: { retry: { vars: { attempts: 3, flags: { retry: true } }, steps: [] } },
    scenarios: [],
    stats: {
      total_scenarios: 0,
      total_setups: 1,
      total_steps: 0,
      locked_scenarios: 0,
      skipped_scenarios: 0,
    },
  };
  server.use(http.get("/api/specs/plan", () => HttpResponse.json(plan)));

  renderAt("/suite");
  await userEvent.click(await screen.findByRole("tab", { name: "Setup" }));

  expect(await screen.findByText("number")).toBeInTheDocument();
});

it("filters to locked scenarios", async () => {
  renderAt("/suite");
  await userEvent.click(
    await screen.findByRole("checkbox", { name: "Chỉ kịch bản bị khóa" }),
  );
  expect(screen.queryByText("Login sai password")).not.toBeInTheDocument();
});
