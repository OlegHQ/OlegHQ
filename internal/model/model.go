package model

import "time"

type RepoInfo struct {
	NameWithOwner string    `json:"nameWithOwner"`
	URL           string    `json:"url"`
	Description   string    `json:"description"`
	UpdatedAt     time.Time `json:"updatedAt"`
	PushedAt      time.Time `json:"pushedAt"`
	IsFork        bool      `json:"isFork"`
	IsPrivate     bool      `json:"isPrivate"`
}

type PR struct {
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	Number   int       `json:"number"`
	ClosedAt time.Time `json:"closedAt"`
	Repo     RepoInfo  `json:"-"`
}
