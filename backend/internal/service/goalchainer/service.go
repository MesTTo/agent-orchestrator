package goalchainer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
)

const maxProjectionBytes = 1 << 20

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

var v2SessionStatuses = map[string]struct{}{
	"working": {}, "pr_open": {}, "draft": {}, "ci_failed": {},
	"review_pending": {}, "changes_requested": {}, "approved": {},
	"mergeable": {}, "merged": {}, "needs_input": {}, "exited": {},
	"idle": {}, "terminated": {}, "no_signal": {},
}

var v2ActivityStates = map[string]struct{}{
	"active": {}, "idle": {}, "waiting_input": {}, "blocked": {}, "exited": {},
}

// Store is the durable projection surface required by Service.
type Store interface {
	GoalChainerProjectExists(context.Context, domain.ProjectID) (bool, error)
	UpsertGoalChainerProjection(context.Context, domain.GoalChainerProjectionRecord) (domain.GoalChainerProjectionRecord, error)
	GetGoalChainerProjection(context.Context, domain.ProjectID) (domain.GoalChainerProjectionRecord, error)
	DeleteGoalChainerProjection(context.Context, domain.ProjectID) (bool, error)
}

// Manager is the controller-facing GoalChainer projection contract.
type Manager interface {
	Get(context.Context, domain.ProjectID) (Projection, error)
	Put(context.Context, domain.ProjectID, Projection) (Projection, error)
	Delete(context.Context, domain.ProjectID) (DeleteResult, error)
}

// Service validates and stores GoalChainer-owned board projections.
type Service struct {
	store Store
	clock func() time.Time
}

var _ Manager = (*Service)(nil)

// New constructs a GoalChainer projection service.
func New(store Store) *Service {
	return &Service{store: store, clock: time.Now}
}

// Get returns the connected projection for one active AO project.
func (s *Service) Get(ctx context.Context, projectID domain.ProjectID) (Projection, error) {
	if err := validateProjectID(projectID); err != nil {
		return Projection{}, err
	}
	record, err := s.store.GetGoalChainerProjection(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return Projection{}, apierr.NotFound("GOALCHAINER_NOT_CONNECTED", "GoalChainer is not connected to this project")
	}
	if err != nil {
		return Projection{}, apierr.Internal("GOALCHAINER_LOAD_FAILED", "Failed to load GoalChainer projection")
	}
	var projection Projection
	if err := json.Unmarshal([]byte(record.Projection), &projection); err != nil {
		return Projection{}, apierr.Internal("GOALCHAINER_PROJECTION_INVALID", "Stored GoalChainer projection is invalid")
	}
	projection.ProjectID = record.ProjectID
	projection.UpdatedAt = record.UpdatedAt
	return projection, nil
}

// Put replaces a project projection without interpreting task lifecycle.
func (s *Service) Put(
	ctx context.Context,
	projectID domain.ProjectID,
	projection Projection,
) (Projection, error) {
	if err := validateProjectID(projectID); err != nil {
		return Projection{}, err
	}
	if err := validateProjectionForProject(projectID, projection); err != nil {
		return Projection{}, err
	}
	exists, err := s.store.GoalChainerProjectExists(ctx, projectID)
	if err != nil {
		return Projection{}, apierr.Internal("PROJECT_LOAD_FAILED", "Failed to load project")
	}
	if !exists {
		return Projection{}, apierr.NotFound("PROJECT_NOT_FOUND", "Unknown project")
	}
	if projection.Protocol == ProtocolV1 {
		projection.ProjectID = ""
	}
	projection.UpdatedAt = time.Time{}
	body, err := json.Marshal(projection)
	if err != nil {
		return Projection{}, apierr.Internal("GOALCHAINER_ENCODE_FAILED", "Failed to encode GoalChainer projection")
	}
	if len(body) > maxProjectionBytes {
		return Projection{}, apierr.Invalid(
			"GOALCHAINER_PROJECTION_TOO_LARGE",
			"GoalChainer projection exceeds the 1 MiB limit",
			map[string]any{"limit": maxProjectionBytes},
		)
	}
	record, err := s.store.UpsertGoalChainerProjection(
		ctx,
		domain.GoalChainerProjectionRecord{
			ProjectID:      projectID,
			Protocol:       projection.Protocol,
			SourceRevision: projection.SourceRevision,
			Projection:     string(body),
			UpdatedAt:      s.clock().UTC(),
		},
	)
	if err != nil {
		return Projection{}, apierr.Internal("GOALCHAINER_SAVE_FAILED", "Failed to save GoalChainer projection")
	}
	projection.ProjectID = record.ProjectID
	projection.UpdatedAt = record.UpdatedAt
	return projection, nil
}

