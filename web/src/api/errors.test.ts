import { translateBackendError } from "./errors";

it.each([
  [
    "a test run is already in progress (ID: run-1)",
    "Một phiên chạy khác đang diễn ra (ID: run-1).",
  ],
  ["no active test run is in progress", "Không có phiên chạy nào đang diễn ra."],
  ["no test runs recorded yet", "Chưa có báo cáo nào."],
  [
    "no spectest.yaml file found in workspace; please specify ?path= or upload one",
    "Không tìm thấy spectest.yaml trong workspace.",
  ],
  ["no spectest.log.yaml file found in workspace", "Không tìm thấy spectest.log.yaml trong workspace."],
  [
    "target URL is required: specify 'target_url' or 'targets' map",
    "Cần nhập địa chỉ máy chủ đích.",
  ],
  ["target alias cannot be empty", "Alias máy chủ không được để trống."],
  [
    'target URL for alias "auth" cannot be empty',
    'Địa chỉ cho alias "auth" không được để trống.',
  ],
  [
    'invalid target URL "auth:8080" for alias "auth": missing scheme',
    'Địa chỉ "auth:8080" cho alias "auth" không hợp lệ.',
  ],
  [
    'cannot load suite at "tests/spectest.yaml": no such file',
    'Không đọc được suite "tests/spectest.yaml".',
  ],
])("translates a known backend message: %s", (source, expected) => {
  expect(translateBackendError(source)).toBe(expected);
});

it("keeps an unknown backend message unchanged", () => {
  expect(translateBackendError("failed to save endpoint: disk full")).toBe(
    "failed to save endpoint: disk full",
  );
});
