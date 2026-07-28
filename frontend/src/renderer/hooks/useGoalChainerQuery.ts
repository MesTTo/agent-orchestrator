import { useQuery } from "@tanstack/react-query";
import type { components } from "../../api/schema";
import { apiClient, apiErrorCode, hasTrustedApiBaseUrl } from "../lib/api-client";

export type GoalChainerProjection = components["schemas"]["GoalChainerProjection"];
export type GoalChainerTask = components["schemas"]["GoalChainerTask"];

export const goalChainerQueryKey = (projectId: string) => ["goalchainer", projectId] as const;

async function fetchGoalChainerProjection(projectId: string): Promise<GoalChainerProjection | null> {
	if (!hasTrustedApiBaseUrl()) return null;
	const { data, error } = await apiClient.GET("/api/v1/projects/{id}/goalchainer", {
		params: { path: { id: projectId } },
	});
	if (error) {
		if (apiErrorCode(error) === "GOALCHAINER_NOT_CONNECTED") return null;
		throw error;
	}
	return data ?? null;
}

export function useGoalChainerQuery(projectId: string) {
	return useQuery({
		queryKey: goalChainerQueryKey(projectId),
		queryFn: () => fetchGoalChainerProjection(projectId),
		retry: 1,
		refetchInterval: 30_000,
	});
}
