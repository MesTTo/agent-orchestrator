# GoalChainer board

Agent Orchestrator can display a GoalChainer task graph beside its coding-agent
sessions. The board has scheduled, backlog, ready, in-progress, blocked, and
done lanes. A linked task opens the existing session terminal.

The board is a read-only projection. It does not claim, complete, block, or
release GoalChainer tasks.

## Authority boundary

Mettabase MeTTa Markdown owns the human-readable specification. GoalChainer
MeTTaScript rewrites task, dependency, schedule, assignment, and observation
atoms into route, release, outcome, and projection atoms. OmegaClaw owns the
durable task lifecycle. Agent Orchestrator owns coding-agent sessions,
worktrees, terminal processes, and transcripts.

Agent status is observational evidence. An exited session does not complete an
Omega task. GoalChainer still requires its evaluator path.

## API

The local daemon exposes:

```text
GET    /api/v1/projects/{projectId}/goalchainer
PUT    /api/v1/projects/{projectId}/goalchainer
DELETE /api/v1/projects/{projectId}/goalchainer
```

PUT accepts one complete `goalchainer-ui-v1` snapshot and atomically replaces
the prior project record. The body limit is 1 MiB. Validation rejects unknown
protocols, invalid state combinations, invalid timestamps, inconsistent
summary counts, unsafe identities, and non-finite values.

The session creation API accepts `sourceTaskId`. Agent Orchestrator serializes
requests for the same project and source ID, stores the mapping, and returns
the existing active session on retry. Different source IDs can launch in
parallel.

The same API accepts the typed `mcpProfile` value `oh-my-goals`. GoalChainer
derives that profile from its MeTTa specification. API callers cannot submit
an executable path or raw MCP arguments.

The session prompt begins with the MeTTaScript-compiled workflow. Ordered
workflow steps tell the worker how to inspect context, perform the released
task, verify acceptance, and report evidence. GoalChainer then appends the
task contract and states that only its evaluator may complete the task.

SQLite migration `0037_goalchainer_projection.sql` adds the projection record
and CDC trigger. Migration `0038_external_session_sources.sql` adds the source
task mapping. Migration `0039_session_mcp_profile.sql` retains the typed
profile across restart and resume. The existing CDC stream emits
`goalchainer_projection_changed`, and the renderer refetches the projection.

## Oh My Goals tools

Set the built Oh My Goals CLI in the Agent Orchestrator project environment:

```json
{
  "env": {
    "OH_MY_GOALS_CLI": "/absolute/path/to/oh-my-goals/dist/cli.js"
  }
}
```

Use `PUT /api/v1/projects/{projectId}/config` or the project settings screen to
store the environment. A CLI installed on the daemon's `PATH` can be used
without this setting.

For each selected session, Agent Orchestrator runs the existing Oh My Goals
`install-skill` command in the worktree and adds one session-scoped stdio MCP
server. Claude Code receives an additive `--mcp-config`. Codex receives
additive `mcp_servers` configuration values. OpenCode receives an AO-owned
configuration file. Existing user-level MCP servers remain available.

The runtime sets:

```text
OH_MY_GOALS_MEMORY_DB
OH_MY_GOALS_REPOSITORY
OH_MY_GOALS_SESSION
```

Sessions in one Agent Orchestrator project share a project memory database.
The repository and session values preserve source identity inside each memory
record. Unsupported harnesses and missing CLIs fail before a durable session
or worktree is created.

## Board behavior

Each card shows its acceptance condition, schedule, release reason,
dependencies, assigned role, attempt count, and linked session. The header
shows the source revision and last update state. A projection older than two
minutes is labeled `source stale`.

The renderer converts the JSON snapshot to MeTTa atoms:

```metta
(gc-projection-record ...)
(gc-task-record ...)
(gc-task-session collect-sources scratch-1)
(gc-agent-record scratch-1)
(gc-agent-task scratch-1 collect-sources)
```

Queries use `match &self` over those atoms. The grounded
`ao-open-session` atom is the narrow UI effect that navigates to the session
screen.

## GoalChainer operations

Run these commands in the GoalChainer checkout while the local daemon is
listening:

```bash
goalchainer thread ui-sync \
  --spec thread.md --plan thread.metta --audit thread.jsonl \
  --project my-project --ao-url http://127.0.0.1:3001 --json

goalchainer thread dispatch-due \
  --spec thread.md --plan thread.metta --audit thread.jsonl \
  --project my-project --ao-url http://127.0.0.1:3001 --json

goalchainer thread ui-watch \
  --spec thread.md --plan thread.metta --audit thread.jsonl \
  --project my-project --ao-url http://127.0.0.1:3001 \
  --interval-seconds 5 --json
```

`ui-observe` performs one audit-only session observation. `disconnect` deletes
the stored board projection. Deleting the projection does not delete sessions
or change GoalChainer lifecycle state.

## Privacy and rollback

This fork disables PostHog transmission in the renderer and daemon. Local
SQLite telemetry may remain enabled. GoalChainer synchronization accepts only
an explicit loopback HTTP origin.

To roll back the UI integration, stop GoalChainer synchronization and call the
DELETE route for each project. Existing sessions remain available. Remove the
`thread-mcp-profile` fact to use the checked default, or stop dispatching MCP
profiles before reverting migration `0039`. Reverting migrations `0038` and
`0037` removes the source mappings and cached projection after the
corresponding code has been rolled back.
