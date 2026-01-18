package sync

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"oleghq-readme-sync/internal/ghcli"
	"oleghq-readme-sync/internal/render"
)

const (
	openSourceStart = "<!-- DYNAMIC:OPEN_SOURCE:START -->"
	openSourceEnd   = "<!-- DYNAMIC:OPEN_SOURCE:END -->"
)

func Run(ctx context.Context, gh *ghcli.Client, cfg Config) error {
	normalized, err := normalizeRepo(cfg.Repo)
	if err != nil {
		return err
	}
	cfg.Repo = normalized
	if cfg.Branch == "" {
		_, defaultBranch, err := resolveRepoOwnerAndDefaultBranch(ctx, gh, cfg.Repo)
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
	owner, _, err := resolveRepoOwnerAndDefaultBranch(ctx, gh, cfg.Repo)
	if err != nil {
		return err
	}
	if cfg.ProjectsOwner == "" {
		cfg.ProjectsOwner = owner
	}
	if cfg.ActorLogin == "" {
		cfg.ActorLogin = owner
	}
	if len(cfg.SkipOwners) == 0 {
		cfg.SkipOwners = []string{owner}
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

	readme, readmeSHA, err := getContent(ctx, gh, cfg.Repo, cfg.ReadmePath, cfg.Branch)
	if err != nil {
		return err
	}
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

	if err := putContent(ctx, gh, cfg.Repo, cfg.ReadmePath, cfg.Branch, readmeSHA, []byte(newReadme), "chore: update README dynamic open source"); err != nil {
		return err
	}

	contribsSHA, _ := getContentSHA(ctx, gh, cfg.Repo, cfg.FullListPath, cfg.Branch) // ok if missing
	if err := putContent(ctx, gh, cfg.Repo, cfg.FullListPath, cfg.Branch, contribsSHA, []byte(contribsMD), "chore: update contributions history"); err != nil {
		return err
	}

	return nil
}

func getContent(ctx context.Context, gh *ghcli.Client, repo, path, branch string) (string, string, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, path, branch)
	out, err := gh.Run(ctx, "api", endpoint)
	if err != nil {
		return "", "", err
	}
	var resp struct {
		SHA      string `json:"sha"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := ghcli.JSON(out, &resp); err != nil {
		return "", "", err
	}
	if resp.Encoding != "base64" {
		return "", "", fmt.Errorf("unexpected encoding for %s: %s", path, resp.Encoding)
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	if err != nil {
		return "", "", err
	}
	return string(b), resp.SHA, nil
}

func getContentSHA(ctx context.Context, gh *ghcli.Client, repo, path, branch string) (string, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, path, branch)
	out, err := gh.Run(ctx, "api", endpoint)
	if err != nil {
		return "", err
	}
	var resp struct {
		SHA string `json:"sha"`
	}
	if err := ghcli.JSON(out, &resp); err != nil {
		return "", err
	}
	return resp.SHA, nil
}

func putContent(ctx context.Context, gh *ghcli.Client, repo, path, branch, sha string, content []byte, message string) error {
	encoded := base64.StdEncoding.EncodeToString(content)

	args := []string{"api", "-X", "PUT", fmt.Sprintf("repos/%s/contents/%s", repo, path), "-f", "branch=" + branch, "-f", "message=" + message, "-f", "content=" + encoded}
	if sha != "" {
		args = append(args, "-f", "sha="+sha)
	}
	_, err := gh.Run(ctx, args...)
	return err
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
