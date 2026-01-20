package sync

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"oleghq-readme-sync/internal/github"
	"oleghq-readme-sync/internal/model"
)

func resolveRepoOwnerAndDefaultBranch(ctx context.Context, gh *github.Client, owner, repo string) (string, string, error) {
	info, err := gh.GetRepo(ctx, owner, repo)
	if err != nil {
		return "", "", err
	}
	return info.Owner, info.DefaultBranch, nil
}

func fetchMergedPRs(ctx context.Context, gh *github.Client, cfg Config) ([]model.PR, error) {
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
	repoMetaCache := map[string]*github.RepoInfo{}

	for _, scope := range scopes {
		var qualifier string
		switch scope {
		case PRScopeAuthored:
			qualifier = "author"
		case PRScopeInvolved:
			qualifier = "involves"
		default:
			return nil, fmt.Errorf("unknown prs-scope: %s", cfg.PRsScope)
		}

		prs, err := gh.SearchMergedPRs(ctx, qualifier, actor, cfg.PRFetchLimit)
		if err != nil {
			return nil, err
		}

		for _, pr := range prs {
			repoNWO := pr.RepoNWO
			if repoNWO == "" {
				continue
			}
			key := repoNWO + "#" + fmt.Sprintf("%d", pr.Number)
			if seen[key] {
				continue
			}

			parts := strings.Split(repoNWO, "/")
			if len(parts) != 2 {
				continue
			}
			repoOwner, repoName := parts[0], parts[1]

			if contains(cfg.SkipOwners, repoOwner) {
				continue
			}

			repoInfo, ok := repoMetaCache[repoNWO]
			if !ok {
				info, err := gh.GetRepo(ctx, repoOwner, repoName)
				if err != nil {
					// If the repo disappeared or is inaccessible, skip it.
					continue
				}
				repoInfo = info
				repoMetaCache[repoNWO] = repoInfo
			}

			if repoInfo.Private || repoInfo.Fork {
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
					NameWithOwner: repoInfo.NameWithOwner,
					URL:           repoInfo.URL,
					Description:   repoInfo.Description,
				},
			})
		}
	}

	sort.Slice(outPRs, func(i, j int) bool { return outPRs[i].ClosedAt.After(outPRs[j].ClosedAt) })
	return outPRs, nil
}

func fetchProjects(ctx context.Context, gh *github.Client, cfg Config) ([]model.RepoInfo, error) {
	owner := cfg.ProjectsOwner
	if owner == "" {
		return nil, fmt.Errorf("projects owner is required")
	}

	limit := 50
	if cfg.ProjectsToShow > limit {
		limit = cfg.ProjectsToShow
	}

	repos, err := gh.ListUserRepos(ctx, owner, limit)
	if err != nil {
		return nil, err
	}

	out := make([]model.RepoInfo, 0, len(repos))
	for _, r := range repos {
		if r.Private || r.Fork {
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
			IsFork:        r.Fork,
			IsPrivate:     r.Private,
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