// Delete disconnects the UI projection. It cannot change GoalChainer state.
func (s *Service) Delete(ctx context.Context, projectID domain.ProjectID) (DeleteResult, error) {
	if err := validateProjectID(projectID); err != nil {
		return DeleteResult{}, err
	}
	deleted, err := s.store.DeleteGoalChainerProjection(ctx, projectID)
	if err != nil {
		return DeleteResult{}, apierr.Internal("GOALCHAINER_DELETE_FAILED", "Failed to disconnect GoalChainer")
	}
	return DeleteResult{OK: true, Deleted: deleted}, nil
}

func validateProjectID(projectID domain.ProjectID) error {
	if !safeID.MatchString(string(projectID)) {
		return apierr.Invalid("INVALID_PROJECT_ID", "Project id is invalid", nil)
	}
	return nil
}

func validateProjection(projection Projection) error {
	if projection.Protocol != ProtocolV1 && projection.Protocol != ProtocolV2 {
		return invalidProjection("protocol must be goalchainer-ui-v1 or goalchainer-ui-v2")
	}
	for name, digest := range map[string]string{
		"sourceRevision": projection.SourceRevision,
		"semanticDigest": projection.SemanticDigest,
		"planDigest":     projection.PlanDigest,
	} {
		if !validDigest(digest) {
			return invalidProjection(name + " must be a lowercase SHA-256 digest")
		}
	}
	if !safeID.MatchString(projection.ThreadID) || !safeID.MatchString(projection.Owner) {
		return invalidProjection("threadId and owner must be safe identifiers")
	}
	if strings.TrimSpace(projection.Title) == "" || len(projection.Title) > 256 {
		return invalidProjection("title must contain 1 to 256 characters")
	}
	if projection.UsageTokens < 0 || projection.Audit.Events < 0 {
		return invalidProjection("usage and audit counts must be nonnegative")
	}
	if projection.Audit.Head != "" && !validDigest(projection.Audit.Head) {
		return invalidProjection("audit head must be a lowercase SHA-256 digest")
	}
	if projection.Audit.Events == 0 && projection.Audit.Head != "" {
		return invalidProjection("an empty audit cannot have a head")
	}
	if projection.Audit.Events > 0 && projection.Audit.Head == "" {
		return invalidProjection("a nonempty audit requires a head")
	}
	seen := make(map[string]struct{}, len(projection.Tasks))
	counts := map[string]int{}
	for _, task := range projection.Tasks {
		if err := validateTask(task); err != nil {
			return err
		}
		if _, duplicate := seen[task.ID]; duplicate {
			return invalidProjection("task IDs must be unique")
		}
		seen[task.ID] = struct{}{}
		counts[task.State]++
	}
	for _, task := range projection.Tasks {
		for _, dependency := range task.Dependencies {
			if _, present := seen[dependency]; !present {
				return invalidProjection("task dependencies must reference existing tasks")
			}
			if dependency == task.ID {
				return invalidProjection("a task cannot depend on itself")
			}
		}
	}
	summary := projection.Summary
	if summary.Total != len(projection.Tasks) ||
		summary.Scheduled != counts["scheduled"] ||
		summary.Backlog != counts["backlog"] ||
		summary.Ready != counts["ready"] ||
		summary.InProgress != counts["in-progress"] ||
		summary.Blocked != counts["blocked"] ||
		summary.Done != counts["done"] {
		return invalidProjection("summary counts must match task states")
	}
	if summary.Progress < 0 || summary.Progress > 100 {
		return invalidProjection("progressPct must be between 0 and 100")
	}
	expectedProgress := 0
	if summary.Total > 0 {
		expectedProgress = summary.Done * 100 / summary.Total
	}
	if summary.Progress != expectedProgress {
		return invalidProjection("progressPct must match the done task count")
	}
	if projection.Protocol == ProtocolV1 {
		return validateV1Projection(projection)
	}
	if err := validateV2Projection(projection); err != nil {
		return err
	}
	return nil
}

