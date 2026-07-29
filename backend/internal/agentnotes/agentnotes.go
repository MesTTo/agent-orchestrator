// Package agentnotes owns the confined MeTTa Atomspace files for stable project
// agents. Callers provide identities, never paths.
package agentnotes

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

const (
	// MaxReadBytes is the largest private notes projection returned by the API.
	MaxReadBytes int64 = 256 * 1024
	// MaxSnapshotBytes bounds validation work before any snapshot reaches an API
	// response or an Oh My Goals process.
	MaxSnapshotBytes int64 = 16 * 1024 * 1024
)

var (
	// ErrSnapshotVersion reports an unsupported snapshot version or fact head.
	ErrSnapshotVersion = errors.New("agent notes: unsupported snapshot version")
	// ErrSnapshotMalformed reports a non-canonical or identity-mismatched file.
	ErrSnapshotMalformed = errors.New("agent notes: malformed snapshot")
	// ErrSnapshotTooLarge reports a snapshot beyond the validation bound.
	ErrSnapshotTooLarge = errors.New("agent notes: snapshot exceeds size limit")
)

var snapshotArities = map[string]int{
	"MemoryProposition":    3,
	"MemoryContent":        2,
	"MemoryState":          3,
	"MemoryRecordedAt":     2,
	"MemoryTree":           2,
	"MemorySource":         4,
	"MemoryAssertionState": 3,
	"MemorySourceTruth":    4,
	"MemoryDerivation":     3,
	"MemorySupersedes":     2,
	"ClaimCore":            4,
}

// Paths are the internal absolute files injected into a linked Oh My Goals
// process. They are never serialized through the HTTP API.
type Paths struct {
	Private        string
	Project        string
	CreatedPrivate bool
	CreatedProject bool
}

// Metadata describes a confined private notes file without exposing its path.
type Metadata struct {
	SizeBytes  int64
	ModifiedAt time.Time
}

// Snapshot is a bounded read of one stable agent's private MeTTa notes.
type Snapshot struct {
	Metadata
	Content          string
	Truncated        bool
	Initialized      bool
	PropositionCount int
}

// ProjectRelativePath returns the generated shared project Atomspace location.
func ProjectRelativePath(projectID domain.ProjectID) string {
	return path.Join(projectRoot(projectID), "atomspaces", "shared.metta")
}

// AgentRelativePath returns the generated private stable-agent notes location.
func AgentRelativePath(
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) string {
	return path.Join(
		projectRoot(projectID),
		"atomspaces",
		"agents",
		string(agentID),
		"notes.metta",
	)
}

func projectRoot(projectID domain.ProjectID) string {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(projectID)))
	return path.Join("mcp", "oh-my-goals", "projects", key)
}

