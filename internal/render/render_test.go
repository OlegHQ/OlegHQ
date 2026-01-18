package render

import (
	"testing"
	"time"

	"oleghq-readme-sync/internal/model"
)

func TestOpenSourceBlock_ContainsMarkers(t *testing.T) {
	block := OpenSourceBlock(OpenSourceInput{
		TopPRs: []model.PR{{
			Title:    "hello",
			URL:      "https://example.com",
			Number:   1,
			ClosedAt: time.Now(),
			Repo: model.RepoInfo{
				NameWithOwner: "a/b",
				URL:           "https://github.com/a/b",
			},
		}},
		Projects: []model.RepoInfo{{
			NameWithOwner: "me/project",
			URL:           "https://github.com/me/project",
			Description:   "desc",
		}},
		FullListPath:  "CONTRIBUTIONS.md",
		GeneratedTime: time.Now(),
	})

	if !containsSubstring(block, "DYNAMIC:OPEN_SOURCE:START") {
		t.Fatalf("expected start marker")
	}
	if !containsSubstring(block, "DYNAMIC:OPEN_SOURCE:END") {
		t.Fatalf("expected end marker")
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
