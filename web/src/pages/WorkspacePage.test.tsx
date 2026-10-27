import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderAt } from "../test/render";

it("lists workspace files, filters by type, and links viewers", async () => {
  renderAt("/workspace");
  expect(await screen.findByText("specs/auth-api.yaml")).toBeInTheDocument();
  expect(screen.getByText("docs/requirements.md")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Đặc tả API" }));
  expect(screen.queryByText("docs/requirements.md")).not.toBeInTheDocument();
  expect(screen.getAllByText("specs/auth-api.yaml").length).toBeGreaterThan(0);
});

it("links a markdown file to the document reader", async () => {
  renderAt("/workspace");
  const link = await screen.findByRole("link", { name: /Mở docs\/requirements.md/ });
  expect(link).toHaveAttribute("href", "/docs?path=docs%2Frequirements.md");
});

it("opens the evolution log from the workspace", async () => {
  renderAt("/workspace");
  const link = await screen.findByRole("link", { name: /Mở tests\/spectest.log.yaml/ });
  expect(link).toHaveAttribute("href", "/history?path=tests%2Fspectest.log.yaml");
});

it("uploads a file via multipart and refreshes the list", async () => {
  renderAt("/workspace");
  const input = await screen.findByLabelText("Chọn tệp để tải lên");
  await userEvent.upload(
    input,
    new File(["openapi: 3.0.0"], "petstore.yaml", { type: "application/x-yaml" }),
  );
  expect(await screen.findByText("Đã tải lên petstore.yaml")).toBeInTheDocument();
  expect(await screen.findByText("petstore.yaml")).toBeInTheDocument();
});

it("rejects an unsupported file before sending it", async () => {
  renderAt("/workspace");
  const input = await screen.findByLabelText("Chọn tệp để tải lên");
  await userEvent.upload(input, new File(["MZ"], "virus.exe"));
  expect(
    await screen.findByText("Chỉ nhận tệp .yaml, .yml, .json hoặc .md"),
  ).toBeInTheDocument();
  expect(screen.queryByText("Đã tải lên virus.exe")).not.toBeInTheDocument();
});