func validateProjectionForProject(
	projectID domain.ProjectID,
	projection Projection,
) error {
	if err := validateProjection(projection); err != nil {
		return err
	}
	if projection.Protocol == ProtocolV2 && projection.ProjectID != projectID {
		return invalidProjection("v2 projectId must match the route project")
	}
	return nil
}

func validateV1Projection(projection Projection) error {
	if len(projection.Agents) != 0 {
		return invalidProjection("v1 projections cannot contain agents")
	}
	for _, task := range projection.Tasks {
		if task.AssignedRole != "" || task.AgentKey != "" || task.AgentID != "" ||
			len(task.SessionAttempts) != 0 || len(task.AtomspaceLinks) != 0 {
			return invalidProjection("v1 tasks cannot contain v2 project-agent fields")
		}
	}
	return nil
}

func validateV2Projection(projection Projection) error {
	if !safeID.MatchString(string(projection.ProjectID)) {
		return invalidProjection("v2 projectId must be a safe identifier")
	}
	if len(projection.Agents) == 0 {
		return invalidProjection("v2 projections require declared agents")
	}
	agents := make(map[domain.ProjectAgentKey]Agent, len(projection.Agents))
	roles := make(map[domain.ProjectAgentRole]struct{}, len(projection.Agents))
	agentIDs := make(map[domain.ProjectAgentID]struct{}, len(projection.Agents))
	for _, agent := range projection.Agents {
		if !domain.ValidProjectAgentKey(agent.AgentKey) ||
			!domain.ValidProjectAgentRole(agent.Role) {
			return invalidProjection("v2 agent keys and roles must be safe identifiers")
		}
		if _, duplicate := agents[agent.AgentKey]; duplicate {
			return invalidProjection("v2 agent keys must be unique")
		}
		if _, duplicate := roles[agent.Role]; duplicate {
			return invalidProjection("v2 agent roles must be unique")
		}
		if agent.AgentID != "" {
			if !domain.ValidProjectAgentID(agent.AgentID) {
				return invalidProjection("v2 agentId must be a canonical UUID")
			}
			if _, duplicate := agentIDs[agent.AgentID]; duplicate {
				return invalidProjection("v2 agent IDs must be unique")
			}
			agentIDs[agent.AgentID] = struct{}{}
		}
		agents[agent.AgentKey] = agent
		roles[agent.Role] = struct{}{}
	}

	sessionIDs := make(map[domain.SessionID]struct{})
	agentAttempts := make(map[string]struct{})
	for _, task := range projection.Tasks {
		if task.AgentKey == "" {
			if task.Capability != "either" ||
				task.AssignedRole != "" ||
				task.AssignedAgent != "" ||
				task.AgentID != "" ||
				task.Attempts != 0 ||
				task.SessionID != "" ||
				len(task.SessionAttempts) != 0 {
				return invalidProjection("only undispatched either tasks may be unassigned in v2")
			}
			if err := validateV2AtomspaceLinks(task, agents, true); err != nil {
				return err
			}
			continue
		}
		agent, ok := agents[task.AgentKey]
		if !ok {
			return invalidProjection("v2 task agentKey must reference a declared agent")
		}
		if task.AssignedRole == "" || task.AssignedRole != agent.Role {
			return invalidProjection("v2 task assignedRole must match its declared agent")
		}
		if task.AssignedAgent != string(task.AgentKey) {
			return invalidProjection("v2 task assignedAgent must equal agentKey")
		}
		if task.AgentID != agent.AgentID {
			return invalidProjection("v2 task agentId must match its declared agent")
		}
		if task.Attempts != len(task.SessionAttempts) {
			return invalidProjection("v2 task attempts must match sessionAttempts")
		}
		if err := validateV2TaskAttempts(
			task,
			sessionIDs,
			agentAttempts,
			agentIDs,
		); err != nil {
			return err
		}
		if err := validateV2AtomspaceLinks(task, agents, false); err != nil {
			return err
		}
	}
	return nil
}

