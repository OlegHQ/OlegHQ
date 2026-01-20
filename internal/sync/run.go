package sync

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"oleghq-readme-sync/internal/github"
	"oleghq-readme-sync/internal/render"
)

const (
	openSourceStart = "<!-- DYNAMIC:OPEN_SOURCE:START -->"
	openSourceEnd   = "<!-- DYNAMIC:OPEN_SOURCE:END -->"
)

func normalizeRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}
	if strings.HasPrefix(repo, "http://") || strings.HasPrefix(repo, "https://") {
		u, err := url.Parse(repo)
		if err != nil {
			return "", err
		}
		path := strings.Trim(u.Path, "/")
		if path == "" {
			return "", fmt.Errorf("invalid repo URL: %s", repo)
		}
		return path, nil
	}
	if strings.HasPrefix(repo, "github.com/") {
		return strings.TrimPrefix(repo, "github.com/"), nil
	}
	return repo, nil
}

func parseOwnerRepo(nwo string) (owner, repo string, err error) {
	parts := strings.Split(nwo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format: %s (expected owner/repo)", nwo)
	}
	return parts[0], parts[1], nil
}

func Run(ctx context.Context, gh *github.Client, cfg Config) error {
	normalized, err := normalizeRepo(cfg.Repo)
	if err != nil {
		return err
	}
	cfg.Repo = normalized

	owner, repo, err := parseOwnerRepo(cfg.Repo)
	if err != nil {
		return err
	}

	if cfg.Branch == "" {
		_, defaultBranch, err := resolveRepoOwnerAndDefaultBranch(ctx, gh, owner, repo)
		if err != nil {
			return err
		}
		cfg.Branch = defaultBranch
	}
	if cfg.ReadmePath == "" {
		cfg.ReadmePath = "README.md"
	}
	if cfg.FullListPath == "" {
		cfg.FullListPath = "CONTRIBUTIONS.md"
	}

	repoOwner, _, err := resolveRepoOwnerAndDefaultBranch(ctx, gh, owner, repo)
	if err != nil {
		return err
	}
	if cfg.ProjectsOwner == "" {
		cfg.ProjectsOwner = repoOwner
	}
	if cfg.ActorLogin == "" {
		cfg.ActorLogin = repoOwner
	}
	if len(cfg.SkipOwners) == 0 {
		cfg.SkipOwners = []string{repoOwner}
	}
	if cfg.PRFetchLimit <= 0 {
		cfg.PRFetchLimit = 200
	}
	if cfg.TopContribs <= 0 {
		cfg.TopContribs = 10
	}
	if cfg.ProjectsToShow <= 0 {
		cfg.ProjectsToShow = 5
	}

	prs, err := fetchMergedPRs(ctx, gh, cfg)
	if err != nil {
		return err
	}
	projects, err := fetchProjects(ctx, gh, cfg)
	if err != nil {
		return err
	}

	sort.Slice(prs, func(i, j int) bool { return prs[i].ClosedAt.After(prs[j].ClosedAt) })
	if len(prs) > cfg.TopContribs {
		prs = prs[:cfg.TopContribs]
	}

	readmeBlock := render.OpenSourceBlock(render.OpenSourceInput{
		TopPRs:        prs,
		Projects:      projects,
		FullListPath:  cfg.FullListPath,
		GeneratedTime: time.Now().UTC(),
	})

	// Re-fetch PRs without truncation for the full list.
	fullPRs, err := fetchMergedPRs(ctx, gh, cfg)
	if err != nil {
		return err
	}
	contribsMD := render.ContributionsDoc(render.ContributionsInput{
		PRs:           fullPRs,
		GeneratedTime: time.Now().UTC(),
	})

	fc, err := gh.GetContent(ctx, owner, repo, cfg.ReadmePath, cfg.Branch)
	if err != nil {
		return fmt.Errorf("get README: %w", err)
	}
	readme, readmeSHA := fc.Content, fc.SHA

	newReadme, err := upsertOpenSourceBlock(readme, readmeBlock)
	if err != nil {
		return err
	}

	if cfg.DryRun {
		fmt.Println("--- README.md (preview) ---")
		fmt.Println(newReadme)
		fmt.Println("--- CONTRIBUTIONS.md (preview) ---")
		fmt.Println(contribsMD)
		return nil
	}

	if err := gh.PutContent(ctx, owner, repo, cfg.ReadmePath, cfg.Branch, readmeSHA, []byte(newReadme), "chore: update README dynamic open source"); err != nil {
		return fmt.Errorf("update README: %w", err)
	}

	contribsSHA, _ := gh.GetContentSHA(ctx, owner, repo, cfg.FullListPath, cfg.Branch) // ok if missing
	if err := gh.PutContent(ctx, owner, repo, cfg.FullListPath, cfg.Branch, contribsSHA, []byte(contribsMD), "chore: update contributions history"); err != nil {
		return fmt.Errorf("update contributions: %w", err)
	}

	return nil
}

func upsertOpenSourceBlock(readme, block string) (string, error) {
	if strings.Contains(readme, openSourceStart) && strings.Contains(readme, openSourceEnd) {
		before, after, ok := strings.Cut(readme, openSourceStart)
		if !ok {
			return "", errors.New("failed to locate start marker")
		}
		_, after, ok = strings.Cut(after, openSourceEnd)
		if !ok {
			return "", errors.New("failed to locate end marker")
		}
		return before + block + after, nil
	}

	header := "### 03 — Open Source"
	idx := strings.Index(readme, header)
	if idx == -1 {
		return "", fmt.Errorf("README missing header %q", header)
	}

	slice := readme[idx+len(header):]
	next := strings.Index(slice, "---")
	if next == -1 {
		return "", errors.New("README missing section divider after open source")
	}

	return readme[:idx+len(header)] + "\n\n" + block + "\n" + slice[next:], nil
}
