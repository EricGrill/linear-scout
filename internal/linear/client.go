// Package linear is a read-only Linear GraphQL client. It issues queries only.
package linear

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/EricGrill/linear-scout/internal/model"
	"github.com/machinebox/graphql"
)

const DefaultEndpoint = "https://api.linear.app/graphql"

type Client struct {
	gql   *graphql.Client
	token string
}

func New(endpoint, token string, httpClient *http.Client) *Client {
	opts := []graphql.ClientOption{}
	if httpClient != nil {
		opts = append(opts, graphql.WithHTTPClient(httpClient))
	}
	return &Client{gql: graphql.NewClient(endpoint, opts...), token: token}
}

type issueNode struct {
	ID          string    `json:"id"`
	Identifier  string    `json:"identifier"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	State       struct {
		Name string `json:"name"`
	} `json:"state"`
	Assignee struct {
		Name string `json:"name"`
	} `json:"assignee"`
	Project struct {
		ID string `json:"id"`
	} `json:"project"`
	Team struct {
		ID string `json:"id"`
	} `json:"team"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
}

func (c *Client) run(ctx context.Context, query string, vars map[string]any, out any) error {
	req := graphql.NewRequest(query)
	req.Header.Set("Authorization", c.token)
	for k, v := range vars {
		req.Var(k, v)
	}
	if err := c.gql.Run(ctx, req, out); err != nil {
		return fmt.Errorf("linear query: %w", err)
	}
	return nil
}

const issuesQuery = `
query Issues($since: DateTimeOrDuration) {
  issues(filter: { updatedAt: { gte: $since } }, first: 100) {
    nodes {
      id identifier title description priority url createdAt updatedAt
      state { name } assignee { name } project { id } team { id }
      labels { nodes { name } }
    }
  }
}`

func (c *Client) Issues(ctx context.Context, since time.Time) ([]model.Issue, error) {
	var resp struct {
		Issues struct {
			Nodes []issueNode `json:"nodes"`
		} `json:"issues"`
	}
	if err := c.run(ctx, issuesQuery, map[string]any{"since": since.Format(time.RFC3339)}, &resp); err != nil {
		return nil, err
	}
	out := make([]model.Issue, 0, len(resp.Issues.Nodes))
	for _, n := range resp.Issues.Nodes {
		labels := make([]string, 0, len(n.Labels.Nodes))
		for _, l := range n.Labels.Nodes {
			labels = append(labels, l.Name)
		}
		out = append(out, model.Issue{
			ID: n.ID, Identifier: n.Identifier, Title: n.Title, Body: n.Description,
			State: n.State.Name, Priority: n.Priority, Labels: labels,
			Assignee: n.Assignee.Name, ProjectID: n.Project.ID, TeamID: n.Team.ID,
			URL: n.URL, CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
		})
	}
	return out, nil
}

const commentsQuery = `
query Comments($since: DateTimeOrDuration) {
  comments(filter: { createdAt: { gte: $since } }, first: 100) {
    nodes { id body url createdAt user { name } issue { id } }
  }
}`

func (c *Client) Comments(ctx context.Context, since time.Time) ([]model.Comment, error) {
	var resp struct {
		Comments struct {
			Nodes []struct {
				ID        string    `json:"id"`
				Body      string    `json:"body"`
				URL       string    `json:"url"`
				CreatedAt time.Time `json:"createdAt"`
				User      struct {
					Name string `json:"name"`
				} `json:"user"`
				Issue struct {
					ID string `json:"id"`
				} `json:"issue"`
			} `json:"nodes"`
		} `json:"comments"`
	}
	if err := c.run(ctx, commentsQuery, map[string]any{"since": since.Format(time.RFC3339)}, &resp); err != nil {
		return nil, err
	}
	out := make([]model.Comment, 0, len(resp.Comments.Nodes))
	for _, n := range resp.Comments.Nodes {
		out = append(out, model.Comment{
			ID: n.ID, IssueID: n.Issue.ID, Author: n.User.Name,
			Body: n.Body, URL: n.URL, CreatedAt: n.CreatedAt,
		})
	}
	return out, nil
}
