package ports

import (
	"fmt"
	"regexp"
	"strings"
)

var mcpServerNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

// MCPServerLaunch is one additive stdio server made available to an agent
// session. AO resolves these from typed profiles. API callers cannot provide
// executable paths or arguments directly.
type MCPServerLaunch struct {
	Name    string
	Command string
	Args    []string
}

// Validate refuses values that cannot be represented consistently in the
// Claude Code, Codex, and OpenCode configuration formats.
func (s MCPServerLaunch) Validate() error {
	if !mcpServerNamePattern.MatchString(s.Name) {
		return fmt.Errorf("MCP server name %q is invalid", s.Name)
	}
	if strings.TrimSpace(s.Command) == "" || strings.ContainsRune(s.Command, '\x00') {
		return fmt.Errorf("MCP server %s command is invalid", s.Name)
	}
	for i, arg := range s.Args {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("MCP server %s argument %d contains a null byte", s.Name, i)
		}
	}
	return nil
}

// MCPServerAgent marks adapters that consume LaunchConfig.MCPServers and
// RestoreConfig.MCPServers rather than silently dropping requested tools.
type MCPServerAgent interface {
	SupportsMCPServers() bool
}
