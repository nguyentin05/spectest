export class ApiError extends Error {
  readonly status: number;
  readonly payload?: unknown;

  constructor(status: number, message: string, payload?: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.payload = payload;
  }
}

export function isMissingEndpoint(error: unknown): error is ApiError {
  return error instanceof ApiError && [404, 405, 501].includes(error.status);
}

// The Go backend only speaks English; translate messages a user can actually
// trigger and leave anything else untouched.
const KNOWN_ERRORS: Array<[RegExp, (...parts: string[]) => string]> = [
  [
    /^a test run is already in progress \(ID: (.+)\)$/,
    (id) => `Một phiên chạy khác đang diễn ra (ID: ${id}).`,
  ],
  [/^no active test run is in progress$/, () => "Không có phiên chạy nào đang diễn ra."],
  [/^no test runs recorded yet$/, () => "Chưa có báo cáo nào."],
  [
    /^no spectest\.yaml file found in workspace;.*$/,
    () => "Không tìm thấy spectest.yaml trong workspace.",
  ],
  [
    /^no spectest\.log\.yaml file found in workspace$/,
    () => "Không tìm thấy spectest.log.yaml trong workspace.",
  ],
  [
    /^target URL is required:.*$/,
    () => "Cần nhập địa chỉ máy chủ đích.",
  ],
  [/^target alias cannot be empty$/, () => "Alias máy chủ không được để trống."],
  [
    /^target URL for alias "(.+)" cannot be empty$/,
    (alias) => `Địa chỉ cho alias "${alias}" không được để trống.`,
  ],
  [
    /^invalid target URL "(.+)" for alias "(.+)": .+$/,
    (url, alias) => `Địa chỉ "${url}" cho alias "${alias}" không hợp lệ.`,
  ],
  [
    /^invalid default target URL "(.+)": .+$/,
    (url) => `Địa chỉ máy chủ "${url}" không hợp lệ.`,
  ],
  [
    /^cannot load suite at "(.+)": .+$/,
    (path) => `Không đọc được suite "${path}".`,
  ],
];

export function translateBackendError(message: string): string {
  for (const [pattern, format] of KNOWN_ERRORS) {
    const match = pattern.exec(message);
    if (match) return format(...match.slice(1));
  }
  return message;
}
