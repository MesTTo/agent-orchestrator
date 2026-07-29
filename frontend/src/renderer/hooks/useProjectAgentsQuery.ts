import { useQuery } from "@tanstack/react-query";
import type { components } from "../../api/schema";
import { apiClient, hasTrustedApiBaseUrl } from "../lib/api-client";

export type ProjectAgent = components["schemas"]["ProjectAgent"];
export type ProjectAgentNotes = components["schemas"]["ProjectAgentNotes"];

export const projectAgentsQueryKey = (projectId?: string) =>
	projectId ? (["project-agents", projectId] as const) : (["project-agents"] as const);

export const projectAgentNotesQueryKey = (projectId: string, agentId: string) =>
	["project-agent-notes", projectId, agentId] as const;

async function fetchProjectAgents(projectId: string): Promise<ProjectAgent[]> {
	if (!hasTrustedApiBaseUrl()) return [];
	const { data, error } = await apiClient.GET("/api/v1/projects/{id}/agents", {
		params: { path: { id: projectId } },
	});
	if (error) throw error;
	return data?.agents ?? [];
}

async function fetchProjectAgentNotes(
	projectId: string,
	agentId: string,
): Promise<ProjectAgentNotes> {
	const { data, error } = await apiClient.GET(
		"/api/v1/projects/{id}/agents/{agentId}/notes",
		{ params: { path: { id: projectId, agentId } } },
	);
	if (error) throw error;
	if (!data) throw new Error("Project agent notes response was empty");
	return data.notes;
}

export function useProjectAgentsQuery(projectId: string) {
	return useQuery({
		queryKey: projectAgentsQueryKey(projectId),
		queryFn: () => fetchProjectAgents(projectId),
		retry: 1,
		refetchInterval: 15_000,
	});
}

export function useProjectAgentNotesQuery(
	projectId: string,
	agentId: string,
	enabled: boolean,
) {
	return useQuery({
		queryKey: projectAgentNotesQueryKey(projectId, agentId),
		queryFn: () => fetchProjectAgentNotes(projectId, agentId),
		enabled,
		retry: 1,
		staleTime: 5_000,
		refetchInterval: enabled ? 15_000 : false,
	});
}
