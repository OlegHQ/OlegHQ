package ghcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Options struct {
	Timeout time.Duration
}

type Client struct {
	timeout time.Duration
}

func New(opts Options) *Client {
	t := opts.Timeout
	if t <= 0 {
		t = 30 * time.Second
	}
	return &Client{timeout: t}
}

func (c *Client) Check(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found in PATH")
	}

	// Avoid failing just because there are stale secondary accounts.
	cmd := exec.CommandContext(ctx, "gh", "api", "user", "--jq", ".login")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("gh auth check failed: %s", msg)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("gh auth check failed: empty user")
	}
	return nil
}

func (c *Client) Run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func JSON[T any](b []byte, v *T) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(v); err != nil {
		var syntaxError *json.SyntaxError
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("invalid JSON: %w", err)
		default:
			return err
		}
	}
	return nil
}
