import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { workspaceDir, createWorkspaceFixture } from "../mocks/fixtures";
import { server } from "../mocks/server";
import { mockState } from "../mocks/store";
import { renderAt } from "../test/render";

it("generates from two APIs with server aliases", async () => {
  renderAt("/generate");
  await userEvent.click(
    await screen.findByRole("checkbox", { name: "specs/auth-api.yaml" }),
  );
  await userEvent.click(screen.getByRole("checkbox", { name: "specs/order-api.json" }));
  await userEvent.type(
    screen.getByLabelText("Alias cho specs/auth-api.yaml"),
    "auth",
  );
  await userEvent.type(
    screen.getByLabelText("Alias cho specs/order-api.json"),
    "order",
  );
  await userEvent.click(screen.getByRole("button", { name: "Sinh kịch bản" }));
  expect(await screen.findByText("Đã sinh 6 kịch bản")).toBeInTheDocument();
  expect(screen.getByText("./tests/spectest.yaml")).toBeInTheDocument();
});

it("detects an API spec by content when the backend labels it other", async () => {
  const items = [
    ...createWorkspaceFixture(),
    {
      name: "specs/petstore.yaml",
      path: `${workspaceDir}/specs/petstore.yaml`,
      type: "other" as const,
      size: 120,
    },
    {
      name: "notes/random.yaml",
      path: `${workspaceDir}/notes/random.yaml`,
      type: "other" as const,
      size: 40,
    },
  ];
  server.use(
    http.get("/api/specs/workspace", () =>
      HttpResponse.json({ current_dir: workspaceDir, items }),
    ),
  );
  mockState.markdown[`${workspaceDir}/specs/petstore.yaml`] =
    "openapi: 3.0.0\ninfo:\n  title: Petstore";
  mockState.markdown[`${workspaceDir}/notes/random.yaml`] = "version: 1\nnotes: []";

  renderAt("/generate");

  expect(
    await screen.findByRole("checkbox", { name: "specs/petstore.yaml" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("checkbox", { name: "notes/random.yaml" }),
  ).not.toBeInTheDocument();
});

it("rejects duplicate server aliases before sending", async () => {
  renderAt("/generate");
  await userEvent.click(
    await screen.findByRole("checkbox", { name: "specs/auth-api.yaml" }),
  );
  await userEvent.click(screen.getByRole("checkbox", { name: "specs/order-api.json" }));
  await userEvent.type(
    screen.getByLabelText("Alias cho specs/auth-api.yaml"),
    "svc",
  );
  await userEvent.type(
    screen.getByLabelText("Alias cho specs/order-api.json"),
    "svc",
  );

  expect(await screen.findAllByText("Alias đã được dùng cho server khác.")).toHaveLength(2);
  expect(screen.getByRole("button", { name: "Sinh kịch bản" })).toBeDisabled();
});

it("keeps the generation result visible while the workspace reloads", async () => {
  let workspaceCalls = 0;
  server.use(
    http.get("/api/specs/workspace", async () => {
      workspaceCalls += 1;
      if (workspaceCalls > 1) {
        await new Promise((resolve) => setTimeout(resolve, 300));
      }
      return HttpResponse.json({
        current_dir: workspaceDir,
        items: createWorkspaceFixture(),
      });
    }),
  );

  renderAt("/generate");
  await userEvent.click(
    await screen.findByRole("checkbox", { name: "specs/auth-api.yaml" }),
  );
  await userEvent.type(
    screen.getByLabelText("Alias cho specs/auth-api.yaml"),
    "auth",
  );
  await userEvent.click(screen.getByRole("button", { name: "Sinh kịch bản" }));

  expect(await screen.findByText("Đã sinh 6 kịch bản")).toBeInTheDocument();
  await waitFor(() => expect(workspaceCalls).toBeGreaterThan(1));
  expect(screen.getByText("Đã sinh 6 kịch bản")).toBeInTheDocument();
});

it("shows the pending synthesis state on 202", async () => {  server.use(
    http.post("/api/generate", () =>
      HttpResponse.json(
        {
          success: true,
          status: "pending_synthesis",
          message: "Inputs verified and normalized. Core synthesis engine is processing.",
        },
        { status: 202 },
      ),
    ),
  );
  renderAt("/generate");
  await userEvent.click(
    await screen.findByRole("checkbox", { name: "specs/auth-api.yaml" }),
  );
  await userEvent.type(
    screen.getByLabelText("Alias cho specs/auth-api.yaml"),
    "auth",
  );
  await userEvent.click(screen.getByRole("button", { name: "Sinh kịch bản" }));
  expect(await screen.findByText("Đang tổng hợp kịch bản")).toBeInTheDocument();
  expect(
    screen.getByText("Inputs verified and normalized. Core synthesis engine is processing."),
  ).toBeInTheDocument();
});
