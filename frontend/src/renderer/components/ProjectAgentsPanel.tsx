import { useNavigate } from "@tanstack/react-router";
import { Bot, ChevronDown, ChevronRight, FileText, Link2 } from "lucide-react";
import { useState } from "react";
import {
	type ProjectAgent,
	useProjectAgentNotesQuery,
	useProjectAgentsQuery,
} from "../hooks/useProjectAgentsQuery";
import { useWorkspaceQuery } from "../hooks/useWorkspaceQuery";
import { cn } from "../lib/utils";
import type { WorkspaceSession } from "../types/workspace";

type ProjectAgentsPanelProps = {
	projectId: string;
};

export function ProjectAgentsPanel({ projectId }: ProjectAgentsPanelProps) {
	const agents = useProjectAgentsQuery(projectId);
	const workspace = useWorkspaceQuery().data?.find((candidate) => candidate.id === projectId);
	const sessions = workspace?.sessions ?? [];

	return (
		<section
			className="shrink-0 border-b border-border-strong bg-surface"
			data-testid="project-agents-panel"
		>
			<header className="flex h-10 items-center justify-between px-4">
				<div className="flex items-center gap-2">
					<Bot className="size-icon-sm text-accent" aria-hidden="true" />
					<h2 className="text-xs font-medium">Stable project agents</h2>
					{agents.data ? (
						<span className="rounded bg-background px-1.5 py-0.5 text-[10px] tabular-nums text-muted-foreground">
							{agents.data.length}
						</span>
					) : null}
				</div>
				<p className="text-[10px] text-passive">Project-owned MeTTa notes and session attempts</p>
			</header>

			{agents.isLoading ? (
				<p className="px-4 pb-3 text-[11px] text-passive">Loading stable agents...</p>
			) : agents.isError ? (
				<p className="px-4 pb-3 text-[11px] text-destructive">
					Stable agents are unavailable. Agent sessions remain usable.
				</p>
			) : agents.data?.length ? (
				<div className="flex gap-2 overflow-x-auto px-3 pb-3">
					{agents.data.map((agent) => (
						<ProjectAgentCard
							agent={agent}
							key={agent.id}
							projectId={projectId}
							sessions={sessions}
						/>
					))}
				</div>
			) : (
				<p className="px-4 pb-3 text-[11px] text-passive">
					No stable agents yet. A GoalChainer dispatch creates the first identity.
				</p>
			)}
		</section>
	);
}

function ProjectAgentCard({
	agent,
	projectId,
	sessions,
}: {
	agent: ProjectAgent;
	projectId: string;
	sessions: WorkspaceSession[];
}) {
	const navigate = useNavigate();
	const [expanded, setExpanded] = useState(false);
	const notes = useProjectAgentNotesQuery(projectId, agent.id, expanded);
	const attempts = [...agent.attempts].sort((left, right) => right.number - left.number);

	const openSession = (sessionId: string) => {
		void navigate({
			to: "/projects/$projectId/sessions/$sessionId",
			params: { projectId, sessionId },
		});
	};

	return (
		<article
			className={cn(
				"w-72 shrink-0 rounded-md border border-border bg-background",
				expanded && "w-[30rem]",
			)}
			data-testid={`project-agent-${agent.id}`}
			id={`project-agent-${agent.id}`}
		>
			<button
				className="flex w-full items-start gap-2 p-3 text-left"
				onClick={() => setExpanded((current) => !current)}
				type="button"
			>
				{expanded ? (
					<ChevronDown className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
				) : (
					<ChevronRight className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
				)}
				<div className="min-w-0 flex-1">
					<div className="flex items-center justify-between gap-2">
						<h3 className="truncate text-xs font-medium">{agent.projectAgentKey}</h3>
						<span className="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] text-accent">
							{agent.role}
						</span>
					</div>
					<p className="mt-1 truncate font-mono text-[10px] text-passive" title={agent.id}>
						{agent.id}
					</p>
					<div className="mt-2 flex items-center justify-between text-[10px] text-muted-foreground">
						<span>
							{agent.attempts.length} {agent.attempts.length === 1 ? "attempt" : "attempts"}
						</span>
						<span>{agent.notes.initialized ? `${agent.notes.sizeBytes} B notes` : "notes uninitialized"}</span>
					</div>
				</div>
			</button>

			{expanded ? (
				<div className="border-t border-border px-3 py-2">
					<div>
						<p className="mb-1 text-[10px] font-medium text-muted-foreground">Session attempts</p>
						{attempts.length ? (
							<ol className="max-h-32 space-y-1 overflow-auto">
								{attempts.map((attempt) => {
									const session = sessions.find(
										(candidate) => candidate.id === attempt.sessionId,
									);
									return (
										<li
											className="flex items-center justify-between gap-2 rounded border border-border bg-surface px-2 py-1.5"
											key={attempt.number}
										>
											<div className="min-w-0 text-[10px] text-muted-foreground">
												<p className="truncate">
													Attempt {attempt.number}
													<span className="px-1">·</span>
													{session?.title ?? attempt.sessionId}
												</p>
												<p className="truncate font-mono text-[9px] text-passive">
													{attempt.sessionId}
													<span className="px-1">·</span>
													{session?.status ?? "session unavailable"}
												</p>
											</div>
											<button
												aria-label={`Open attempt ${attempt.number} session`}
												className="flex shrink-0 items-center gap-1 rounded px-1.5 py-1 text-[10px] text-accent hover:bg-accent/10"
												onClick={() => openSession(attempt.sessionId)}
												type="button"
											>
												<Link2 className="size-3" aria-hidden="true" />
												Open
											</button>
										</li>
									);
								})}
							</ol>
						) : (
							<p className="text-[10px] text-passive">No session attempts</p>
						)}
					</div>

					<div className="mt-2 rounded border border-border bg-surface p-2">
						<div className="mb-1 flex items-center justify-between text-[10px] text-muted-foreground">
							<span className="flex items-center gap-1">
								<FileText className="size-3" aria-hidden="true" />
								Private MeTTa notes
							</span>
							{notes.data ? (
								<span>
									{notes.data.propositionCount} propositions
									{notes.data.truncated ? " · bounded view" : ""}
								</span>
							) : null}
						</div>
						{notes.isLoading ? (
							<p className="py-2 text-[10px] text-passive">Loading notes...</p>
						) : notes.isError ? (
							<p className="py-2 text-[10px] text-destructive">
								Notes could not be read or did not match the supported snapshot schema.
							</p>
						) : notes.data?.content ? (
							<pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words font-mono text-[10px] leading-4 text-muted-foreground">
								{notes.data.content}
							</pre>
						) : (
							<p className="py-2 text-[10px] text-passive">No private notes recorded yet.</p>
						)}
					</div>
				</div>
			) : null}
		</article>
	);
}
