package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"oleghq-readme-sync/internal/github"
	"oleghq-readme-sync/internal/sync"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		repo      = flag.String("repo", "github.com/oleghq/oleghq", "target repo (owner/name or GitHub URL)")
		branch    = flag.String("branch", "", "target branch (default: repo default)")
		readme    = flag.String("readme", "README.md", "README path")
		fullList  = flag.String("full-list", "CONTRIBUTIONS.md", "full contributions history path")
		dryRun    = flag.Bool("dry-run", false, "print changes without pushing")
		prLimit   = flag.Int("pr-limit", 200, "max PRs fetched before filtering")
		topN      = flag.Int("top", 10, "top latest merged contributions")
		projectsN = flag.Int("projects", 5, "recent projects to show")
		prsScope  = flag.String("prs-scope", "both", "PR scope: authored, involved, or both")
		user      = flag.String("user", "", "GitHub login to analyze (default: repo owner)")
	)
	flag.Parse()

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "GITHUB_TOKEN environment variable is required")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := github.New(github.Options{
		Token:   token,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if err := client.Check(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	cfg := sync.Config{
		Repo:         *repo,
		Branch:       *branch,
		ReadmePath:   *readme,
		FullListPath: *fullList,
		DryRun:       *dryRun,

		PRFetchLimit:   *prLimit,
		TopContribs:    *topN,
		ProjectsToShow: *projectsN,

		SkipOwners:    nil, // filled from repo owner
		ProjectsOwner: "",  // filled from repo owner
		PRsScope:      sync.PRScope(*prsScope),
		ActorLogin:    *user,
	}

	if err := sync.Run(ctx, client, cfg); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintln(os.Stderr, "timed out")
			return 1
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if *dryRun {
		fmt.Println("dry-run complete")
		return 0
	}

	fmt.Println("update complete")
	return 0
}
