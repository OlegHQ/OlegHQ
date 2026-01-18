package sync

import "testing"

func TestUpsertOpenSourceBlock_InsertsWhenMissing(t *testing.T) {
	readme := "# Title\n\n---\n\n### 03 — Open Source\n\nold\n\n---\n\n### 04 — Background\n"
	block := "<!-- DYNAMIC:OPEN_SOURCE:START -->\nnew\n<!-- DYNAMIC:OPEN_SOURCE:END -->"

	got, err := upsertOpenSourceBlock(readme, block)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == readme {
		t.Fatalf("expected README to change")
	}
	if !containsSubstring(got, block) {
		t.Fatalf("expected block to be inserted")
	}
}

func TestUpsertOpenSourceBlock_ReplacesWhenPresent(t *testing.T) {
	readme := "# Title\n\n### 03 — Open Source\n\n<!-- DYNAMIC:OPEN_SOURCE:START -->\na\n<!-- DYNAMIC:OPEN_SOURCE:END -->\n\n---\n"
	block := "<!-- DYNAMIC:OPEN_SOURCE:START -->\nb\n<!-- DYNAMIC:OPEN_SOURCE:END -->"

	got, err := upsertOpenSourceBlock(readme, block)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsSubstring(got, "\nb\n") {
		t.Fatalf("expected replacement")
	}
	if containsSubstring(got, "\na\n") {
		t.Fatalf("did not expect old content")
	}
}

func containsSubstring(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
