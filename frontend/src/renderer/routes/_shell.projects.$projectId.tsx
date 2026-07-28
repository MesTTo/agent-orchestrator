import { createFileRoute } from "@tanstack/react-router";
import { useState, type ReactNode } from "react";
import { GoalChainerBoard } from "../components/GoalChainerBoard";
import { SessionsBoard } from "../components/SessionsBoard";
import { useGoalChainerQuery } from "../hooks/useGoalChainerQuery";
import { cn } from "../lib/utils";

export const Route = createFileRoute("/_shell/projects/$projectId")({
	component: ProjectBoardRoute,
});

function ProjectBoardRoute() {
	const { projectId } = Route.useParams();
	const goalChainer = useGoalChainerQuery(projectId);
	const [selectedView, setSelectedView] = useState<"agents" | "goalchainer" | null>(null);
	const view = selectedView ?? (goalChainer.data ? "goalchainer" : "agents");
	return (
		<div className="flex h-full min-h-0 flex-col">
			<div className="flex h-9 shrink-0 items-center gap-1 border-b border-border-strong bg-surface px-3">
				<ViewButton active={view === "agents"} onClick={() => setSelectedView("agents")}>
					Agent sessions
				</ViewButton>
				<ViewButton active={view === "goalchainer"} onClick={() => setSelectedView("goalchainer")}>
					GoalChainer
					{goalChainer.data ? (
						<span className="ml-1 rounded bg-accent/15 px-1 text-[10px] text-accent">
							{goalChainer.data.summary.total}
						</span>
					) : null}
				</ViewButton>
			</div>
			<div className="min-h-0 flex-1">
				{view === "agents" ? (
					<SessionsBoard projectId={projectId} />
				) : goalChainer.isLoading ? (
					<p className="py-10 text-center text-xs text-passive">Loading GoalChainer board...</p>
				) : goalChainer.isError ? (
					<div className="mx-auto max-w-lg px-6 py-12 text-center">
						<h1 className="text-sm font-medium">GoalChainer board unavailable</h1>
						<p className="mt-2 text-xs text-muted-foreground">
							Agent sessions remain available. The next successful GoalChainer sync will restore this view.
						</p>
					</div>
				) : goalChainer.data ? (
					<GoalChainerBoard projectId={projectId} projection={goalChainer.data} />
				) : (
					<div className="mx-auto max-w-lg px-6 py-12 text-center">
						<h1 className="text-sm font-medium">GoalChainer is not connected</h1>
						<p className="mt-2 text-xs text-muted-foreground">
							Run the GoalChainer UI sync command for this AO project. AO will display the resulting read-only MeTTa task projection here.
						</p>
					</div>
				)}
			</div>
		</div>
	);
}

function ViewButton({
	active,
	children,
	onClick,
}: {
	active: boolean;
	children: ReactNode;
	onClick: () => void;
}) {
	return (
		<button
			className={cn(
				"rounded px-2.5 py-1 text-xs text-muted-foreground",
				active && "bg-background text-foreground shadow-sm",
			)}
			onClick={onClick}
			type="button"
		>
			{children}
		</button>
	);
}
