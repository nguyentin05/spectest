import { toHistoryResponse, toWorkspaceResponse } from "./contracts";
import type { RawHistory } from "./contracts";
import type { WorkspaceResponse } from "./types";

it("marks the evolution log as history when the backend calls it other", () => {
  const raw: WorkspaceResponse = {
    current_dir: "/repo",
    items: [
      {
        name: "tests/spectest.log.yaml",
        path: "/repo/tests/spectest.log.yaml",
        type: "other",
        size: 10,
      },
      {
        name: "tests\\spectest.log.yml",
        path: "C:\\repo\\tests\\spectest.log.yml",
        type: "other",
        size: 20,
      },
      {
        name: "specs/auth-api.yaml",
        path: "/repo/specs/auth-api.yaml",
        type: "other",
        size: 30,
      },
    ],
  };

  expect(toWorkspaceResponse(raw).items.map((item) => item.type)).toEqual([
    "history",
    "history",
    "other",
  ]);
});

it("keeps restored operations and seeded variables from the log", () => {
  const raw: RawHistory = {
    updates: [
      {
        n: 2,
        trigger: { openapi: { restored: ["GET {auth}/api/v1/me"] } },
        create: { seed: { admin: { vars: { admin_email: "admin@shop.com" } } } },
      },
    ],
  };

  const update = toHistoryResponse(raw).updates[0]!;

  expect(update.trigger.openapi?.restored).toEqual(["GET {auth}/api/v1/me"]);
  expect(update.create.seed_added).toEqual(["admin"]);
});