func validateV2TaskAttempts(
	task Task,
	sessionIDs map[domain.SessionID]struct{},
	agentAttempts map[string]struct{},
	declaredAgentIDs map[domain.ProjectAgentID]struct{},
) error {
	goalAttempts := make(map[int]struct{}, len(task.SessionAttempts))
	var latest SessionAttempt
	for _, attempt := range task.SessionAttempts {
		if attempt.Attempt <= 0 || attempt.AgentAttempt <= 0 {
			return invalidProjection("v2 Goal and AO attempt numbers must be positive")
		}
		if !domain.ValidProjectAgentID(attempt.AgentID) {
			return invalidProjection("v2 session attempt agentId must be a canonical UUID")
		}
		if _, declared := declaredAgentIDs[attempt.AgentID]; !declared {
			return invalidProjection("v2 session attempt agentId must reference a declared bound agent")
		}
		if !safeID.MatchString(string(attempt.SessionID)) {
			return invalidProjection("v2 session attempt sessionId must be a safe identifier")
		}
		if _, ok := v2SessionStatuses[attempt.Status]; !ok {
			return invalidProjection("v2 session attempt status is invalid")
		}
		if _, ok := v2ActivityStates[attempt.Activity]; !ok {
			return invalidProjection("v2 session attempt activity is invalid")
		}
		if _, duplicate := goalAttempts[attempt.Attempt]; duplicate {
			return invalidProjection("v2 Goal attempt numbers must be unique per task")
		}
		if _, duplicate := sessionIDs[attempt.SessionID]; duplicate {
			return invalidProjection("v2 session IDs must be unique")
		}
		agentAttemptKey := string(attempt.AgentID) + "\x00" +
			strconv.FormatInt(attempt.AgentAttempt, 10)
		if _, duplicate := agentAttempts[agentAttemptKey]; duplicate {
			return invalidProjection("v2 AO attempt numbers must be unique per agent")
		}
		goalAttempts[attempt.Attempt] = struct{}{}
		sessionIDs[attempt.SessionID] = struct{}{}
		agentAttempts[agentAttemptKey] = struct{}{}
		if latest.SessionID == "" || attempt.Attempt > latest.Attempt {
			latest = attempt
		}
	}
	if len(task.SessionAttempts) == 0 {
		if task.SessionID != "" {
			return invalidProjection("v2 tasks without attempts cannot name a session")
		}
		return nil
	}
	for expected := 1; expected <= len(task.SessionAttempts); expected++ {
		if _, present := goalAttempts[expected]; !present {
			return invalidProjection("v2 Goal attempts must be contiguous from 1")
		}
	}
	if task.SessionID != latest.SessionID {
		return invalidProjection("v2 task sessionId must name its latest Goal attempt")
	}
	if task.AgentID == "" || task.AgentID != latest.AgentID {
		return invalidProjection("v2 task agentId must match its latest Goal attempt")
	}
	return nil
}

