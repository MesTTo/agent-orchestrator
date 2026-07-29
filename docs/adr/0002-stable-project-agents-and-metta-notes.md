# 2. Stable project agents and confined MeTTa notes

Date: 2026-07-29
Status: Accepted

## Context

GoalChainer schedules logical agents across more than one AO session. A session
ID identifies one runtime attempt, so it cannot identify the agent across
retries or preserve that agent's private MeTTa notes. GoalChainer also needs a
project-shared Atomspace, full attempt history, and a UI that can trace tasks,
agents, and sessions without giving callers control of filesystem paths.

## Decision

AO owns a stable project-agent UUID. A caller supplies a project-scoped
`projectAgentKey` and immutable `projectAgentRole` during session spawn. AO
resolves the pair, generates the UUID, and transactionally links each new
session as the next attempt. External source replay must resolve to the same
stable agent.

The identity and attempt links live in SQLite. They remain readable after
project archive. Archived projects still reject new sessions.

AO creates two confined MeTTa files:

- one shared file keyed by the SHA-256 project identity;
- one private file keyed by the AO-generated stable UUID.

API callers cannot submit or read these paths. Snapshot reads use a closed,
line-oriented schema and never evaluate MeTTa. Empty files are valid
uninitialized snapshots. Validation is limited to 16 MiB and API output is
limited to 256 KiB on complete lines.

GoalChainer projection v2 carries a path-free project-agent registry, separate
Goal and AO attempts, complete session history, and logical Atomspace links.
Projection v1 remains accepted with its prior stored JSON shape.

## Consequences

One stable agent can span many AO sessions while retaining one private notes
file. Goal task retries can change agents without rewriting historical
attempts. The UI can navigate from a task to its stable agent and from an agent
attempt to its session.

Migration `0040_project_agents.sql` adds the identity and attempt tables.
Migration `0041_goalchainer_projection_v2.sql` rebuilds the existing protocol
check for v1 and v2. A downgrade to v1 refuses to copy connected v2 rows, so an
operator must disconnect v2 projections first.

Dropping the tables does not remove Atomspace files. File retention or deletion
requires a separate explicit operator decision.