// Ensure creates the shared project and private stable-agent Atomspace files.
// Existing files are preserved byte-for-byte.
func Ensure(
	dataDir string,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
	storedPrivateRelativePath string,
) (Paths, error) {
	if strings.TrimSpace(dataDir) == "" {
		return Paths{}, errors.New("agent notes: AO data directory is empty")
	}
	expectedPrivate := AgentRelativePath(projectID, agentID)
	if storedPrivateRelativePath != expectedPrivate {
		return Paths{}, errors.New("agent notes: stored private path does not match identity")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return Paths{}, fmt.Errorf("agent notes: create data directory: %w", err)
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return Paths{}, fmt.Errorf("agent notes: open data directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	projectRelative := ProjectRelativePath(projectID)
	createdProject, err := ensureEmptyRegularFile(root, projectRelative)
	if err != nil {
		return Paths{}, err
	}
	createdPrivate, err := ensureEmptyRegularFile(root, expectedPrivate)
	if err != nil {
		return Paths{}, err
	}
	if _, err := readValidated(
		root,
		projectRelative,
		expectedHeader("project", projectID, ""),
		map[string]bool{"project": true, "derived": true},
	); err != nil {
		return Paths{}, fmt.Errorf("agent notes: validate shared snapshot: %w", err)
	}
	if _, err := readValidated(
		root,
		expectedPrivate,
		expectedHeader("agent", projectID, agentID),
		map[string]bool{"session": true},
	); err != nil {
		return Paths{}, fmt.Errorf("agent notes: validate private snapshot: %w", err)
	}
	return Paths{
		Private:        filepath.Join(dataDir, filepath.FromSlash(expectedPrivate)),
		Project:        filepath.Join(dataDir, filepath.FromSlash(projectRelative)),
		CreatedPrivate: createdPrivate,
		CreatedProject: createdProject,
	}, nil
}

// RemoveCreatedPrivate removes only the private file created by this Ensure
// result. It preserves existing files and the project-shared Atomspace.
func RemoveCreatedPrivate(
	dataDir string,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
	paths Paths,
) error {
	if !paths.CreatedPrivate {
		return nil
	}
	relative := AgentRelativePath(projectID, agentID)
	expected := filepath.Join(dataDir, filepath.FromSlash(relative))
	if paths.Private != expected {
		return errors.New("agent notes: cleanup path does not match identity")
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return fmt.Errorf("agent notes: open data directory for cleanup: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := root.Remove(relative); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("agent notes: remove created private file: %w", err)
	}
	// Best-effort only. A failure means another file now belongs to the agent or
	// the directory is already absent, either of which must be preserved.
	_ = root.Remove(path.Dir(relative))
	return nil
}

// Stat returns private notes metadata after checking the stored location still
// belongs to the requested project agent.
func Stat(
	dataDir string,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
	storedPrivateRelativePath string,
) (Metadata, error) {
	root, relative, err := openPrivate(dataDir, projectID, agentID, storedPrivateRelativePath)
	if err != nil {
		return Metadata{}, err
	}
	defer func() { _ = root.Close() }()
	info, err := root.Lstat(relative)
	if err != nil {
		return Metadata{}, fmt.Errorf("agent notes: stat private file: %w", err)
	}
	if err := requireRegular(info); err != nil {
		return Metadata{}, err
	}
	return Metadata{SizeBytes: info.Size(), ModifiedAt: info.ModTime().UTC()}, nil
}

// Read returns a bounded private notes projection. It never follows a caller
// path and never returns the shared project Atomspace.
func Read(
	dataDir string,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
	storedPrivateRelativePath string,
) (Snapshot, error) {
	root, relative, err := openPrivate(dataDir, projectID, agentID, storedPrivateRelativePath)
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = root.Close() }()
	info, err := root.Lstat(relative)
	if err != nil {
		return Snapshot{}, fmt.Errorf("agent notes: stat private file: %w", err)
	}
	if err := requireRegular(info); err != nil {
		return Snapshot{}, err
	}
	validated, err := readValidated(
		root,
		relative,
		expectedHeader("agent", projectID, agentID),
		map[string]bool{"session": true},
	)
	if err != nil {
		return Snapshot{}, err
	}
	content, truncated := boundedCanonical(validated.body)
	return Snapshot{
		Metadata: Metadata{
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		},
		Content:          content,
		Truncated:        truncated,
		Initialized:      len(validated.body) > 0,
		PropositionCount: validated.propositions,
	}, nil
}

func openPrivate(
	dataDir string,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
	storedPrivateRelativePath string,
) (*os.Root, string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, "", errors.New("agent notes: AO data directory is empty")
	}
	expected := AgentRelativePath(projectID, agentID)
	if storedPrivateRelativePath != expected {
		return nil, "", errors.New("agent notes: stored private path does not match identity")
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return nil, "", fmt.Errorf("agent notes: open data directory: %w", err)
	}
	return root, expected, nil
}

func ensureEmptyRegularFile(root *os.Root, relative string) (bool, error) {
	if err := root.MkdirAll(path.Dir(relative), 0o700); err != nil {
		return false, fmt.Errorf("agent notes: create Atomspace directory: %w", err)
	}
	file, err := root.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		if closeErr := file.Close(); closeErr != nil {
			return false, fmt.Errorf("agent notes: close new Atomspace file: %w", closeErr)
		}
		return true, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return false, fmt.Errorf("agent notes: create Atomspace file: %w", err)
	}
	info, err := root.Lstat(relative)
	if err != nil {
		return false, fmt.Errorf("agent notes: inspect existing Atomspace file: %w", err)
	}
	if err := requireRegular(info); err != nil {
		return false, err
	}
	if err := root.Chmod(relative, 0o600); err != nil {
		return false, fmt.Errorf("agent notes: secure Atomspace file: %w", err)
	}
	return false, nil
}

func requireRegular(info fs.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("agent notes: Atomspace path is not a regular file")
	}
	return nil
}

type validatedSnapshot struct {
	body         []byte
	propositions int
}

