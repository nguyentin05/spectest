import { render, screen } from "@testing-library/react";
import type { RunStreamEvent } from "../../api/types";
import { RunEventItem } from "./RunEventItem";

it("renders a start event for a suite without servers", () => {
  const event = {
    type: "start",
    timestamp: "2026-10-26T10:00:00Z",
    message: "Beginning execution",
    data: { total_scenarios: 2, servers: null },
  } as unknown as RunStreamEvent;

  render(<RunEventItem event={event} />);

  expect(screen.getByText(/2 kịch bản/)).toBeInTheDocument();
});
