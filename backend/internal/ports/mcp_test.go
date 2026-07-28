package ports

import "testing"

func TestMCPServerLaunchValidate(t *testing.T) {
	valid := MCPServerLaunch{
		Name:    "oh-my-goals",
		Command: "/usr/bin/node",
		Args:    []string{"/opt/oh-my-goals/cli.js", "mcp"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid server rejected: %v", err)
	}

	for _, server := range []MCPServerLaunch{
		{Name: "../unsafe", Command: "/usr/bin/node"},
		{Name: "oh-my-goals", Command: " \t "},
		{Name: "oh-my-goals", Command: "node\x00bad"},
		{Name: "oh-my-goals", Command: "node", Args: []string{"mcp\x00bad"}},
	} {
		if err := server.Validate(); err == nil {
			t.Fatalf("invalid server accepted: %#v", server)
		}
	}
}
