import { isOpenApiDocument } from "./openapi";

it("accepts an OpenAPI YAML document", () => {
  expect(isOpenApiDocument("openapi: 3.0.0\ninfo:\n  title: Petstore")).toBe(true);
});

it("accepts a Swagger YAML document with a leading document marker", () => {
  expect(isOpenApiDocument("---\nswagger: \"2.0\"\ninfo:\n  title: Legacy")).toBe(true);
});

it("accepts an OpenAPI JSON document", () => {
  expect(isOpenApiDocument('{"openapi": "3.1.0", "info": {"title": "Petstore"}}')).toBe(true);
});

it("rejects a YAML file that is not an API spec", () => {
  expect(isOpenApiDocument("version: 1\nservers:\n  - auth\nscenarios: []")).toBe(false);
});
