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
			agentKey: "security-specialist",
			role: "specialist",
			agentId: "272a4c63-64b9-47a0-9624-7fc0fff46aa1",
		},
		{
			agentKey: "result-evaluator",
			role: "evaluator",
		},
	];
	value.tasks[1] = {
		...value.tasks[1],
		assignedAgent: "local-worker",
		assignedRole: "worker",
		agentKey: "local-worker",
		agentId: "7e66da4a-2b4f-4e87-865f-14f554070faa",
		attempts: 2,
		sessionId: "goalchainer-7",
		sessionAttempts: [
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
		],
		atomspaceLinks: [
			{ linkKey: "project-shared", scope: "shared", owner: "project" },
			{
				linkKey: "worker-private",
				scope: "private",
				owner: "local-worker",
			},
		],
	};
	return value;
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

it("grounds the v2 top-level registry including an unbound stable agent", () => {
	const space = createGoalChainerAgentSpace(v2Projection(), vi.fn());

	expect(space.agents()).toContainEqual({
		id: "result-evaluator",
		agentKey: "result-evaluator",
		role: "evaluator",
		taskIds: [],
		sessionIds: [],
	});
	expect(space.agentForTask("collect")).toEqual({
		id: "local-worker",
		agentKey: "local-worker",
		role: "worker",
		agentId: "7e66da4a-2b4f-4e87-865f-14f554070faa",
		taskIds: ["collect"],
		sessionIds: ["goalchainer-3", "goalchainer-7"],
	});
	expect(space.agentForTask("later")).toBeUndefined();
});

it("opens every v2 session attempt through MeTTa attempt relations", () => {
	const openSession = vi.fn();
	const space = createGoalChainerAgentSpace(v2Projection(), openSession);

	expect(space.openTaskAttemptSession("collect", 1)).toBe(true);
	expect(space.openTaskAttemptSession("collect", 2)).toBe(true);
	expect(space.openTaskAttemptSession("collect", 3)).toBe(false);
	expect(openSession.mock.calls).toEqual([
		["goalchainer-3"],
		["goalchainer-7"],
	]);
});