func readValidated(
	root *os.Root,
	relative string,
	header string,
	allowedScopes map[string]bool,
) (validatedSnapshot, error) {
	file, err := root.Open(relative)
	if err != nil {
		return validatedSnapshot{}, fmt.Errorf("agent notes: open snapshot: %w", err)
	}
	defer func() { _ = file.Close() }()
	body, err := io.ReadAll(io.LimitReader(file, MaxSnapshotBytes+1))
	if err != nil {
		return validatedSnapshot{}, fmt.Errorf("agent notes: read snapshot: %w", err)
	}
	if int64(len(body)) > MaxSnapshotBytes {
		return validatedSnapshot{}, ErrSnapshotTooLarge
	}
	if len(body) == 0 {
		return validatedSnapshot{body: body}, nil
	}
	if !utf8.Valid(body) || body[len(body)-1] != '\n' {
		return validatedSnapshot{}, ErrSnapshotMalformed
	}
	lines := strings.Split(string(body[:len(body)-1]), "\n")
	if len(lines) == 0 || lines[0] != header {
		if len(lines) > 0 {
			tokens, parseErr := topLevelTokens(lines[0])
			if parseErr == nil && len(tokens) == 5 && tokens[0] == "MemorySnapshot" {
				if tokens[1] != "oh-my-goals-atomspace-v1" {
					return validatedSnapshot{}, ErrSnapshotVersion
				}
				return validatedSnapshot{}, ErrSnapshotMalformed
			}
		}
		return validatedSnapshot{}, ErrSnapshotMalformed
	}
	previous := ""
	propositions := 0
	for _, line := range lines[1:] {
		if line == "" || (previous != "" && line <= previous) {
			return validatedSnapshot{}, ErrSnapshotMalformed
		}
		previous = line
		tokens, err := topLevelTokens(line)
		if err != nil || len(tokens) == 0 {
			return validatedSnapshot{}, ErrSnapshotMalformed
		}
		wantArity, ok := snapshotArities[tokens[0]]
		if !ok {
			return validatedSnapshot{}, ErrSnapshotVersion
		}
		if len(tokens)-1 != wantArity {
			return validatedSnapshot{}, ErrSnapshotMalformed
		}
		if tokens[0] == "MemoryProposition" {
			if !allowedScopes[tokens[2]] {
				return validatedSnapshot{}, ErrSnapshotMalformed
			}
			propositions++
		}
	}
	return validatedSnapshot{body: body, propositions: propositions}, nil
}

func expectedHeader(
	kind string,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) string {
	return "(MemorySnapshot oh-my-goals-atomspace-v1 " +
		kind + " " + strconv.Quote(string(projectID)) + " " +
		strconv.Quote(string(agentID)) + ")"
}

func topLevelTokens(line string) ([]string, error) {
	if len(line) < 2 || line[0] != '(' || line[len(line)-1] != ')' {
		return nil, ErrSnapshotMalformed
	}
	inner := line[1 : len(line)-1]
	var tokens []string
	for index := 0; index < len(inner); {
		for index < len(inner) && inner[index] == ' ' {
			index++
		}
		if index == len(inner) {
			break
		}
		start := index
		depth := 0
		quoted := false
		escaped := false
		for index < len(inner) {
			char := inner[index]
			if quoted {
				if escaped {
					escaped = false
				} else if char == '\\' {
					escaped = true
				} else if char == '"' {
					quoted = false
				}
				index++
				continue
			}
			switch char {
			case '"':
				// MeTTa permits quote characters inside symbols. A quote opens
				// a string only at an expression token boundary.
				if index == start || inner[index-1] == '(' || inner[index-1] == ' ' {
					quoted = true
				}
			case '(':
				depth++
			case ')':
				if depth == 0 {
					return nil, ErrSnapshotMalformed
				}
				depth--
			case ' ':
				if depth == 0 {
					goto tokenComplete
				}
			case '\t', '\r', '\n':
				return nil, ErrSnapshotMalformed
			}
			index++
		}
	tokenComplete:
		if quoted || depth != 0 || index == start {
			return nil, ErrSnapshotMalformed
		}
		tokens = append(tokens, inner[start:index])
	}
	return tokens, nil
}

func boundedCanonical(body []byte) (string, bool) {
	if int64(len(body)) <= MaxReadBytes {
		return string(body), false
	}
	lines := strings.SplitAfter(string(body), "\n")
	var out strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		if int64(out.Len()+len(line)) > MaxReadBytes {
			break
		}
		out.WriteString(line)
	}
	return out.String(), true
}
