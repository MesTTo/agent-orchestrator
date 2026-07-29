package sessionmanager

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/agentnotes"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestSpawnPreparesOhMyGoalsMCPProfile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test fixture uses a POSIX shell CLI")
	}
	workspacePath := t.TempDir()
	dataDir := t.TempDir()
	cliPath := writeOhMyGoalsFixture(t)
	store := newFakeStore()
	store.projects["research"] = domain.ProjectRecord{
		ID: "research",
		Config: domain.ProjectConfig{
			Env: map[string]string{ohMyGoalsCLIEnv: cliPath},
			Worker: domain.RoleOverride{
				Harness: domain.HarnessCodex,
			},
		},
	}
	agent := &recordingAgent{}
	runtimeController := &fakeRuntime{}
	workspace := &fakeWorkspace{path: workspacePath}
	manager := New(Deps{
		Runtime:   runtimeController,
		Agents:    singleAgent{agent: agent},
		Workspace: workspace,
		Store:     store,
		Messenger: &fakeMessenger{},
		Lifecycle: &fakeLCM{store: store},
		DataDir:   dataDir,
		LookPath:  func(string) (string, error) { return "/bin/true", nil },
		NewProjectAgentID: func() string {
			return firstProjectAgentID
		},
	})

	record, _, _, err := manager.Spawn(t.Context(), ports.SpawnConfig{
		ProjectID:        "research",
		Kind:             domain.KindWorker,
		MCPProfile:       domain.MCPProfileOhMyGoals,
		ProjectAgentKey:  "evidence-worker",
		ProjectAgentRole: "researcher",
		Prompt:           "investigate the claim",
	})
	if err != nil {
		t.Fatal(err)
	}

	if record.Metadata.MCPProfile != domain.MCPProfileOhMyGoals {
		t.Fatalf("stored MCP profile = %q", record.Metadata.MCPProfile)
	}
	if len(agent.lastLaunch.MCPServers) != 1 {
		t.Fatalf("MCP servers = %#v, want one server", agent.lastLaunch.MCPServers)
	}
	server := agent.lastLaunch.MCPServers[0]
	if server.Name != "oh-my-goals" || server.Command != cliPath {
		t.Fatalf("MCP server = %#v", server)
	}
	if len(server.Args) != 1 || server.Args[0] != "mcp" {
		t.Fatalf("MCP server args = %#v, want [mcp]", server.Args)
	}
	if _, err := os.Stat(filepath.Join(
		workspacePath,
		".agents",
		"skills",
		"oh-my-goals",
		"SKILL.md",
	)); err != nil {
		t.Fatalf("installed skill: %v", err)
	}
	if !containsString(
		workspace.excludePatterns,
		"/.agents/skills/oh-my-goals/",
	) {
		t.Fatalf("exclude patterns = %#v", workspace.excludePatterns)
	}
	env := runtimeController.lastCfg.Env
	if env[ohMyGoalsRepositoryEnv] != "research" {
		t.Fatalf("repository identity = %q", env[ohMyGoalsRepositoryEnv])
	}
	if env[ohMyGoalsSessionEnv] != string(record.ID) {
		t.Fatalf("session identity = %q", env[ohMyGoalsSessionEnv])
	}
	if env[ohMyGoalsAgentIDEnv] != firstProjectAgentID ||
		env[ohMyGoalsAttemptEnv] != "1" {
		t.Fatalf("Oh My Goals stable identity = %q attempt = %q", env[ohMyGoalsAgentIDEnv], env[ohMyGoalsAttemptEnv])
	}
	if env[EnvProjectAgentID] != firstProjectAgentID ||
		env[EnvProjectAgentRole] != "researcher" ||
		env[EnvProjectAgentAttempt] != "1" {
		t.Fatalf("AO stable project-agent environment = %#v", env)
	}
	wantPrivate := filepath.Join(
		dataDir,
		filepath.FromSlash(agentnotes.AgentRelativePath("research", firstProjectAgentID)),
	)
	wantShared := filepath.Join(
		dataDir,
		filepath.FromSlash(agentnotes.ProjectRelativePath("research")),
	)
	if env[ohMyGoalsAtomspacePathEnv] != wantPrivate ||
		env[ohMyGoalsProjectAtomspacePathEnv] != wantShared {
		t.Fatalf(
			"Atomspace paths = private:%q shared:%q",
			env[ohMyGoalsAtomspacePathEnv],
			env[ohMyGoalsProjectAtomspacePathEnv],
		)
	}
	for _, path := range []string{wantPrivate, wantShared} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Atomspace %q: %v", path, err)
		}
	}
	wantDBRoot := filepath.Join(dataDir, "mcp", "oh-my-goals", "projects")
	if got := env[ohMyGoalsMemoryDBEnv]; !strings.HasPrefix(got, wantDBRoot) ||
		filepath.Base(got) != "memory.db" {
		t.Fatalf("memory database = %q, want under %q", got, wantDBRoot)
	}

	terminal := store.sessions[record.ID]
	terminal.IsTerminated = true
	terminal.Activity = domain.Activity{State: domain.ActivityExited}
	terminal.Metadata.AgentSessionID = "native-agent-1"
	store.sessions[record.ID] = terminal
	if _, err := manager.RestoreWithMode(t.Context(), record.ID); err != nil {
		t.Fatalf("restore linked Oh My Goals session: %v", err)
	}
	restoredEnv := runtimeController.lastCfg.Env
	if restoredEnv[ohMyGoalsSessionEnv] != string(record.ID) ||
		restoredEnv[ohMyGoalsAgentIDEnv] != firstProjectAgentID ||
		restoredEnv[ohMyGoalsAttemptEnv] != "1" ||
		restoredEnv[ohMyGoalsAtomspacePathEnv] != wantPrivate ||
		restoredEnv[ohMyGoalsProjectAtomspacePathEnv] != wantShared {
		t.Fatalf("restored Oh My Goals environment = %#v", restoredEnv)
	}
}

