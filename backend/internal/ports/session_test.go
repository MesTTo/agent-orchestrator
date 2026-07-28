package ports_test

import (
	"regexp"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func FuzzValidExternalSourceTaskIDMatchesTheAPIGrammar(f *testing.F) {
	f.Add("goalchainer:incident-42:collect-evidence:1")
	f.Add("../invalid")
	pattern := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

	f.Fuzz(func(t *testing.T, value string) {
		got := ports.ValidExternalSourceTaskID(value)
		want := pattern.MatchString(value)
		if got != want {
			t.Fatalf("ValidExternalSourceTaskID(%q) = %v, want %v", value, got, want)
		}
	})
}
