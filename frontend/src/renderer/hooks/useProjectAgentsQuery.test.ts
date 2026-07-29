import { beforeEach, expect, it, vi } from "vitest";

const useQueryMock = vi.hoisted(() => vi.fn());

vi.mock("@tanstack/react-query", () => ({
	useQuery: useQueryMock,
}));

import { useProjectAgentNotesQuery } from "./useProjectAgentsQuery";

beforeEach(() => {
	useQueryMock.mockReset().mockReturnValue({});
});

it("polls expanded notes so external Atomspace writes become visible", () => {
	useProjectAgentNotesQuery("research", "agent-1", true);

	expect(useQueryMock).toHaveBeenCalledWith(
		expect.objectContaining({
			enabled: true,
			refetchInterval: 15_000,
		}),
	);
});

it("does not poll notes while the agent card is collapsed", () => {
	useProjectAgentNotesQuery("research", "agent-1", false);

	expect(useQueryMock).toHaveBeenCalledWith(
		expect.objectContaining({
			enabled: false,
			refetchInterval: false,
		}),
	);
});
