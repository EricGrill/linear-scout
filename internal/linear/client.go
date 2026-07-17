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

// pageSize is the per-request node count; pagination fetches all pages.
const pageSize = 100

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

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
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
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	Team struct {
		ID   string `json:"id"`
		Name string `json:"name"`
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
query Issues($since: DateTimeOrDuration, $after: String) {
  issues(filter: { updatedAt: { gte: $since } }, first: 100, after: $after) {
    nodes {
      id identifier title description priority url createdAt updatedAt
      state { name } assignee { name } project { id name } team { id name }
      labels { nodes { name } }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// Issues fetches all issues updated since the given time, following pagination.
func (c *Client) Issues(ctx context.Context, since time.Time) ([]model.Issue, error) {
	var out []model.Issue
	after := ""
	for {
		var resp struct {
			Issues struct {
				Nodes    []issueNode `json:"nodes"`
				PageInfo pageInfo    `json:"pageInfo"`
			} `json:"issues"`
		}
		vars := map[string]any{"since": since.Format(time.RFC3339)}
		if after != "" {
			vars["after"] = after
		}
		if err := c.run(ctx, issuesQuery, vars, &resp); err != nil {
			return nil, err
		}
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
		if !resp.Issues.PageInfo.HasNextPage {
			break
		}
		after = resp.Issues.PageInfo.EndCursor
	}
	return out, nil
}

// ProjectNames returns id->name for projects referenced in the fetched window.
func (c *Client) ProjectNames(ctx context.Context) (map[string]string, error) {
	return c.namedEntities(ctx, projectsQuery, "projects")
}

// TeamNames returns id->name for all teams.
func (c *Client) TeamNames(ctx context.Context) (map[string]string, error) {
	return c.namedEntities(ctx, teamsQuery, "teams")
}

const projectsQuery = `
query Projects($after: String) {
  projects(first: 100, after: $after) {
    nodes { id name }
    pageInfo { hasNextPage endCursor }
  }
}`

const teamsQuery = `
query Teams($after: String) {
  teams(first: 100, after: $after) {
    nodes { id name }
    pageInfo { hasNextPage endCursor }
  }
}`

// namedEntities paginates a query whose connection field yields {id,name} nodes.
func (c *Client) namedEntities(ctx context.Context, query, field string) (map[string]string, error) {
	out := map[string]string{}
	after := ""
	for {
		var resp map[string]struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
			PageInfo pageInfo `json:"pageInfo"`
		}
		vars := map[string]any{}
		if after != "" {
			vars["after"] = after
		}
		if err := c.run(ctx, query, vars, &resp); err != nil {
			return nil, err
		}
		conn := resp[field]
		for _, n := range conn.Nodes {
			out[n.ID] = n.Name
		}
		if !conn.PageInfo.HasNextPage {
			break
		}
		after = conn.PageInfo.EndCursor
	}
	return out, nil
}

const commentsQuery = `
query Comments($since: DateTimeOrDuration, $after: String) {
  comments(filter: { createdAt: { gte: $since } }, first: 100, after: $after) {
    nodes { id body url createdAt user { name } issue { id } }
    pageInfo { hasNextPage endCursor }
  }
}`

// Comments fetches all comments created since the given time, following pagination.
func (c *Client) Comments(ctx context.Context, since time.Time) ([]model.Comment, error) {
	var out []model.Comment
	after := ""
	for {
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
				PageInfo pageInfo `json:"pageInfo"`
			} `json:"comments"`
		}
		vars := map[string]any{"since": since.Format(time.RFC3339)}
		if after != "" {
			vars["after"] = after
		}
		if err := c.run(ctx, commentsQuery, vars, &resp); err != nil {
			return nil, err
		}
		for _, n := range resp.Comments.Nodes {
			out = append(out, model.Comment{
				ID: n.ID, IssueID: n.Issue.ID, Author: n.User.Name,
				Body: n.Body, URL: n.URL, CreatedAt: n.CreatedAt,
			})
		}
		if !resp.Comments.PageInfo.HasNextPage {
			break
		}
		after = resp.Comments.PageInfo.EndCursor
	}
	return out, nil
}
