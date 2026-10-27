const HEAD = 2048;
const YAML_HEADER = /^\uFEFF?\s*(openapi|swagger)\s*:/m;
const JSON_KEY = /"(openapi|swagger)"\s*:/;

// OpenAPI specs can be named anything, so the first bytes are the only
// reliable signal the web app has without a backend change.
export function isOpenApiDocument(raw: string): boolean {
  const head = raw.slice(0, HEAD);
  return YAML_HEADER.test(head) || JSON_KEY.test(head);
}
