package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://api.github.com"

type Client struct {
	token      string
	httpClient *http.Client
}

type Options struct {
	Token   string
	Timeout time.Duration
}

func New(opts Options) (*Client, error) {
	if opts.Token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		token: opts.Token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *Client) do(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	u := baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBody, &errResp)
		msg := errResp.Message
		if msg == "" {
			msg = string(respBody)
		}
		return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, msg)
	}

	return respBody, nil
}

// Check verifies the token is valid by fetching the authenticated user.
func (c *Client) Check(ctx context.Context) error {
	b, err := c.do(ctx, "GET", "/user", nil)
	if err != nil {
		return fmt.Errorf("auth check failed: %w", err)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(b, &user); err != nil {
		return fmt.Errorf("parse user response: %w", err)
	}
	if user.Login == "" {
		return fmt.Errorf("auth check failed: empty user")
	}
	return nil
}

type RepoInfo struct {
	Owner             string `json:"owner"`
	Name              string `json:"name"`
	NameWithOwner     string `json:"full_name"`
	URL               string `json:"html_url"`
	Description       string `json:"description"`
	DefaultBranch     string `json:"default_branch"`
	UpdatedAt         string `json:"updated_at"`
	PushedAt          string `json:"pushed_at"`
	Fork              bool   `json:"fork"`
	Private           bool   `json:"private"`
}

// GetRepo fetches repository information.
func (c *Client) GetRepo(ctx context.Context, owner, repo string) (*RepoInfo, error) {
	b, err := c.do(ctx, "GET", fmt.Sprintf("/repos/%s/%s", owner, repo), nil)
	if err != nil {
		return nil, err
	}
	var r struct {
		FullName      string `json:"full_name"`
		HTMLURL       string `json:"html_url"`
		Description   string `json:"description"`
		DefaultBranch string `json:"default_branch"`
		UpdatedAt     string `json:"updated_at"`
		PushedAt      string `json:"pushed_at"`
		Fork          bool   `json:"fork"`
		Private       bool   `json:"private"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse repo response: %w", err)
	}
	return &RepoInfo{
		Owner:         r.Owner.Login,
		Name:          repo,
		NameWithOwner: r.FullName,
		URL:           r.HTMLURL,
		Description:   r.Description,
		DefaultBranch: r.DefaultBranch,
		UpdatedAt:     r.UpdatedAt,
		PushedAt:      r.PushedAt,
		Fork:          r.Fork,
		Private:       r.Private,
	}, nil
}

type SearchPR struct {
	Title    string
	URL      string
	Number   int
	ClosedAt string
	RepoNWO  string // owner/repo
}

// SearchMergedPRs searches for merged PRs using the given qualifier (author or involves).
func (c *Client) SearchMergedPRs(ctx context.Context, qualifier, actor string, limit int) ([]SearchPR, error) {
	// Build query: is:pr is:merged is:public author:X or involves:X
	q := fmt.Sprintf("is:pr is:merged is:public %s:%s", qualifier, actor)

	var allPRs []SearchPR
	perPage := 100
	if limit < perPage {
		perPage = limit
	}

	for page := 1; len(allPRs) < limit; page++ {
		endpoint := fmt.Sprintf("/search/issues?q=%s&per_page=%d&page=%d&sort=updated&order=desc",
			url.QueryEscape(q), perPage, page)

		b, err := c.do(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("search PRs: %w", err)
		}

		var resp struct {
			TotalCount int `json:"total_count"`
			Items      []struct {
				Title     string `json:"title"`
				HTMLURL   string `json:"html_url"`
				Number    int    `json:"number"`
				ClosedAt  string `json:"closed_at"`
				RepoURL   string `json:"repository_url"`
			} `json:"items"`
		}
		if err := json.Unmarshal(b, &resp); err != nil {
			return nil, fmt.Errorf("parse search response: %w", err)
		}

		for _, item := range resp.Items {
			// Extract owner/repo from repository_url: https://api.github.com/repos/owner/repo
			repoNWO := strings.TrimPrefix(item.RepoURL, "https://api.github.com/repos/")
			allPRs = append(allPRs, SearchPR{
				Title:    item.Title,
				URL:      item.HTMLURL,
				Number:   item.Number,
				ClosedAt: item.ClosedAt,
				RepoNWO:  repoNWO,
			})
			if len(allPRs) >= limit {
				break
			}
		}

		if len(resp.Items) < perPage || len(allPRs) >= limit {
			break
		}
	}

	return allPRs, nil
}

type ListedRepo struct {
	NameWithOwner string
	URL           string
	Description   string
	UpdatedAt     string
	PushedAt      string
	Fork          bool
	Private       bool
}

// ListUserRepos lists repositories for a user (source repos only, not forks).
func (c *Client) ListUserRepos(ctx context.Context, owner string, limit int) ([]ListedRepo, error) {
	var allRepos []ListedRepo
	perPage := 100
	if limit < perPage {
		perPage = limit
	}

	for page := 1; len(allRepos) < limit; page++ {
		endpoint := fmt.Sprintf("/users/%s/repos?type=owner&sort=pushed&direction=desc&per_page=%d&page=%d",
			owner, perPage, page)

		b, err := c.do(ctx, "GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("list repos: %w", err)
		}

		var repos []struct {
			FullName    string `json:"full_name"`
			HTMLURL     string `json:"html_url"`
			Description string `json:"description"`
			UpdatedAt   string `json:"updated_at"`
			PushedAt    string `json:"pushed_at"`
			Fork        bool   `json:"fork"`
			Private     bool   `json:"private"`
		}
		if err := json.Unmarshal(b, &repos); err != nil {
			return nil, fmt.Errorf("parse repos response: %w", err)
		}

		for _, r := range repos {
			allRepos = append(allRepos, ListedRepo{
				NameWithOwner: r.FullName,
				URL:           r.HTMLURL,
				Description:   r.Description,
				UpdatedAt:     r.UpdatedAt,
				PushedAt:      r.PushedAt,
				Fork:          r.Fork,
				Private:       r.Private,
			})
			if len(allRepos) >= limit {
				break
			}
		}

		if len(repos) < perPage || len(allRepos) >= limit {
			break
		}
	}

	return allRepos, nil
}

type FileContent struct {
	Content string
	SHA     string
}

// GetContent fetches file content from a repository.
func (c *Client) GetContent(ctx context.Context, owner, repo, path, ref string) (*FileContent, error) {
	endpoint := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, ref)
	b, err := c.do(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SHA      string `json:"sha"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("parse content response: %w", err)
	}

	if resp.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", resp.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}

	return &FileContent{
		Content: string(decoded),
		SHA:     resp.SHA,
	}, nil
}

// GetContentSHA fetches just the SHA of a file (useful for checking if file exists).
func (c *Client) GetContentSHA(ctx context.Context, owner, repo, path, ref string) (string, error) {
	fc, err := c.GetContent(ctx, owner, repo, path, ref)
	if err != nil {
		return "", err
	}
	return fc.SHA, nil
}

// PutContent creates or updates a file in a repository.
func (c *Client) PutContent(ctx context.Context, owner, repo, path, branch, sha string, content []byte, message string) error {
	endpoint := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)

	body := map[string]string{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  branch,
	}
	if sha != "" {
		body["sha"] = sha
	}

	_, err := c.do(ctx, "PUT", endpoint, body)
	return err
}
