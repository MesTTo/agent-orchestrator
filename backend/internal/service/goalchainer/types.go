// Package goalchainer owns the read-only GoalChainer board projection.
package goalchainer

import (
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

// Protocol is the versioned wire contract accepted from GoalChainer.
const Protocol = "goalchainer-ui-v1"

// Task is one MeTTa-derived GoalChainer task shown on the project board.
type Task struct {
	ID              string           `json:"id" minLength:"1" maxLength:"128"`
	Capability      string           `json:"capability" enum:"local,specialist,either"`
	Acceptance      string           `json:"acceptance" minLength:"1" maxLength:"128"`
	Stakes          string           `json:"stakes" enum:"normal,high"`
	Dependencies    []string         `json:"dependencies"`
	State           string           `json:"state" enum:"scheduled,backlog,ready,in-progress,blocked,done"`
	UpstreamBlocked bool             `json:"upstreamBlocked"`
	AssignedAgent   string           `json:"assignedAgent,omitempty" maxLength:"128"`
	Attempts        int              `json:"attempts" minimum:"0"`
	SessionID       domain.SessionID `json:"sessionId,omitempty"`
	ScheduledAt     *time.Time       `json:"scheduledAt,omitempty"`
	ReleaseReason   string           `json:"releaseReason,omitempty" maxLength:"256"`
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
	Protocol       string           `json:"protocol" enum:"goalchainer-ui-v1"`
	SourceRevision string           `json:"sourceRevision" pattern:"^[0-9a-f]{64}$"`
	ThreadID       string           `json:"threadId" minLength:"1" maxLength:"128"`
	Title          string           `json:"title" minLength:"1" maxLength:"256"`
	Owner          string           `json:"owner" minLength:"1" maxLength:"128"`
	SemanticDigest string           `json:"semanticDigest" pattern:"^[0-9a-f]{64}$"`
	PlanDigest     string           `json:"planDigest" pattern:"^[0-9a-f]{64}$"`
	UsageTokens    int              `json:"usageTokens" minimum:"0"`
	Summary        Summary          `json:"summary"`
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
