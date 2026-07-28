import { expect, it, vi } from "vitest";
import type { GoalChainerProjection } from "../hooks/useGoalChainerQuery";
import { createGoalChainerAgentSpace } from "./goalchainer-atomspace";

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
		usageTokens: 0,
		summary: {
			total: 2,
			scheduled: 1,
			backlog: 0,
			ready: 1,
			inProgress: 0,
			blocked: 0,
			done: 0,
			progressPct: 0,
		},
		tasks: [
			{
				id: "later",
				capability: "local",
				acceptance: "later-complete",
				stakes: "normal",
				dependencies: [],
				state: "scheduled",
				upstreamBlocked: false,
				attempts: 0,
				scheduledAt: "2026-07-30T04:00:00Z",
			},
			{
				id: "collect",
				capability: "local",
				acceptance: "evidence-collected",
				stakes: "high",
				dependencies: [],
				state: "ready",
				upstreamBlocked: false,
				assignedAgent: "coder",
				attempts: 1,
				sessionId: "goalchainer-7",
			},
		],
		audit: { events: 1, head: digest },
		projectId: "goalchainer",
	};
}

it("selects board tasks and grounded agents through MeTTaScript", () => {
	const space = createGoalChainerAgentSpace(projection(), vi.fn());

	expect(space.tasksForState("scheduled").map((task) => task.id)).toEqual([
		"later",
	]);
	expect(space.tasksForState("ready").map((task) => task.id)).toEqual([
		"collect",
	]);
	expect(space.agentForTask("collect")).toEqual({
		id: "coder",
		taskIds: ["collect"],
		sessionIds: ["goalchainer-7"],
	});
	expect(space.agentForTask("later")).toBeUndefined();
});

it("opens a linked Agent Orchestrator session through a grounded operation", () => {
	const openSession = vi.fn();
	const space = createGoalChainerAgentSpace(projection(), openSession);

	expect(space.openTaskSession("collect")).toBe(true);
	expect(openSession).toHaveBeenCalledWith("goalchainer-7");
	expect(space.openTaskSession("later")).toBe(false);
	expect(openSession).toHaveBeenCalledTimes(1);
});
