package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxProjectAgentKeyBytes bounds caller-owned semantic agent keys.
	MaxProjectAgentKeyBytes = 128
	// MaxProjectAgentRoleBytes bounds MeTTa-selected role identifiers.
	MaxProjectAgentRoleBytes = 128
)

var (
	projectAgentKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	projectAgentRolePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

	// ErrExternalProjectAgentMismatch reports an idempotency replay that names
	// a different stable project agent from the original session attempt.
	ErrExternalProjectAgentMismatch = errors.New("external source project agent differs from the existing session")
	// ErrProjectAgentRoleMismatch reports reuse of a semantic agent key with a
	// role different from its immutable initial assignment.
	ErrProjectAgentRoleMismatch = errors.New("project agent role differs from the existing assignment")
)

// ProjectAgentID is the AO-generated stable UUID for one project-owned agent.
type ProjectAgentID string

// ProjectAgentKey is the caller-owned semantic key used to resolve an agent
// inside one project. It is never used as the AO identity or in a file path.
type ProjectAgentKey string

// ProjectAgentRole is the immutable MeTTa-selected role for a stable agent.
type ProjectAgentRole string

// ValidProjectAgentID reports whether value is a canonical UUID.
func ValidProjectAgentID(value ProjectAgentID) bool {
	parsed, err := uuid.Parse(string(value))
	return err == nil && parsed.String() == strings.ToLower(string(value))
}

// ValidProjectAgentKey reports whether value is a bounded safe identifier.
func ValidProjectAgentKey(value ProjectAgentKey) bool {
	return projectAgentKeyPattern.MatchString(string(value))
}

// ValidProjectAgentRole reports whether value is a bounded safe identifier.
func ValidProjectAgentRole(value ProjectAgentRole) bool {
	return projectAgentRolePattern.MatchString(string(value))
}

// ProjectAgentRecord is the durable stable identity resolved from a project and
// caller-owned semantic key.
type ProjectAgentRecord struct {
	ID                ProjectAgentID
	ProjectID         ProjectID
	ProjectAgentKey   ProjectAgentKey
	Role              ProjectAgentRole
	NotesRelativePath string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProjectAgentAttemptRecord immutably links one stable agent attempt to one AO
// session. Attempt numbers are scoped to the project agent.
type ProjectAgentAttemptRecord struct {
	ProjectID ProjectID
	AgentID   ProjectAgentID
	Attempt   int64
	SessionID SessionID
	CreatedAt time.Time
}

// ExternalProjectAgentMismatchError carries both stable identities for a typed
// idempotency conflict without exposing caller-owned keys.
type ExternalProjectAgentMismatchError struct {
	SessionID        SessionID
	ExistingAgentID  ProjectAgentID
	RequestedAgentID ProjectAgentID
}

func (e *ExternalProjectAgentMismatchError) Error() string {
	return ErrExternalProjectAgentMismatch.Error()
}

// Unwrap supports errors.Is with ErrExternalProjectAgentMismatch.
func (e *ExternalProjectAgentMismatchError) Unwrap() error {
	return ErrExternalProjectAgentMismatch
}

// ProjectAgentRoleMismatchError describes an immutable role conflict.
type ProjectAgentRoleMismatchError struct {
	AgentID       ProjectAgentID
	ExistingRole  ProjectAgentRole
	RequestedRole ProjectAgentRole
}

func (e *ProjectAgentRoleMismatchError) Error() string {
	return ErrProjectAgentRoleMismatch.Error()
}

// Unwrap supports errors.Is with ErrProjectAgentRoleMismatch.
func (e *ProjectAgentRoleMismatchError) Unwrap() error {
	return ErrProjectAgentRoleMismatch
}
