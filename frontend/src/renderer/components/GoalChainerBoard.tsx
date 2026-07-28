import { useNavigate } from "@tanstack/react-router";
import { CalendarClock, CircleAlert, Link2, ShieldCheck } from "lucide-react";
import { useMemo } from "react";
import type {
  GoalChainerProjection,
  GoalChainerTask,
} from "../hooks/useGoalChainerQuery";
import { createGoalChainerAgentSpace } from "../lib/goalchainer-atomspace";
import { cn } from "../lib/utils";

type GoalChainerBoardProps = {
  projectId: string;
  projection: GoalChainerProjection;
};

type TaskState = GoalChainerTask["state"];
const SOURCE_STALE_AFTER_MS = 2 * 60 * 1000;

const columns: { state: TaskState; label: string }[] = [
  { state: "scheduled", label: "Scheduled" },
  { state: "backlog", label: "Backlog" },
  { state: "ready", label: "Ready" },
  { state: "in-progress", label: "In progress" },
  { state: "blocked", label: "Blocked" },
  { state: "done", label: "Done" },
];

export function GoalChainerBoard({
  projectId,
  projection,
}: GoalChainerBoardProps) {
  const navigate = useNavigate();
  const freshness = projectionFreshness(projection.updatedAt);
  const agentSpace = useMemo(
    () =>
      createGoalChainerAgentSpace(projection, (sessionId) => {
        void navigate({
          to: "/projects/$projectId/sessions/$sessionId",
          params: { projectId, sessionId },
        });
      }),
    [navigate, projectId, projection],
  );

  return (
    <section
      className="flex h-full min-h-0 flex-col bg-background text-foreground"
      data-testid="goalchainer-board"
    >
      <header className="flex shrink-0 items-center gap-4 border-b border-border-strong px-4 py-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <ShieldCheck
              className="size-icon-md text-accent"
              aria-hidden="true"
            />
            <h1 className="truncate text-sm font-medium">{projection.title}</h1>
            <span className="rounded border border-border px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-muted-foreground">
              MeTTaScript authority
            </span>
          </div>
          <p className="mt-1 truncate text-xs text-muted-foreground">
            {projection.threadId} · {projection.owner} ·{" "}
            {projection.summary.done}/{projection.summary.total} done
          </p>
        </div>
        <div className="text-right text-xs text-muted-foreground">
          <div>{projection.summary.progressPct}% complete</div>
          <div
            className={freshness.stale ? "text-warning" : undefined}
            data-stale={freshness.stale}
            title={freshness.title}
          >
            {freshness.label}
          </div>
          <div title={projection.sourceRevision}>
            revision {projection.sourceRevision.slice(0, 8)}
          </div>
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-x-auto overflow-y-hidden">
        <div className="grid h-full min-w-[90rem] grid-cols-6 divide-x divide-border-strong">
          {columns.map((column) => {
            const tasks = agentSpace.tasksForState(column.state);
            return (
              <div className="flex min-h-0 flex-col" key={column.state}>
                <div className="flex h-12 shrink-0 items-center justify-between border-b border-border-strong px-3">
                  <h2 className="text-xs font-medium">{column.label}</h2>
                  <span className="text-xs tabular-nums text-muted-foreground">
                    {tasks.length}
                  </span>
                </div>
                <div className="min-h-0 flex-1 space-y-2 overflow-y-auto p-2">
                  {tasks.map((task) => (
                    <GoalTaskCard
                      key={task.id}
                      task={task}
                      onOpenSession={
                        task.sessionId
                          ? () => agentSpace.openTaskSession(task.id)
                          : undefined
                      }
                    />
                  ))}
                  {tasks.length === 0 ? (
                    <p className="px-2 py-5 text-center text-[11px] text-passive">
                      No tasks
                    </p>
                  ) : null}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <footer className="flex shrink-0 items-center justify-between border-t border-border-strong px-4 py-2 text-[11px] text-muted-foreground">
        <span>
          Audit {projection.audit.events} events ·{" "}
          {projection.usageTokens.toLocaleString()} tokens
        </span>
        <span title={projection.planDigest}>
          plan {projection.planDigest.slice(0, 8)}
        </span>
      </footer>
    </section>
  );
}

function GoalTaskCard({
  task,
  onOpenSession,
}: {
  task: GoalChainerTask;
  onOpenSession?: () => void;
}) {
  return (
    <button
      className={cn(
        "block w-full rounded-md border border-border bg-surface p-3 text-left",
        onOpenSession &&
          "cursor-pointer transition-colors hover:border-border-strong hover:bg-surface-hover",
      )}
      disabled={!onOpenSession}
      onClick={onOpenSession}
      type="button"
    >
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <h3 className="break-words text-xs font-medium">{task.id}</h3>
          <p className="mt-1 break-words text-[11px] text-muted-foreground">
            {task.acceptance}
          </p>
        </div>
        {task.stakes === "high" ? (
          <span className="rounded bg-warning/15 px-1.5 py-0.5 text-[10px] text-warning">
            high
          </span>
        ) : null}
      </div>
      {task.scheduledAt ? (
        <p className="mt-2 flex items-center gap-1 text-[11px] text-muted-foreground">
          <CalendarClock className="size-3" aria-hidden="true" />
          {new Date(task.scheduledAt).toLocaleString()}
        </p>
      ) : null}
      {task.releaseReason ? (
        <p className="mt-1 break-words text-[10px] text-passive">
          {task.releaseReason}
        </p>
      ) : null}
      {task.upstreamBlocked ? (
        <p className="mt-2 flex items-center gap-1 text-[11px] text-warning">
          <CircleAlert className="size-3" aria-hidden="true" />
          Waiting on dependencies
        </p>
      ) : null}
      {task.dependencies.length > 0 ? (
        <p className="mt-2 break-words text-[10px] text-passive">
          after {task.dependencies.join(", ")}
        </p>
      ) : null}
      <div className="mt-2 flex items-center justify-between gap-2 text-[10px] text-passive">
        <span>{task.assignedAgent || task.capability}</span>
        {task.sessionId ? (
          <span className="flex items-center gap-1 text-accent">
            <Link2 className="size-3" aria-hidden="true" />
            {task.sessionId}
          </span>
        ) : task.attempts > 0 ? (
          <span>{task.attempts} attempts</span>
        ) : null}
      </div>
    </button>
  );
}

function projectionFreshness(updatedAt: string | undefined): {
  label: string;
  title: string;
  stale: boolean;
} {
  if (!updatedAt) {
    return {
      label: "sync time unknown",
      title: "AO did not provide a projection update time",
      stale: true,
    };
  }
  const timestamp = Date.parse(updatedAt);
  if (!Number.isFinite(timestamp)) {
    return { label: "sync time invalid", title: updatedAt, stale: true };
  }
  const stale = Date.now() - timestamp > SOURCE_STALE_AFTER_MS;
  return {
    label: stale ? "source stale" : "source current",
    title: `Last GoalChainer sync: ${new Date(timestamp).toLocaleString()}`,
    stale,
  };
}
