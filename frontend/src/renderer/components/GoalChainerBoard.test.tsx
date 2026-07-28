import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import type { GoalChainerProjection } from "../hooks/useGoalChainerQuery";

const { navigateMock } = vi.hoisted(() => ({ navigateMock: vi.fn() }));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigateMock,
}));

import { GoalChainerBoard } from "./GoalChainerBoard";

const digest =
  "a3c0c8a6e6a6c9308f32af7fd614159fd64a35f9ce90f9b443f350b3887275fa";

function projection(): GoalChainerProjection {
  return {
    protocol: "goalchainer-ui-v1",
    sourceRevision: digest,
    threadId: "incident-42",
    title: "Incident response",
    owner: "responder",
    semanticDigest: digest,
    planDigest: digest,
    usageTokens: 42,
    summary: {
      total: 4,
      scheduled: 2,
      backlog: 0,
      ready: 1,
      inProgress: 0,
      blocked: 1,
      done: 0,
      progressPct: 0,
    },
    tasks: [
      {
        id: "later-normal",
        capability: "local",
        acceptance: "later-accepted",
        stakes: "normal",
        dependencies: [],
        state: "scheduled",
        upstreamBlocked: false,
        attempts: 0,
        scheduledAt: "2026-07-30T04:00:00Z",
        releaseReason: "not-before constraint has not elapsed",
      },
      {
        id: "earlier-high",
        capability: "specialist",
        acceptance: "earlier-accepted",
        stakes: "high",
        dependencies: [],
        state: "scheduled",
        upstreamBlocked: false,
        attempts: 0,
        scheduledAt: "2026-07-30T03:00:00Z",
        releaseReason: "release window has not opened",
      },
      {
        id: "collect-evidence",
        capability: "local",
        acceptance: "evidence-collected",
        stakes: "normal",
        dependencies: [],
        state: "ready",
        upstreamBlocked: false,
        assignedAgent: "coder",
        attempts: 1,
        sessionId: "goalchainer-7",
      },
      {
        id: "publish",
        capability: "specialist",
        acceptance: "update-published",
        stakes: "high",
        dependencies: ["collect-evidence"],
        state: "blocked",
        upstreamBlocked: true,
        attempts: 0,
      },
    ],
    audit: { events: 3, head: digest },
    updatedAt: "2026-07-29T04:00:00Z",
    projectId: "goalchainer",
  };
}

beforeEach(() => navigateMock.mockReset());

it("renders MeTTa task states as a read-only six-lane board", () => {
  render(
    <GoalChainerBoard projectId="goalchainer" projection={projection()} />,
  );

  expect(screen.getByTestId("goalchainer-board")).toBeInTheDocument();
  for (const name of [
    "Scheduled",
    "Backlog",
    "Ready",
    "In progress",
    "Blocked",
    "Done",
  ]) {
    expect(screen.getByRole("heading", { name })).toBeInTheDocument();
  }
  expect(screen.getByText("MeTTaScript authority")).toBeInTheDocument();
  expect(screen.getByText("Waiting on dependencies")).toBeInTheDocument();
  expect(
    screen.getByText("not-before constraint has not elapsed"),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /complete|block|claim/i }),
  ).not.toBeInTheDocument();
});

it("orders scheduled cards by release time and opens linked AO sessions", () => {
  render(
    <GoalChainerBoard projectId="goalchainer" projection={projection()} />,
  );

  const scheduled = screen.getByRole("heading", { name: "Scheduled" })
    .parentElement?.parentElement;
  if (!scheduled) throw new Error("missing Scheduled lane");
  const cards = within(scheduled).getAllByRole("button");
  expect(cards[0]).toHaveTextContent("earlier-high");
  expect(cards[1]).toHaveTextContent("later-normal");

  fireEvent.click(screen.getByRole("button", { name: /^collect-evidence/i }));
  expect(navigateMock).toHaveBeenCalledWith({
    to: "/projects/$projectId/sessions/$sessionId",
    params: { projectId: "goalchainer", sessionId: "goalchainer-7" },
  });
});

it("shows empty lanes without creating lifecycle controls", () => {
  const empty = projection();
  empty.tasks = [];
  empty.summary = {
    total: 0,
    scheduled: 0,
    backlog: 0,
    ready: 0,
    inProgress: 0,
    blocked: 0,
    done: 0,
    progressPct: 0,
  };

  render(<GoalChainerBoard projectId="goalchainer" projection={empty} />);

  expect(screen.getAllByText("No tasks")).toHaveLength(6);
  expect(screen.queryByRole("button")).not.toBeInTheDocument();
});

it("marks a projection stale from AO's persisted update time", () => {
  vi.useFakeTimers();
  try {
    vi.setSystemTime(new Date("2026-07-29T04:03:00Z"));
    render(
      <GoalChainerBoard projectId="goalchainer" projection={projection()} />,
    );

    const freshness = screen.getByText("source stale");
    expect(freshness).toHaveAttribute("data-stale", "true");
    expect(freshness).toHaveAttribute(
      "title",
      expect.stringContaining("Last GoalChainer sync"),
    );
  } finally {
    vi.useRealTimers();
  }
});
