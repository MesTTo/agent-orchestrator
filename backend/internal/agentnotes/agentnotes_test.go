package agentnotes

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

const testAgentID domain.ProjectAgentID = "7e66da4a-2b4f-4e87-865f-14f554070faa"
const otherTestAgentID domain.ProjectAgentID = "272a4c63-64b9-47a0-9624-7fc0fff46aa1"

func mustEnsure(
	t *testing.T,
	dataDir string,
	projectID domain.ProjectID,
	agentID domain.ProjectAgentID,
) Paths {
	t.Helper()
	paths, err := Ensure(dataDir, projectID, agentID, AgentRelativePath(projectID, agentID))
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func mustWriteAndReadPrivateSnapshot(t *testing.T, dataDir, body string) Snapshot {
	t.Helper()
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	if err := os.WriteFile(paths.Private, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Read(
		dataDir,
		"project-a",
		testAgentID,
		AgentRelativePath("project-a", testAgentID),
	)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestEnsureCreatesSeparateSharedAndPrivateFiles(t *testing.T) {
	dataDir := t.TempDir()
	private := AgentRelativePath("project-a", testAgentID)
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	if paths.Private == paths.Project {
		t.Fatal("private and project paths must differ")
	}
	for _, file := range []string{paths.Private, paths.Project} {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", file, info.Mode().Perm())
		}
	}
	other := AgentRelativePath("project-b", testAgentID)
	if other == private {
		t.Fatal("different projects resolved to the same private path")
	}
}

func TestPrivateNotesStayIsolatedAcrossProjectsAndStableAgents(t *testing.T) {
	dataDir := t.TempDir()
	type identity struct {
		project domain.ProjectID
		agent   domain.ProjectAgentID
		content string
	}
	identities := []identity{
		{project: "project-a", agent: testAgentID, content: "alpha"},
		{project: "project-a", agent: otherTestAgentID, content: "beta"},
		{project: "project-b", agent: testAgentID, content: "gamma"},
	}
	paths := make([]Paths, 0, len(identities))
	for index, identity := range identities {
		created := mustEnsure(t, dataDir, identity.project, identity.agent)
		body := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"" +
			string(identity.project) + "\" \"" + string(identity.agent) + "\")\n" +
			"(MemoryContent note-1 \"" + identity.content + "\")\n" +
			"(MemoryProposition note-1 session observation)\n"
		if err := os.WriteFile(created.Private, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		for previous := 0; previous < index; previous++ {
			if paths[previous].Private == created.Private {
				t.Fatalf("private path collision: %q", created.Private)
			}
			if identities[previous].project != identity.project &&
				paths[previous].Project == created.Project {
				t.Fatalf("shared project path collision: %q", created.Project)
			}
		}
		paths = append(paths, created)
	}

	for index, identity := range identities {
		snapshot, err := Read(
			dataDir,
			identity.project,
			identity.agent,
			AgentRelativePath(identity.project, identity.agent),
		)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(snapshot.Content, `"`+identity.content+`"`) {
			t.Fatalf("snapshot %d missing own content: %q", index, snapshot.Content)
		}
		for otherIndex, other := range identities {
			if otherIndex != index &&
				strings.Contains(snapshot.Content, `"`+other.content+`"`) {
				t.Fatalf(
					"snapshot %d leaked content from %d: %q",
					index,
					otherIndex,
					snapshot.Content,
				)
			}
		}
	}
}

func TestRemoveCreatedPrivatePreservesSharedAndExistingPrivate(t *testing.T) {
	dataDir := t.TempDir()
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	if !paths.CreatedPrivate || !paths.CreatedProject {
		t.Fatalf("created = private:%v project:%v", paths.CreatedPrivate, paths.CreatedProject)
	}
	if err := RemoveCreatedPrivate(dataDir, "project-a", testAgentID, paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.Private); !os.IsNotExist(err) {
		t.Fatalf("private stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(paths.Project); err != nil {
		t.Fatalf("shared file removed: %v", err)
	}

	recreated := mustEnsure(t, dataDir, "project-a", testAgentID)
	if !recreated.CreatedPrivate {
		t.Fatal("expected missing private file to be recreated")
	}
	existing := mustEnsure(t, dataDir, "project-a", testAgentID)
	if existing.CreatedPrivate {
		t.Fatal("existing private should not be reported as created")
	}
	if err := RemoveCreatedPrivate(dataDir, "project-a", testAgentID, existing); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(existing.Private); err != nil {
		t.Fatalf("existing private removed: %v", err)
	}
}

func TestEnsurePreservesExistingNotes(t *testing.T) {
	dataDir := t.TempDir()
	private := AgentRelativePath("project-a", testAgentID)
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	want := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"project-a\" \"" +
		string(testAgentID) + "\")\n" +
		"(MemoryContent note-1 \"persistent\")\n" +
		"(MemoryProposition note-1 session observation)\n"
	if err := os.WriteFile(paths.Private, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Ensure(dataDir, "project-a", testAgentID, private); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths.Private)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("notes = %q, want %q", got, want)
	}
}

func TestReadIsBoundedAndRejectsStoredPathMismatch(t *testing.T) {
	dataDir := t.TempDir()
	private := AgentRelativePath("project-a", testAgentID)
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	body := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"project-a\" \"" +
		string(testAgentID) + "\")\n" +
		"(MemoryContent note-1 \"" + strings.Repeat("x", int(MaxReadBytes)+4) + "\")\n" +
		"(MemoryProposition note-1 session observation)\n"
	if err := os.WriteFile(paths.Private, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dataDir, "project-a", testAgentID, private)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Truncated || got.SizeBytes != int64(len(body)) || !got.Initialized || got.PropositionCount != 1 {
		t.Fatalf("snapshot = %#v", got)
	}
	if _, err := Read(dataDir, "project-a", testAgentID, filepath.Join("elsewhere", "notes.metta")); err == nil {
		t.Fatal("expected stored path mismatch")
	}
}

func TestReadRejectsUnknownSnapshotVersionAndHead(t *testing.T) {
	dataDir := t.TempDir()
	private := AgentRelativePath("project-a", testAgentID)
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	for _, body := range []string{
		"(MemorySnapshot oh-my-goals-atomspace-v2 agent \"project-a\" \"" + string(testAgentID) + "\")\n",
		"(MemorySnapshot oh-my-goals-atomspace-v1 agent \"project-a\" \"" + string(testAgentID) + "\")\n(Execute arbitrary)\n",
	} {
		if err := os.WriteFile(paths.Private, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(dataDir, "project-a", testAgentID, private); !errors.Is(err, ErrSnapshotVersion) {
			t.Fatalf("error = %v, want snapshot version error", err)
		}
	}
}

func TestSnapshotParserRejectsMalformedOrForeignFactsWithoutExecution(t *testing.T) {
	dataDir := t.TempDir()
	private := AgentRelativePath("project-a", testAgentID)
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	header := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"project-a\" \"" +
		string(testAgentID) + "\")\n"
	cases := []struct {
		name string
		body string
		want error
	}{
		{
			name: "unclosed string",
			body: header + "(MemoryContent note-1 \"unterminated)\n",
			want: ErrSnapshotMalformed,
		},
		{
			name: "tab delimiter",
			body: header + "(MemoryContent\tnote-1 \"content\")\n",
			want: ErrSnapshotMalformed,
		},
		{
			name: "comment line",
			body: header + "; never evaluate me\n",
			want: ErrSnapshotMalformed,
		},
		{
			name: "wrong arity",
			body: header + "(MemoryContent note-1)\n",
			want: ErrSnapshotMalformed,
		},
		{
			name: "unknown head",
			body: header + "(RunCommand \"rm\" \"-rf\")\n",
			want: ErrSnapshotVersion,
		},
		{
			name: "project scope in private file",
			body: header + "(MemoryProposition note-1 project observation)\n",
			want: ErrSnapshotMalformed,
		},
		{
			name: "wrong stable agent header",
			body: "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"project-a\" \"272a4c63-64b9-47a0-9624-7fc0fff46aa1\")\n",
			want: ErrSnapshotMalformed,
		},
		{
			name: "unsorted facts",
			body: header +
				"(MemoryProposition note-1 session observation)\n" +
				"(MemoryContent note-1 \"content\")\n",
			want: ErrSnapshotMalformed,
		},
		{
			name: "duplicate facts",
			body: header +
				"(MemoryContent note-1 \"content\")\n" +
				"(MemoryContent note-1 \"content\")\n",
			want: ErrSnapshotMalformed,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(paths.Private, []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Read(dataDir, "project-a", testAgentID, private); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestSnapshotParserAcceptsEscapedStringsAndNestedTerms(t *testing.T) {
	dataDir := t.TempDir()
	body := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"project-a\" \"" +
		string(testAgentID) + "\")\n" +
		"(MemoryContent note-1 \"quoted \\\\\"value\\\\\"\")\n" +
		"(MemoryDerivation note-1 rule-1 (premise-1 premise-2))\n" +
		"(MemoryProposition note-1 session observation)\n"
	snapshot := mustWriteAndReadPrivateSnapshot(t, dataDir, body)
	if snapshot.PropositionCount != 1 || snapshot.Content != body {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestSnapshotParserAcceptsQuoteInsideSymbol(t *testing.T) {
	dataDir := t.TempDir()
	body := "(MemorySnapshot oh-my-goals-atomspace-v1 agent \"project-a\" \"" +
		string(testAgentID) + "\")\n" +
		"(MemoryContent note-1 quoted\"id)\n" +
		"(MemoryProposition note-1 session observation)\n"
	snapshot := mustWriteAndReadPrivateSnapshot(t, dataDir, body)
	if snapshot.Content != body {
		t.Fatalf("content = %q, want %q", snapshot.Content, body)
	}
}

func TestTopLevelTokensDistinguishesStringsFromQuotedSymbols(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{
			line: `(MemoryContent note-1 quoted"id)`,
			want: []string{"MemoryContent", "note-1", `quoted"id`},
		},
		{
			line: `(MemoryContent note-1 "quoted \"value\"")`,
			want: []string{"MemoryContent", "note-1", `"quoted \"value\""`},
		},
		{
			line: `(MemoryDerivation note-1 rule-1 (premise "quoted value"))`,
			want: []string{"MemoryDerivation", "note-1", "rule-1", `(premise "quoted value")`},
		},
	}
	for _, test := range tests {
		got, err := topLevelTokens(test.line)
		if err != nil {
			t.Fatalf("topLevelTokens(%q): %v", test.line, err)
		}
		if len(got) != len(test.want) {
			t.Fatalf("topLevelTokens(%q) = %#v, want %#v", test.line, got, test.want)
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Fatalf("topLevelTokens(%q) = %#v, want %#v", test.line, got, test.want)
			}
		}
	}
}

func TestReadRejectsSymlinkAndInvalidUTF8(t *testing.T) {
	dataDir := t.TempDir()
	private := AgentRelativePath("project-a", testAgentID)
	paths := mustEnsure(t, dataDir, "project-a", testAgentID)
	if err := os.WriteFile(paths.Private, []byte{0xff}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dataDir, "project-a", testAgentID, private); err == nil {
		t.Fatal("expected invalid UTF-8 error")
	}
	if err := os.Remove(paths.Private); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(paths.Project, paths.Private); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dataDir, "project-a", testAgentID, private); err == nil {
		t.Fatal("expected symlink error")
	}
}
