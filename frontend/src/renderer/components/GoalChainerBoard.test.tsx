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

function v2Projection(): GoalChainerProjection {
  const value = projection();
  value.protocol = "goalchainer-ui-v2";
  value.projectId = "goalchainer";
  value.agents = [
    {
      agentKey: "local-worker",
      role: "worker",
      agentId: "7e66da4a-2b4f-4e87-865f-14f554070faa",
    },
    {
      agentKey: "old-worker",
      role: "reviewer",
      agentId: "272a4c63-64b9-47a0-9624-7fc0fff46aa1",
    },
  ];
  const task = value.tasks[2];
  task.assignedAgent = "local-worker";
  task.assignedRole = "worker";
  task.agentKey = "local-worker";
  task.agentId = "7e66da4a-2b4f-4e87-865f-14f554070faa";
  task.attempts = 2;
  task.sessionAttempts = [
    {
      attempt: 1,
      agentId: "272a4c63-64b9-47a0-9624-7fc0fff46aa1",
      agentAttempt: 1,
      sessionId: "goalchainer-3",
      status: "terminated",
      activity: "exited",
      isTerminated: true,
    },
    {
      attempt: 2,
      agentId: "7e66da4a-2b4f-4e87-865f-14f554070faa",
      agentAttempt: 4,
      sessionId: "goalchainer-7",
      status: "working",
      activity: "active",
      isTerminated: false,
    },
  ];
  task.atomspaceLinks = [
    { linkKey: "project-shared", scope: "shared", owner: "project" },
    {
      linkKey: "worker-private",
      scope: "private",
      owner: "local-worker",
    },
  ];
  return value;
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
  const legacyCard = screen.getByTestId("goal-task-collect-evidence");
  expect(legacyCard).toHaveTextContent("coder");
  expect(legacyCard).not.toHaveTextContent("Role");
  expect(legacyCard).not.toHaveTextContent("Goal attempts");
});

it("orders scheduled cards by release time and opens linked AO sessions", () => {
  render(
    <GoalChainerBoard projectId="goalchainer" projection={projection()} />,
  );

  const scheduled = screen.getByRole("heading", { name: "Scheduled" })
    .parentElement?.parentElement;
  if (!scheduled) throw new Error("missing Scheduled lane");
  const cards = within(scheduled).getAllByTestId(/goal-task-/);
  expect(cards[0]).toHaveTextContent("earlier-high");
  expect(cards[1]).toHaveTextContent("later-normal");

  fireEvent.click(
    screen.getByRole("button", { name: "Open collect-evidence session" }),
  );
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

it("renders v2 stable identity, separate attempts, links, and every session", () => {
  render(
    <GoalChainerBoard projectId="goalchainer" projection={v2Projection()} />,
  );

  const card = screen.getByTestId("goal-task-collect-evidence");
  expect(card).toHaveTextContent("Role worker");
  expect(card).toHaveTextContent("Agent local-worker");
  expect(card).toHaveTextContent(
    "7e66da4a-2b4f-4e87-865f-14f554070faa",
  );
  expect(card).toHaveTextContent("Goal attempts 2 · AO history 2");
  expect(card).toHaveTextContent("Goal 2 · AO 4 · working");
  expect(card).toHaveTextContent("Goal 1 · AO 1 · terminated");
  expect(card).toHaveTextContent("project-shared · shared · project");
  expect(card).toHaveTextContent(
    "worker-private · private · local-worker",
  );
  expect(
    screen.getByRole("link", { name: "View stable agent local-worker" }),
  ).toHaveAttribute(
    "href",
    "#project-agent-7e66da4a-2b4f-4e87-865f-14f554070faa",
  );

  fireEvent.click(
    screen.getByRole("button", {
      name: "Open collect-evidence Goal attempt 1 session",
    }),
  );
  fireEvent.click(
    screen.getByRole("button", {
      name: "Open collect-evidence Goal attempt 2 session",
    }),
  );
  expect(navigateMock.mock.calls).toEqual([
    [
      {
        to: "/projects/$projectId/sessions/$sessionId",
        params: { projectId: "goalchainer", sessionId: "goalchainer-3" },
      },
    ],
    [
      {
        to: "/projects/$projectId/sessions/$sessionId",
        params: { projectId: "goalchainer", sessionId: "goalchainer-7" },
      },
    ],
  ]);
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