func validateV2AtomspaceLinks(
	task Task,
	agents map[domain.ProjectAgentKey]Agent,
	unassigned bool,
) error {
	if len(task.AtomspaceLinks) == 0 {
		return invalidProjection("v2 tasks require logical Atomspace links")
	}
	linkKeys := make(map[string]struct{}, len(task.AtomspaceLinks))
	hasShared := false
	hasCurrentPrivate := false
	for _, link := range task.AtomspaceLinks {
		if !safeID.MatchString(link.LinkKey) {
			return invalidProjection("v2 Atomspace link keys must be safe identifiers")
		}
		if _, duplicate := linkKeys[link.LinkKey]; duplicate {
			return invalidProjection("v2 Atomspace link keys must be unique per task")
		}
		linkKeys[link.LinkKey] = struct{}{}
		switch link.Scope {
		case "shared":
			if link.Owner != "project" {
				return invalidProjection("v2 shared Atomspace links must be owned by project")
			}
			hasShared = true
		case "private":
			if unassigned {
				return invalidProjection("v2 unassigned tasks cannot contain private Atomspace links")
			}
			if _, ok := agents[domain.ProjectAgentKey(link.Owner)]; !ok {
				return invalidProjection("v2 private Atomspace links must reference a declared agent")
			}
			if link.Owner == string(task.AgentKey) {
				hasCurrentPrivate = true
			}
		default:
			return invalidProjection("v2 Atomspace link scope must be shared or private")
		}
	}
	if !hasShared {
		return invalidProjection("v2 tasks require a shared project Atomspace link")
	}
	if !unassigned && !hasCurrentPrivate {
		return invalidProjection("v2 assigned tasks require a private link for the current agent")
	}
	return nil
}

func validateTask(task Task) error {
	if !safeID.MatchString(task.ID) || !safeID.MatchString(task.Acceptance) {
		return invalidProjection("task id and acceptance must be safe identifiers")
	}
	switch task.Capability {
	case "local", "specialist", "either":
	default:
		return invalidProjection("task capability is invalid")
	}
	switch task.Stakes {
	case "normal", "high":
	default:
		return invalidProjection("task stakes is invalid")
	}
	switch task.State {
	case "scheduled", "backlog", "ready", "in-progress", "blocked", "done":
	default:
		return invalidProjection("task state is invalid")
	}
	if task.Attempts < 0 {
		return invalidProjection("task attempts must be nonnegative")
	}
	if task.AssignedAgent != "" && !safeID.MatchString(task.AssignedAgent) {
		return invalidProjection("assignedAgent is invalid")
	}
	if task.State == "scheduled" {
		if task.ScheduledAt == nil {
			return invalidProjection("scheduled tasks require scheduledAt")
		}
		if strings.TrimSpace(task.ReleaseReason) == "" {
			return invalidProjection("scheduled tasks require releaseReason")
		}
	} else if task.ScheduledAt != nil || task.ReleaseReason != "" {
		return invalidProjection(
			"only scheduled tasks may have scheduledAt or releaseReason",
		)
	}
	if task.ScheduledAt != nil && task.ScheduledAt.Location() != time.UTC {
		return invalidProjection("scheduledAt must use UTC")
	}
	if len(task.ReleaseReason) > 256 {
		return invalidProjection("releaseReason exceeds 256 characters")
	}
	if task.SessionID != "" && !safeID.MatchString(string(task.SessionID)) {
		return invalidProjection("sessionId is invalid")
	}
	dependencies := map[string]struct{}{}
	for _, dependency := range task.Dependencies {
		if !safeID.MatchString(dependency) {
			return invalidProjection("task dependency is invalid")
		}
		if _, duplicate := dependencies[dependency]; duplicate {
			return invalidProjection("task dependencies must be unique")
		}
		dependencies[dependency] = struct{}{}
	}
	return nil
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func invalidProjection(message string) error {
	return apierr.Invalid("INVALID_GOALCHAINER_PROJECTION", message, nil)
}
