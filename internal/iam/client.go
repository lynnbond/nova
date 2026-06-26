// Package iam provides a client for IAM Server authentication.
package iam

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// User represents a user returned by IAM /users/self.
type User struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	RealName string `json:"realName"`
	Email    string `json:"email"`
	OrgName  string `json:"orgName"`
	Status   string `json:"status"`
}

// Client is an HTTP client for IAM Server.
type Client struct {
	BaseURL    string
	httpClient *http.Client
}

// New creates a new IAM client.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetUserByToken validates a CSRF Token against IAM and returns the user.
// Calls GET /iam/api/v2/users/self with Csrf-Token header.
func (c *Client) GetUserByToken(ctx context.Context, csrfToken string) (*User, error) {
	if csrfToken == "" {
		return nil, fmt.Errorf("csrf token is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/iam/api/v2/users/self", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Csrf-Token", csrfToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call iam: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("iam auth failed: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}

	if user.UserID == "" {
		return nil, fmt.Errorf("iam returned empty userId")
	}

	return &user, nil
}
