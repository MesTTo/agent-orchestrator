package sessionmanager

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	aoprocess "github.com/aoagents/agent-orchestrator/backend/internal/process"
)

const (
	ohMyGoalsCLIEnv                  = "OH_MY_GOALS_CLI"
	ohMyGoalsMemoryDBEnv             = "OH_MY_GOALS_MEMORY_DB"
	ohMyGoalsRepositoryEnv           = "OH_MY_GOALS_REPOSITORY"
	ohMyGoalsSessionEnv              = "OH_MY_GOALS_SESSION"
	ohMyGoalsAgentIDEnv              = "OH_MY_GOALS_AGENT_ID"
	ohMyGoalsAttemptEnv              = "OH_MY_GOALS_ATTEMPT"
	ohMyGoalsAtomspacePathEnv        = "OH_MY_GOALS_ATOMSPACE_PATH"
	ohMyGoalsProjectAtomspacePathEnv = "OH_MY_GOALS_PROJECT_ATOMSPACE_PATH"
	mcpProfileSetupTimeout           = 30 * time.Second
	maxMCPSetupOutputBytes           = 16 * 1024
)

type mcpProfileRuntime struct {
	server    ports.MCPServerLaunch
	installer []string
	skillPath string
}

func (m *Manager) validateMCPProfileSupport(
	profile domain.MCPProfile,
	harness domain.AgentHarness,
	agent ports.Agent,
) error {
	if profile == "" {
		return nil
	}
	if !profile.Valid() || profile != domain.MCPProfileOhMyGoals {
		return fmt.Errorf("%w: %q", ErrMCPProfileUnsupported, profile)
	}
	support, ok := agent.(ports.MCPServerAgent)
	if !ok || !support.SupportsMCPServers() {
		return fmt.Errorf("%w: %s", ErrMCPProfileUnsupported, harness)
	}
	if _, _, ok := ohMyGoalsAgentTarget(harness); !ok {
		return fmt.Errorf("%w: %s", ErrMCPProfileUnsupported, harness)
	}
	return nil
}

func (m *Manager) resolveMCPProfile(
	ctx context.Context,
	profile domain.MCPProfile,
	harness domain.AgentHarness,
	projectEnv map[string]string,
) (mcpProfileRuntime, error) {
	if profile == "" {
		return mcpProfileRuntime{}, nil
	}
	agent, ok := m.agents.Agent(harness)
	if !ok {
		return mcpProfileRuntime{}, fmt.Errorf("%w: %s", ErrMCPProfileUnsupported, harness)
	}
	if err := m.validateMCPProfileSupport(profile, harness, agent); err != nil {
		return mcpProfileRuntime{}, err
	}
	target, skillPath, _ := ohMyGoalsAgentTarget(harness)
	cli, err := m.resolveOhMyGoalsCLI(ctx, projectEnv)
	if err != nil {
		return mcpProfileRuntime{}, err
	}
	command := cli
	prefix := make([]string, 1, 6)
	prefix[0] = cli
	if isNodeScript(cli) {
		node, err := m.lookPath("node")
		if err != nil {
			return mcpProfileRuntime{}, fmt.Errorf(
				"%w: node is required to run %s", ErrMCPProfileUnavailable, cli,
			)
		}
		command = node
		prefix = append(prefix[:0], node, cli)
	}
	return mcpProfileRuntime{
		server: ports.MCPServerLaunch{
			Name:    string(domain.MCPProfileOhMyGoals),
			Command: command,
			Args:    append(append([]string(nil), prefix[1:]...), "mcp"),
		},
		installer: append(prefix,
			"install-skill",
			"--agent", target,
			"--scope", "project",
		),
		skillPath: skillPath,
	}, nil
}

