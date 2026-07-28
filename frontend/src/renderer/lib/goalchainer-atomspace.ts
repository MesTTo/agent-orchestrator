import {
	E,
	MeTTa,
	S,
	ValueAtom,
	atomToJs,
	type Atom,
} from "@mettascript/hyperon";
import type {
	GoalChainerProjection,
	GoalChainerTask,
} from "../hooks/useGoalChainerQuery";
import goalChainerUiSource from "../metta/goalchainer-ui.metta?raw";

export type GoalChainerAgent = Readonly<{
	id: string;
	taskIds: readonly string[];
	sessionIds: readonly string[];
}>;

export type GoalChainerAgentSpace = Readonly<{
	tasksForState: (state: GoalChainerTask["state"]) => GoalChainerTask[];
	agentForTask: (taskId: string) => GoalChainerAgent | undefined;
	openTaskSession: (taskId: string) => boolean;
}>;

export function createGoalChainerAgentSpace(
	projection: GoalChainerProjection,
	openSession: (sessionId: string) => void,
): GoalChainerAgentSpace {
	const metta = new MeTTa();
	metta.registerOperation("ao-open-session", (args) => {
		const sessionId = stringValue(args[0]);
		if (sessionId === undefined) return [ValueAtom(false)];
		openSession(sessionId);
		return [ValueAtom(true)];
	});
	metta.run(goalChainerUiSource);

	const space = metta.space();
	space.addAtom(E(S("gc-projection-record"), ValueAtom(projection, "GoalChainerProjection")));
	for (const task of projection.tasks) {
		space.addAtom(
			E(
				S("gc-task-record"),
				ValueAtom(task.id),
				ValueAtom(task.state),
				ValueAtom(task, "GoalChainerTask"),
			),
		);
		if (task.sessionId) {
			space.addAtom(
				E(
					S("gc-task-session"),
					ValueAtom(task.id),
					ValueAtom(task.sessionId),
				),
			);
		}
	}
	for (const agent of agentsFromProjection(projection)) {
		space.addAtom(
			E(
				S("gc-agent-record"),
				ValueAtom(agent.id),
				ValueAtom(agent, "GoalChainerAgent"),
			),
		);
		for (const taskId of agent.taskIds) {
			space.addAtom(
				E(
					S("gc-agent-task"),
					ValueAtom(agent.id),
					ValueAtom(taskId),
				),
			);
		}
	}

	return {
		tasksForState: (state) =>
			metta
				.evaluateAtom(E(S("gc-ui-task"), ValueAtom(state)))
				.map(atomToJs)
				.filter(isGoalChainerTask)
				.sort(compareTasks),
		agentForTask: (taskId) => {
			const results = metta
				.evaluateAtom(E(S("gc-ui-agent-for-task"), ValueAtom(taskId)))
				.map(atomToJs)
				.filter(isGoalChainerAgent);
			return results[0];
		},
		openTaskSession: (taskId) => {
			const results = metta
				.evaluateAtom(E(S("gc-ui-open-session"), ValueAtom(taskId)))
				.map(atomToJs);
			return results.includes(true);
		},
	};
}

function agentsFromProjection(
	projection: GoalChainerProjection,
): GoalChainerAgent[] {
	const tasksByAgent = new Map<string, GoalChainerTask[]>();
	for (const task of projection.tasks) {
		if (!task.assignedAgent) continue;
		const tasks = tasksByAgent.get(task.assignedAgent) ?? [];
		tasks.push(task);
		tasksByAgent.set(task.assignedAgent, tasks);
	}
	return [...tasksByAgent]
		.map(([id, tasks]) => ({
			id,
			taskIds: tasks.map((task) => task.id).sort(),
			sessionIds: tasks
				.flatMap((task) => (task.sessionId ? [task.sessionId] : []))
				.sort(),
		}))
		.sort((left, right) => left.id.localeCompare(right.id));
}

function stringValue(atom: Atom | undefined): string | undefined {
	if (atom === undefined) return undefined;
	const value = atomToJs(atom);
	return typeof value === "string" && value.length > 0 ? value : undefined;
}

function isGoalChainerTask(value: unknown): value is GoalChainerTask {
	return (
		typeof value === "object" &&
		value !== null &&
		typeof (value as GoalChainerTask).id === "string" &&
		typeof (value as GoalChainerTask).state === "string"
	);
}

function isGoalChainerAgent(value: unknown): value is GoalChainerAgent {
	return (
		typeof value === "object" &&
		value !== null &&
		typeof (value as GoalChainerAgent).id === "string" &&
		Array.isArray((value as GoalChainerAgent).taskIds) &&
		Array.isArray((value as GoalChainerAgent).sessionIds)
	);
}

function compareTasks(
	left: GoalChainerTask,
	right: GoalChainerTask,
): number {
	const leftSchedule = left.scheduledAt
		? Date.parse(left.scheduledAt)
		: Number.POSITIVE_INFINITY;
	const rightSchedule = right.scheduledAt
		? Date.parse(right.scheduledAt)
		: Number.POSITIVE_INFINITY;
	if (leftSchedule !== rightSchedule) return leftSchedule - rightSchedule;
	if (left.stakes !== right.stakes) return left.stakes === "high" ? -1 : 1;
	return left.id.localeCompare(right.id);
}
