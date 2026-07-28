package domain

import "time"

// GoalChainerProjectionRecord is AO's serialized copy of one project board.
// The lifecycle facts inside Projection remain authoritative in GoalChainer.
type GoalChainerProjectionRecord struct {
	ProjectID      ProjectID
	Protocol       string
	SourceRevision string
	Projection     string
	UpdatedAt      time.Time
}
