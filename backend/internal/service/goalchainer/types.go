// Package goalchainer owns the read-only GoalChainer board projection.
package goalchainer

import (
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

const (
	// ProtocolV1 is the legacy task-only GoalChainer board contract.
	ProtocolV1 = "goalchainer-ui-v1"
	// ProtocolV2 adds stable project agents, attempt history, and logical
	// Atomspace links.
	ProtocolV2 = "goalchainer-ui-v2"
	// Protocol remains the v1 name for source compatibility with existing
	// callers and fixtures.
	Protocol = ProtocolV1
)

// Agent is one declared semantic project agent in a v2 projection.
type Agent struct {
	AgentKey domain.ProjectAgentKey  `json:"agentKey" minLength:"1" maxLength:"128"`
	Role     domain.ProjectAgentRole `json:"role" minLength:"1" maxLength:"128"`
	AgentID  domain.ProjectAgentID   `json:"agentId,omitempty" pattern:"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"`
}

// SessionAttempt keeps Goal task attempts separate from AO stable-agent
// attempts.
type SessionAttempt struct {
	Attempt      int                   `json:"attempt" minimum:"1"`
	AgentID      domain.ProjectAgentID `json:"agentId" pattern:"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"`
	AgentAttempt int64                 `json:"agentAttempt" minimum:"1"`
	SessionID    domain.SessionID      `json:"sessionId" minLength:"1" maxLength:"128"`
	Status       string                `json:"status" enum:"working,pr_open,draft,ci_failed,review_pending,changes_requested,approved,mergeable,merged,needs_input,exited,idle,terminated,no_signal"`
	Activity     string                `json:"activity" enum:"active,idle,waiting_input,blocked,exited"`
	IsTerminated bool                  `json:"isTerminated"`
}

// AtomspaceLink is a path-free logical memory relation.
type AtomspaceLink struct {
	LinkKey string `json:"linkKey" minLength:"1" maxLength:"128"`
	Scope   string `json:"scope" enum:"shared,private"`
	Owner   string `json:"owner" minLength:"1" maxLength:"128"`
}

// Task is one MeTTa-derived GoalChainer task shown on the project board.
type Task struct {
	ID              string                  `json:"id" minLength:"1" maxLength:"128"`
	Capability      string                  `json:"capability" enum:"local,specialist,either"`
	Acceptance      string                  `json:"acceptance" minLength:"1" maxLength:"128"`
	Stakes          string                  `json:"stakes" enum:"normal,high"`
	Dependencies    []string                `json:"dependencies"`
	State           string                  `json:"state" enum:"scheduled,backlog,ready,in-progress,blocked,done"`
	UpstreamBlocked bool                    `json:"upstreamBlocked"`
	AssignedAgent   string                  `json:"assignedAgent,omitempty" maxLength:"128"`
	AssignedRole    domain.ProjectAgentRole `json:"assignedRole,omitempty" maxLength:"128"`
	AgentKey        domain.ProjectAgentKey  `json:"agentKey,omitempty" maxLength:"128"`
	AgentID         domain.ProjectAgentID   `json:"agentId,omitempty" pattern:"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"`
	Attempts        int                     `json:"attempts" minimum:"0"`
	SessionID       domain.SessionID        `json:"sessionId,omitempty"`
	SessionAttempts []SessionAttempt        `json:"sessionAttempts,omitempty"`
	AtomspaceLinks  []AtomspaceLink         `json:"atomspaceLinks,omitempty"`
	ScheduledAt     *time.Time              `json:"scheduledAt,omitempty"`
	ReleaseReason   string                  `json:"releaseReason,omitempty" maxLength:"256"`
}

// Summary contains counts from the exact GoalChainer board snapshot.
type Summary struct {
	Total      int `json:"total" minimum:"0"`
	Scheduled  int `json:"scheduled" minimum:"0"`
	Backlog    int `json:"backlog" minimum:"0"`
	Ready      int `json:"ready" minimum:"0"`
	InProgress int `json:"inProgress" minimum:"0"`
	Blocked    int `json:"blocked" minimum:"0"`
	Done       int `json:"done" minimum:"0"`
	Progress   int `json:"progressPct" minimum:"0" maximum:"100"`
}

// Audit identifies the verified hash-chained GoalChainer event head.
type Audit struct {
	Events int    `json:"events" minimum:"0"`
	Head   string `json:"head,omitempty" pattern:"^[0-9a-f]{64}$"`
}

// Projection is a versioned, immutable-by-AO view of one GoalChainer thread.
type Projection struct {
	Protocol       string           `json:"protocol" enum:"goalchainer-ui-v1,goalchainer-ui-v2"`
	SourceRevision string           `json:"sourceRevision" pattern:"^[0-9a-f]{64}$"`
	ThreadID       string           `json:"threadId" minLength:"1" maxLength:"128"`
	Title          string           `json:"title" minLength:"1" maxLength:"256"`
	Owner          string           `json:"owner" minLength:"1" maxLength:"128"`
	SemanticDigest string           `json:"semanticDigest" pattern:"^[0-9a-f]{64}$"`
	PlanDigest     string           `json:"planDigest" pattern:"^[0-9a-f]{64}$"`
	UsageTokens    int              `json:"usageTokens" minimum:"0"`
	Summary        Summary          `json:"summary"`
	Agents         []Agent          `json:"agents,omitempty"`
	Tasks          []Task           `json:"tasks"`
	Audit          Audit            `json:"audit"`
	UpdatedAt      time.Time        `json:"updatedAt,omitempty"`
	ProjectID      domain.ProjectID `json:"projectId,omitempty"`
}

// DeleteResult reports whether a projection was connected before deletion.
type DeleteResult struct {
	OK      bool `json:"ok"`
	Deleted bool `json:"deleted"`
}
