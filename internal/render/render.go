package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"oleghq-readme-sync/internal/model"
)

type OpenSourceInput struct {
	TopPRs        []model.PR
	Projects      []model.RepoInfo
	FullListPath  string
	GeneratedTime time.Time
}

func OpenSourceBlock(in OpenSourceInput) string {
	var b strings.Builder
	b.WriteString("<!-- DYNAMIC:OPEN_SOURCE:START -->\n\n")

	b.WriteString("**Latest merged OSS contributions**\n\n")
	if len(in.TopPRs) == 0 {
		b.WriteString("- (no recent merged PRs found)\n")
	} else {
		for _, pr := range in.TopPRs {
			repoPR := fmt.Sprintf("%s#%d", pr.Repo.NameWithOwner, pr.Number)
			b.WriteString(fmt.Sprintf("- [%s](%s) — %s\n", repoPR, pr.URL, escapeInline(pr.Title)))
		}
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("_Full history: [%s](%s)_\n\n", in.FullListPath, in.FullListPath))

	b.WriteString("**Projects I'm working on**\n\n")
	if len(in.Projects) == 0 {
		b.WriteString("- (no recent projects found)\n")
	} else {
		for _, r := range in.Projects {
			desc := strings.TrimSpace(r.Description)
			if desc == "" {
				desc = "(no description)"
			}
			// Use just the repo name, not owner/name
			repoName := r.NameWithOwner
			if idx := strings.Index(repoName, "/"); idx >= 0 {
				repoName = repoName[idx+1:]
			}
			b.WriteString(fmt.Sprintf("- **[%s](%s)** — %s\n", repoName, r.URL, escapeInline(desc)))
		}
	}

	b.WriteString("\n<!-- DYNAMIC:OPEN_SOURCE:END -->")
	return b.String()
}

type ContributionsInput struct {
	PRs           []model.PR
	GeneratedTime time.Time
}

func ContributionsDoc(in ContributionsInput) string {
	prs := append([]model.PR(nil), in.PRs...)
	sort.Slice(prs, func(i, j int) bool { return prs[i].ClosedAt.After(prs[j].ClosedAt) })

	var b strings.Builder
	b.WriteString("# Contributions\n\n")
	b.WriteString("Merged pull requests to third-party public repositories.\n\n")
	b.WriteString(fmt.Sprintf("_Last updated: %s_\n\n", in.GeneratedTime.Format(time.RFC3339)))

	if len(prs) == 0 {
		b.WriteString("(no merged PRs found)\n")
		return b.String()
	}

	for _, pr := range prs {
		date := pr.ClosedAt.Format("2006-01-02")
		repoPR := fmt.Sprintf("%s#%d", pr.Repo.NameWithOwner, pr.Number)
		b.WriteString(fmt.Sprintf("- %s — [%s](%s) — %s\n", date, repoPR, pr.URL, escapeInline(pr.Title)))
	}

	return b.String()
}

func escapeInline(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}