func TestResolveOhMyGoalsNodeCLIAndAgentTargets(t *testing.T) {
	cliPath := filepath.Join(t.TempDir(), "cli.js")
	if err := os.WriteFile(cliPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	agent := &recordingAgent{}
	manager := New(Deps{
		Agents: singleAgent{agent: agent},
		LookPath: func(name string) (string, error) {
			if name == "node" {
				return "/usr/bin/node", nil
			}
			return "", errors.New("not found")
		},
	})
	resolved, err := manager.resolveMCPProfile(
		t.Context(),
		domain.MCPProfileOhMyGoals,
		domain.HarnessCodex,
		map[string]string{ohMyGoalsCLIEnv: cliPath},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.server.Command != "/usr/bin/node" ||
		!slices.Equal(resolved.server.Args, []string{cliPath, "mcp"}) {
		t.Fatalf("server launch = %#v", resolved.server)
	}
	if !slices.Equal(resolved.installer, []string{
		"/usr/bin/node",
		cliPath,
		"install-skill",
		"--agent",
		"codex",
		"--scope",
		"project",
	}) {
		t.Fatalf("installer = %#v", resolved.installer)
	}

	targets := map[domain.AgentHarness][2]string{
		domain.HarnessClaudeCode: {"claude", ".claude/skills/oh-my-goals"},
		domain.HarnessCodex:      {"codex", ".agents/skills/oh-my-goals"},
		domain.HarnessOpenCode:   {"opencode", ".opencode/skills/oh-my-goals"},
	}
	for harness, want := range targets {
		target, skillPath, ok := ohMyGoalsAgentTarget(harness)
		if !ok || target != want[0] || skillPath != want[1] {
			t.Fatalf(
				"target for %s = %q, %q, %v",
				harness,
				target,
				skillPath,
				ok,
			)
		}
	}
}

func TestExternalSourceRetryDoesNotRequireMCPRuntimeResolution(t *testing.T) {
	base := newFakeStore()
	base.projects["research"] = domain.ProjectRecord{
		ID: "research",
		Config: domain.ProjectConfig{
			Env: map[string]string{
				ohMyGoalsCLIEnv: filepath.Join(t.TempDir(), "now-missing"),
			},
			Worker: domain.RoleOverride{
				Harness: domain.HarnessCodex,
			},
		},
	}
	existing := domain.SessionRecord{
		ID:        "research-1",
		ProjectID: "research",
		Kind:      domain.KindWorker,
		Harness:   domain.HarnessCodex,
		Metadata: domain.SessionMetadata{
			RuntimeHandleID: "runtime-1",
			Prompt:          "investigate",
			MCPProfile:      domain.MCPProfileOhMyGoals,
		},
	}
	base.sessions[existing.ID] = existing
	store := &blockingExternalStore{
		fakeStore: base,
		sources: map[string]domain.SessionID{
			"research\x00goalchainer:thread:task:1": existing.ID,
		},
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	close(store.release)
	manager := New(Deps{
		Runtime:   &fakeRuntime{},
		Agents:    singleAgent{agent: &recordingAgent{}},
		Workspace: &fakeWorkspace{},
		Store:     store,
		Messenger: &fakeMessenger{},
		Lifecycle: &fakeLCM{store: base},
		DataDir:   t.TempDir(),
	})

	record, _, _, created, err := manager.SpawnFromExternalSource(
		t.Context(),
		ports.SpawnConfig{
			ProjectID:    "research",
			SourceTaskID: "goalchainer:thread:task:1",
			Kind:         domain.KindWorker,
			MCPProfile:   domain.MCPProfileOhMyGoals,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if created || record.ID != existing.ID {
		t.Fatalf("retry = %#v, created=%v", record, created)
	}
}

func TestRestoreArgvForwardsMCPServers(t *testing.T) {
	agent := &recordingAgent{}
	servers := []ports.MCPServerLaunch{{
		Name:    "oh-my-goals",
		Command: "/usr/bin/node",
		Args:    []string{"/opt/oh-my-goals/cli.js", "mcp"},
	}}
	command, _, mode, err := restoreArgv(
		t.Context(),
		agent,
		"research-1",
		"/workspace",
		domain.SessionMetadata{AgentSessionID: "native-1"},
		"",
		"",
		ports.AgentConfig{},
		domain.KindWorker,
		domain.HarnessCodex,
		t.TempDir(),
		servers,
	)
	if err != nil {
		t.Fatal(err)
	}
	if mode != RestoreModeNative || !slices.Equal(command, []string{"resume"}) {
		t.Fatalf("restore command = %#v, mode=%q", command, mode)
	}
	if len(agent.lastRestore.MCPServers) != 1 ||
		agent.lastRestore.MCPServers[0].Name != "oh-my-goals" {
		t.Fatalf("restore MCP servers = %#v", agent.lastRestore.MCPServers)
	}
}

func TestSpawnRejectsMCPPreflightFailureBeforeCreatingSession(t *testing.T) {
	tests := []struct {
		name    string
		harness domain.AgentHarness
		cli     string
		agent   ports.Agent
		want    error
	}{
		{
			name:    "unsupported harness",
			harness: domain.HarnessAider,
			cli:     "/does/not/matter",
			agent:   fakeAgent{},
			want:    ErrMCPProfileUnsupported,
		},
		{
			name:    "unavailable CLI",
			harness: domain.HarnessCodex,
			cli:     filepath.Join(t.TempDir(), "missing"),
			agent:   &recordingAgent{},
			want:    ErrMCPProfileUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeStore()
			store.projects["research"] = domain.ProjectRecord{
				ID: "research",
				Config: domain.ProjectConfig{
					Env: map[string]string{ohMyGoalsCLIEnv: test.cli},
					Worker: domain.RoleOverride{
						Harness: test.harness,
					},
				},
			}
			manager := New(Deps{
				Agents:  singleAgent{agent: test.agent},
				Store:   store,
				DataDir: t.TempDir(),
			})

			_, _, _, err := manager.Spawn(t.Context(), ports.SpawnConfig{
				ProjectID:  "research",
				Kind:       domain.KindWorker,
				MCPProfile: domain.MCPProfileOhMyGoals,
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("spawn error = %v, want %v", err, test.want)
			}
			if len(store.sessions) != 0 {
				t.Fatalf("sessions created after preflight failure: %#v", store.sessions)
			}
		})
	}
}

func writeOhMyGoalsFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "oh-my-goals")
	script := `#!/bin/sh
set -eu
root=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --project-root)
      root="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
test -n "$root"
mkdir -p "$root/.agents/skills/oh-my-goals"
printf '%s\n' '# Oh My Goals' > "$root/.agents/skills/oh-my-goals/SKILL.md"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
