package domain

import "testing"

func FuzzProjectAgentKeyAndRoleValidators(f *testing.F) {
	f.Add("evidence-worker")
	f.Add("agent/path")
	f.Add("")
	f.Fuzz(func(t *testing.T, value string) {
		keyValid := ValidProjectAgentKey(ProjectAgentKey(value))
		roleValid := ValidProjectAgentRole(ProjectAgentRole(value))
		if keyValid != roleValid {
			t.Fatalf("key and role validator drift for %q", value)
		}
		if len(value) == 0 || len(value) > MaxProjectAgentKeyBytes {
			if keyValid {
				t.Fatalf("accepted out-of-bound identifier %q", value)
			}
		}
	})
}

func FuzzProjectAgentIDRequiresCanonicalUUID(f *testing.F) {
	f.Add("7e66da4a-2b4f-4e87-865f-14f554070faa")
	f.Add("7E66DA4A-2B4F-4E87-865F-14F554070FAA")
	f.Add("caller-key")
	f.Fuzz(func(t *testing.T, value string) {
		valid := ValidProjectAgentID(ProjectAgentID(value))
		if valid && len(value) != 36 {
			t.Fatalf("accepted noncanonical UUID %q", value)
		}
	})
}
