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

PUT accepts one complete `goalchainer-ui-v1` or `goalchainer-ui-v2` snapshot
and atomically replaces the prior project record. The body limit is 1 MiB.
Validation rejects unknown protocols, invalid state combinations, invalid
timestamps, inconsistent summary counts, unsafe identities, and non-finite
values.

The v2 projection adds a top-level project ID and agent registry. Each task can
name its assigned role, semantic agent key, AO-generated stable UUID, complete
session attempt history, and logical Atomspace links. AO checks that:

- the projection project ID matches the route;
- agent keys, roles, and bound UUIDs are unique;
- task assignments refer to declared agents;
- Goal attempts are contiguous from 1;
- AO agent attempts are positive and unique per stable agent;
- the latest Goal attempt matches the task's current session and agent;
- logical links contain no filesystem paths;
- every task has a shared project link;
- assigned tasks have a private link owned by the current agent.

An `either` task may remain unassigned before its first dispatch. In that state
it has no attempts or private link. Historical attempts may name an earlier
stable agent when GoalChainer rerouted the task.

The session creation API accepts `sourceTaskId`, `projectAgentKey`, and
`projectAgentRole`. The key and role must be supplied together. AO resolves
the semantic key inside the project, generates the stable UUID, and keeps the
initial role immutable. Callers cannot submit a raw stable UUID or notes path.
Agent Orchestrator serializes requests for the same project and source ID,
stores the mapping, and returns the existing active session on retry.

The same API accepts the typed `mcpProfile` value `oh-my-goals`. GoalChainer
derives that profile from its MeTTa specification. API callers cannot submit
an executable path or raw MCP arguments.

The session prompt begins with the MeTTaScript-compiled workflow. Ordered
workflow steps tell the worker how to inspect context, perform the released
task, verify acceptance, and report evidence. GoalChainer then appends the
task contract and states that only its evaluator may complete the task.

The stable-agent read API is:

```text
GET /api/v1/projects/{projectId}/agents
GET /api/v1/projects/{projectId}/agents/{agentUuid}
GET /api/v1/projects/{projectId}/agents/{agentUuid}/notes
```

These routes are read-only. They never expose an internal path.

SQLite migration `0037_goalchainer_projection.sql` adds the projection record
and CDC trigger. Migration `0038_external_session_sources.sql` adds the source
task mapping. Migration `0039_session_mcp_profile.sql` retains the typed
profile across restart and resume. Migration
`0040_project_agents.sql` adds stable identities and immutable session attempt
links. Migration `0041_goalchainer_projection_v2.sql` rebuilds the projection
table check so v1 and v2 can coexist. The existing CDC stream emits
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
OH_MY_GOALS_AGENT_ID
OH_MY_GOALS_ATTEMPT
OH_MY_GOALS_ATOMSPACE_PATH
OH_MY_GOALS_PROJECT_ATOMSPACE_PATH
```

Sessions in one Agent Orchestrator project share a project memory database.
`OH_MY_GOALS_SESSION` is the actual AO session ID.
`OH_MY_GOALS_AGENT_ID` is the stable AO-generated project-agent UUID.
`OH_MY_GOALS_ATTEMPT` is that stable agent's AO attempt number. The private
Atomspace path remains stable across resume. Unsupported harnesses and missing
CLIs fail before a durable session or worktree is created.

## Stable MeTTa notes

AO generates both Atomspace files beneath `AO_DATA_DIR`. The internal shape is:

```text
mcp/oh-my-goals/projects/{sha256-project}/atomspaces/shared.metta
mcp/oh-my-goals/projects/{sha256-project}/atomspaces/agents/{agent-uuid}/notes.metta
```

The paths are generated from the project and AO UUID. They are never accepted
from an API caller or returned by a read route. Directories use mode `0700`
and files use `0600`. Confined file operations reject symlinks and identity
path mismatches.

A zero-byte AO-created file is an uninitialized snapshot with zero
propositions. A nonempty private snapshot starts with:

```metta
(MemorySnapshot oh-my-goals-atomspace-v1 agent "PROJECT" "AGENT-UUID")
```

A nonempty shared snapshot starts with:

```metta
(MemorySnapshot oh-my-goals-atomspace-v1 project "PROJECT" "")
```

The parser accepts only sorted canonical one-line facts with these heads:
`MemoryProposition`, `MemoryContent`, `MemoryState`, `MemoryRecordedAt`,
`MemoryTree`, `MemorySource`, `MemoryAssertionState`, `MemorySourceTruth`,
`MemoryDerivation`, `MemorySupersedes`, and `ClaimCore`. Private propositions
use `session` scope. Shared propositions use `project` or `derived` scope. AO
does not evaluate snapshot text.

Validation reads at most 16 MiB. The notes API returns at most 256 KiB and
stops on a complete line. The expanded UI view polls every 15 seconds so
external Oh My Goals writes appear while the card stays open.

## Board behavior

Each card shows its acceptance condition, schedule, release reason,
dependencies, assigned role, semantic agent key, stable UUID, logical
Atomspace labels, separate Goal and AO attempts, and every linked session. The
stable UUID links to the matching project-agent row. Each project-agent
attempt links back to its session. The header shows the source revision and
last update state. A projection older than two minutes is labeled
`source stale`.

The renderer converts the JSON snapshot to MeTTa atoms:

```metta
(gc-projection-record ...)
(gc-task-record ...)
(gc-task-session collect-sources scratch-1)
(gc-agent-record scratch-1)
(gc-agent-task scratch-1 collect-sources)
```

Queries use `match &self` over those atoms. The grounded
`ao-open-session` atom is the narrow UI effect that navigates to the latest
session or one selected historical attempt.

Archived projects keep their stable-agent identities, attempts, and notes
readable by project ID. New spawns remain blocked for archived projects.

## Checked demo

These checks exercise the behavior shown in the UI:

```bash
cd backend
go test ./internal/session_manager \
  -run 'TestSpawnPreparesOhMyGoalsMCPProfile|TestSpawnReusesStableProjectAgentAndAppendsAttempts'
go test ./internal/agentnotes \
  -run TestPrivateNotesStayIsolatedAcrossProjectsAndStableAgents
go test ./internal/httpd/controllers \
  -run TestGoalChainerV2SyncFixtureRoundTripsStableAgentData

cd ../frontend
npm test -- --run \
  src/renderer/components/GoalChainerBoard.test.tsx \
  src/renderer/components/ProjectAgentsPanel.test.tsx
```

The first check spawns and restores the same stable agent, verifies the AO
session ID, stable UUID, AO attempt, and private/shared paths, then writes a
valid private note and checks that termination plus a second attempt preserves
the exact snapshot. The second writes different notes for two agents and two
projects, then reads each snapshot and checks that content did not cross either
boundary. The controller fixture is a complete v2 sync with two different
tasks, each at Goal attempt 1, mapped to AO agent attempts 1 and 2.

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
profiles before reverting migration `0039`.

Migration `0041` can downgrade only after every v2 projection has been
disconnected. Its v1 table check makes an attempted downgrade with v2 data
fail instead of dropping that data. Reverting migration `0040` removes the
stable identity and attempt tables. It does not delete Atomspace files under
`AO_DATA_DIR`; remove those only with an explicit backup and file-retention
decision. Reverting migrations `0038` and `0037` removes the source mappings
and cached projection after the corresponding code has been rolled back.
