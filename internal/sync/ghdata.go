package sync

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"oleghq-readme-sync/internal/ghcli"
	"oleghq-readme-sync/internal/model"
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

func resolveRepoOwnerAndDefaultBranch(ctx context.Context, gh *ghcli.Client, repo string) (owner, defaultBranch string, err error) {
	out, err := gh.Run(ctx, "repo", "view", repo, "--json", "owner,defaultBranchRef", "--jq", ".owner.login + \"\\n\" + .defaultBranchRef.name")
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected repo view output")
	}
	return parts[0], parts[1], nil
}

type ghSearchPR struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	Number     int    `json:"number"`
	ClosedAt   string `json:"closedAt"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

type ghRepoView struct {
	NameWithOwner string `json:"nameWithOwner"`
	URL           string `json:"url"`
	Description   string `json:"description"`
	UpdatedAt     string `json:"updatedAt"`
	PushedAt      string `json:"pushedAt"`
	IsFork        bool   `json:"isFork"`
	IsPrivate     bool   `json:"isPrivate"`
}

func fetchMergedPRs(ctx context.Context, gh *ghcli.Client, cfg Config) ([]model.PR, error) {
	actor := cfg.ActorLogin
	if actor == "" {
		actor = cfg.ProjectsOwner
	}
	if actor == "" {
		return nil, fmt.Errorf("actor login is required")
	}

	scopes := []PRScope{cfg.PRsScope}
	if cfg.PRsScope == PRScopeBoth {
		scopes = []PRScope{PRScopeAuthored, PRScopeInvolved}
	}

	seen := map[string]bool{}
	var outPRs []model.PR

	for _, scope := range scopes {
		args := []string{"search", "prs", "--merged", "--visibility", "public", "--limit", fmt.Sprintf("%d", cfg.PRFetchLimit), "--json", "title,url,closedAt,number,repository"}
		switch scope {
		case PRScopeAuthored:
			args = append(args, "--author", actor)
		case PRScopeInvolved:
			args = append(args, "--involves", actor)
		default:
			return nil, fmt.Errorf("unknown prs-scope: %s", cfg.PRsScope)
		}
		// Allow raw query qualifiers if needed later.

		b, err := gh.Run(ctx, args...)
		if err != nil {
			return nil, err
		}
		var prs []ghSearchPR
		if err := ghcli.JSON(b, &prs); err != nil {
			return nil, err
		}

		repoMetaCache := map[string]ghRepoView{}
		for _, pr := range prs {
			repoNWO := pr.Repository.NameWithOwner
			if repoNWO == "" {
				continue
			}
			key := repoNWO + "#" + fmt.Sprintf("%d", pr.Number)
			if seen[key] {
				continue
			}

			if owner := strings.Split(repoNWO, "/")[0]; contains(cfg.SkipOwners, owner) {
				continue
			}

			repoView, ok := repoMetaCache[repoNWO]
			if !ok {
				raw, err := gh.Run(ctx, "repo", "view", repoNWO, "--json", "nameWithOwner,url,description,updatedAt,pushedAt,isFork,isPrivate")
				if err != nil {
					// If the repo disappeared or is inaccessible, skip it.
					continue
				}
				if err := ghcli.JSON(raw, &repoView); err != nil {
					continue
				}
				repoMetaCache[repoNWO] = repoView
			}

			if repoView.IsPrivate || repoView.IsFork {
				continue
			}

			closedAt, err := time.Parse(time.RFC3339, pr.ClosedAt)
			if err != nil {
				continue
			}

			seen[key] = true
			outPRs = append(outPRs, model.PR{
				Title:    pr.Title,
				URL:      pr.URL,
				Number:   pr.Number,
				ClosedAt: closedAt,
				Repo: model.RepoInfo{
					NameWithOwner: repoView.NameWithOwner,
					URL:           repoView.URL,
					Description:   repoView.Description,
				},
			})
		}
	}

	sort.Slice(outPRs, func(i, j int) bool { return outPRs[i].ClosedAt.After(outPRs[j].ClosedAt) })
	return outPRs, nil
}

func fetchProjects(ctx context.Context, gh *ghcli.Client, cfg Config) ([]model.RepoInfo, error) {
	owner := cfg.ProjectsOwner
	if owner == "" {
		return nil, fmt.Errorf("projects owner is required")
	}

	limit := 50
	if cfg.ProjectsToShow > limit {
		limit = cfg.ProjectsToShow
	}

	b, err := gh.Run(ctx, "repo", "list", owner, "--source", "--limit", fmt.Sprintf("%d", limit), "--json", "nameWithOwner,description,updatedAt,pushedAt,url,isFork,isPrivate")
	if err != nil {
		return nil, err
	}
	var repos []ghRepoView
	if err := ghcli.JSON(b, &repos); err != nil {
		return nil, err
	}

	out := make([]model.RepoInfo, 0, len(repos))
	for _, r := range repos {
		if r.IsPrivate || r.IsFork {
			continue
		}
		updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)
		pushedAt, _ := time.Parse(time.RFC3339, r.PushedAt)
		out = append(out, model.RepoInfo{
			NameWithOwner: r.NameWithOwner,
			URL:           r.URL,
			Description:   r.Description,
			UpdatedAt:     updatedAt,
			PushedAt:      pushedAt,
			IsFork:        r.IsFork,
			IsPrivate:     r.IsPrivate,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		ai, aj := out[i].PushedAt, out[j].PushedAt
		if ai.IsZero() || aj.IsZero() {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return ai.After(aj)
	})

	if len(out) > cfg.ProjectsToShow {
		out = out[:cfg.ProjectsToShow]
	}
	return out, nil
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
