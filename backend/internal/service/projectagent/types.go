// Package projectagent exposes stable project-owned agents and their immutable
// AO session attempts.
package projectagent

import (
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

// NotesMetadata is the bounded file metadata exposed without its internal path.
type NotesMetadata struct {
	Exists      bool      `json:"exists"`
	Initialized bool      `json:"initialized"`
	SizeBytes   int64     `json:"sizeBytes" minimum:"0"`
	ModifiedAt  time.Time `json:"modifiedAt,omitempty"`
}

// Attempt is one immutable stable-agent to AO-session link.
type Attempt struct {
	Number    int64            `json:"number" minimum:"1"`
	SessionID domain.SessionID `json:"sessionId"`
	CreatedAt time.Time        `json:"createdAt"`
}

// Agent is the API read model for one AO-generated stable identity.
type Agent struct {
	ID              domain.ProjectAgentID   `json:"id" pattern:"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"`
	ProjectID       domain.ProjectID        `json:"projectId"`
	ProjectAgentKey domain.ProjectAgentKey  `json:"projectAgentKey"`
	Role            domain.ProjectAgentRole `json:"role"`
	Notes           NotesMetadata           `json:"notes"`
	Attempts        []Attempt               `json:"attempts"`
	CreatedAt       time.Time               `json:"createdAt"`
	UpdatedAt       time.Time               `json:"updatedAt"`
}

// Notes is the closed, validated, bounded MeTTa projection for one private
// stable-agent Atomspace.
type Notes struct {
	AgentID          domain.ProjectAgentID `json:"agentId" pattern:"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"`
	Content          string                `json:"content"`
	Truncated        bool                  `json:"truncated"`
	Initialized      bool                  `json:"initialized"`
	PropositionCount int                   `json:"propositionCount" minimum:"0"`
	SizeBytes        int64                 `json:"sizeBytes" minimum:"0"`
	ModifiedAt       time.Time             `json:"modifiedAt"`
}