func (m *Manager) resolveOhMyGoalsCLI(
	ctx context.Context,
	projectEnv map[string]string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	configured := strings.TrimSpace(projectEnv[ohMyGoalsCLIEnv])
	if configured == "" {
		path, err := m.lookPath("oh-my-goals")
		if err != nil {
			return "", fmt.Errorf(
				"%w: set %s to the built Oh My Goals CLI",
				ErrMCPProfileUnavailable,
				ohMyGoalsCLIEnv,
			)
		}
		return path, nil
	}
	if filepath.IsAbs(configured) {
		info, err := os.Stat(configured)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf(
				"%w: %s does not name a regular file",
				ErrMCPProfileUnavailable,
				ohMyGoalsCLIEnv,
			)
		}
		return configured, nil
	}
	if strings.ContainsAny(configured, `/\`) {
		return "", fmt.Errorf(
			"%w: %s must be absolute or a command name",
			ErrMCPProfileUnavailable,
			ohMyGoalsCLIEnv,
		)
	}
	path, err := m.lookPath(configured)
	if err != nil {
		return "", fmt.Errorf(
			"%w: cannot resolve %s",
			ErrMCPProfileUnavailable,
			configured,
		)
	}
	return path, nil
}

func (m *Manager) prepareMCPProfile(
	ctx context.Context,
	runtimeConfig mcpProfileRuntime,
	id domain.SessionID,
	projectID domain.ProjectID,
	workspace ports.WorkspaceInfo,
	env map[string]string,
) ([]ports.MCPServerLaunch, error) {
	if runtimeConfig.server.Name == "" {
		return nil, nil
	}
	if strings.TrimSpace(m.dataDir) == "" {
		return nil, fmt.Errorf("%w: AO data directory is empty", ErrMCPProfileUnavailable)
	}
	projectKey := fmt.Sprintf("%x", sha256.Sum256([]byte(projectID)))
	env[ohMyGoalsMemoryDBEnv] = filepath.Join(
		m.dataDir,
		"mcp",
		"oh-my-goals",
		"projects",
		projectKey,
		"memory.db",
	)
	env[ohMyGoalsRepositoryEnv] = string(projectID)
	env[ohMyGoalsSessionEnv] = string(id)

	setupCtx, cancel := context.WithTimeout(ctx, mcpProfileSetupTimeout)
	defer cancel()
	args := append(
		append([]string(nil), runtimeConfig.installer[1:]...),
		"--project-root", workspace.Path,
	)
	cmd := aoprocess.CommandContext(setupCtx, runtimeConfig.installer[0], args...)
	cmd.Dir = workspace.Path
	cmd.Env = commandEnvironment(env)
	output := &boundedOutput{limit: maxMCPSetupOutputBytes}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		if setupCtx.Err() != nil {
			return nil, fmt.Errorf(
				"%w: Oh My Goals skill setup timed out: %w",
				ErrMCPProfileUnavailable,
				setupCtx.Err(),
			)
		}
		return nil, fmt.Errorf(
			"%w: Oh My Goals skill setup failed: %w: %s",
			ErrMCPProfileUnavailable,
			err,
			strings.TrimSpace(output.String()),
		)
	}
	if err := m.workspace.AddExclude(
		ctx,
		workspace,
		"/"+filepath.ToSlash(runtimeConfig.skillPath)+"/",
	); err != nil {
		return nil, fmt.Errorf(
			"%w: exclude Oh My Goals skill: %w",
			ErrMCPProfileUnavailable,
			err,
		)
	}
	return []ports.MCPServerLaunch{runtimeConfig.server}, nil
}

func ohMyGoalsAgentTarget(harness domain.AgentHarness) (target, skillPath string, ok bool) {
	switch harness {
	case domain.HarnessClaudeCode:
		return "claude", ".claude/skills/oh-my-goals", true
	case domain.HarnessCodex:
		return "codex", ".agents/skills/oh-my-goals", true
	case domain.HarnessOpenCode:
		return "opencode", ".opencode/skills/oh-my-goals", true
	default:
		return "", "", false
	}
}

func isNodeScript(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs":
		return true
	default:
		return false
	}
}

func commandEnvironment(overrides map[string]string) []string {
	env := make(map[string]string, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	for key, value := range overrides {
		env[key] = value
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

type boundedOutput struct {
	data  []byte
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.limit > len(b.data) {
		remaining := b.limit - len(b.data)
		b.data = append(b.data, p[:min(len(p), remaining)]...)
	}
	return len(p), nil
}

func (b *boundedOutput) String() string {
	return string(b.data)
}

var _ io.Writer = (*boundedOutput)(nil)
