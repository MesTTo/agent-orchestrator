import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";

const {
	navigateMock,
	notesQueryMock,
	projectAgentsQueryMock,
	workspaceQueryMock,
} = vi.hoisted(() => ({
	navigateMock: vi.fn(),
	notesQueryMock: vi.fn(),
	projectAgentsQueryMock: vi.fn(),
	workspaceQueryMock: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
	useNavigate: () => navigateMock,
}));

vi.mock("../hooks/useProjectAgentsQuery", () => ({
	useProjectAgentsQuery: projectAgentsQueryMock,
	useProjectAgentNotesQuery: notesQueryMock,
}));

vi.mock("../hooks/useWorkspaceQuery", () => ({
	useWorkspaceQuery: workspaceQueryMock,
}));

import { ProjectAgentsPanel } from "./ProjectAgentsPanel";

function expectSessionNavigation(sessionId: string) {
	expect(navigateMock).toHaveBeenCalledWith({
		to: "/projects/$projectId/sessions/$sessionId",
		params: { projectId: "research", sessionId },
	});
}

const agent = {
	id: "7e66da4a-2b4f-4e87-865f-14f554070faa",
	projectId: "research",
	projectAgentKey: "evidence-worker",
	role: "researcher",
	notes: {
		exists: true,
		initialized: true,
		sizeBytes: 180,
		modifiedAt: "2026-07-29T04:00:00Z",
	},
	attempts: [
		{
			number: 2,
			sessionId: "research-7",
			createdAt: "2026-07-29T04:00:00Z",
		},
		{
			number: 1,
			sessionId: "research-3",
			createdAt: "2026-07-29T03:30:00Z",
		},
	],
	createdAt: "2026-07-29T03:00:00Z",
	updatedAt: "2026-07-29T03:00:00Z",
};

beforeEach(() => {
	navigateMock.mockReset();
	projectAgentsQueryMock.mockReset().mockReturnValue({
		data: [agent],
		isLoading: false,
		isError: false,
	});
	workspaceQueryMock.mockReset().mockReturnValue({
		data: [
			{
				id: "research",
				sessions: [
					{
						id: "research-7",
						workspaceId: "research",
						title: "Collect evidence",
						status: "working",
					},
					{
						id: "research-3",
						workspaceId: "research",
						title: "Initial survey",
						status: "terminated",
					},
				],
			},
		],
	});
	notesQueryMock.mockReset().mockImplementation(
		(_projectId: string, _agentId: string, enabled: boolean) =>
			enabled
				? {
						data: {
							agentId: agent.id,
							content:
								'(MemorySnapshot oh-my-goals-atomspace-v1 agent "research" "7e66da4a-2b4f-4e87-865f-14f554070faa")\n',
							truncated: false,
							initialized: true,
							propositionCount: 3,
							sizeBytes: 180,
							modifiedAt: "2026-07-29T04:00:00Z",
						},
						isLoading: false,
						isError: false,
					}
				: { data: undefined, isLoading: false, isError: false },
	);
});

it("shows stable identity, role, notes metadata, and attempt count", () => {
	render(<ProjectAgentsPanel projectId="research" />);

	expect(screen.getByTestId("project-agents-panel")).toBeInTheDocument();
	expect(screen.getByTestId(`project-agent-${agent.id}`)).toHaveAttribute(
		"id",
		`project-agent-${agent.id}`,
	);
	expect(screen.getByText("evidence-worker")).toBeInTheDocument();
	expect(screen.getByText("researcher")).toBeInTheDocument();
	expect(screen.getByText("2 attempts")).toBeInTheDocument();
	expect(screen.getByText("180 B notes")).toBeInTheDocument();
	expect(notesQueryMock).toHaveBeenLastCalledWith("research", agent.id, false);
});

it("loads notes only after expansion and renders every attempt latest-first", () => {
	render(<ProjectAgentsPanel projectId="research" />);

	fireEvent.click(screen.getByRole("button", { name: /evidence-worker/i }));

	expect(notesQueryMock).toHaveBeenLastCalledWith("research", agent.id, true);
	expect(screen.getByText("3 propositions")).toBeInTheDocument();
	expect(screen.getByText(/MemorySnapshot/)).toBeInTheDocument();
	expect(screen.getByText(/Collect evidence/)).toBeInTheDocument();
	expect(screen.getByText(/Initial survey/)).toBeInTheDocument();
	expect(screen.getByText(/terminated/)).toBeInTheDocument();
	const attemptLabels = screen.getAllByText(/Attempt [12]/).map((element) => element.textContent);
	expect(attemptLabels[0]).toContain("Attempt 2");
	expect(attemptLabels[1]).toContain("Attempt 1");

	fireEvent.click(screen.getByRole("button", { name: "Open attempt 1 session" }));
	expectSessionNavigation("research-3");
});

it("keeps an immutable attempt visible and navigable when its workspace session is absent", () => {
	workspaceQueryMock.mockReturnValue({
		data: [{ id: "research", sessions: [] }],
	});

	render(<ProjectAgentsPanel projectId="research" />);
	fireEvent.click(screen.getByRole("button", { name: /evidence-worker/i }));

	expect(screen.getAllByText(/session unavailable/)).toHaveLength(2);
	expect(screen.getAllByText(/research-3/).length).toBeGreaterThan(0);
	fireEvent.click(screen.getByRole("button", { name: "Open attempt 1 session" }));
	expectSessionNavigation("research-3");
});

it("pluralizes a single attempt", () => {
	projectAgentsQueryMock.mockReturnValue({
		data: [{ ...agent, attempts: [agent.attempts[0]] }],
		isLoading: false,
		isError: false,
	});

	render(<ProjectAgentsPanel projectId="research" />);

	expect(screen.getByText("1 attempt")).toBeInTheDocument();
});

it("keeps the sessions surface usable when no stable agents exist", () => {
	projectAgentsQueryMock.mockReturnValue({
		data: [],
		isLoading: false,
		isError: false,
	});

	render(<ProjectAgentsPanel projectId="research" />);

	expect(screen.getByText(/No stable agents yet/)).toBeInTheDocument();
});
