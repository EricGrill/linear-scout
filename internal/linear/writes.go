package linear

import (
	"context"
	"fmt"
)

// This file holds the ONLY mutating operations in the client. They are kept
// separate from client.go to keep the read-only surface unambiguous. There are
// deliberately no methods to close, reprioritize existing issues, or delete
// anything — those are disallowed by the safety model.

// CreateIssueInput is the metadata for a new Linear issue.
type CreateIssueInput struct {
	Title       string
	Description string
	TeamID      string
	Priority    int
}

// CreatedIssue identifies a newly created issue.
type CreatedIssue struct {
	ID         string
	Identifier string
	URL        string
}

const issueCreateMutation = `
mutation IssueCreate($title: String!, $description: String, $teamId: String!, $priority: Int) {
  issueCreate(input: { title: $title, description: $description, teamId: $teamId, priority: $priority }) {
    success
    issue { id identifier url }
  }
}`

// CreateIssue creates a new Linear issue.
func (c *Client) CreateIssue(ctx context.Context, in CreateIssueInput) (CreatedIssue, error) {
	if in.Title == "" || in.TeamID == "" {
		return CreatedIssue{}, fmt.Errorf("create issue: title and teamId are required")
	}
	var resp struct {
		IssueCreate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
				URL        string `json:"url"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}
	vars := map[string]any{
		"title": in.Title, "description": in.Description,
		"teamId": in.TeamID, "priority": in.Priority,
	}
	if err := c.run(ctx, issueCreateMutation, vars, &resp); err != nil {
		return CreatedIssue{}, err
	}
	if !resp.IssueCreate.Success {
		return CreatedIssue{}, fmt.Errorf("create issue: Linear reported failure")
	}
	return CreatedIssue{
		ID:         resp.IssueCreate.Issue.ID,
		Identifier: resp.IssueCreate.Issue.Identifier,
		URL:        resp.IssueCreate.Issue.URL,
	}, nil
}

// CreatedComment identifies a newly created comment.
type CreatedComment struct {
	ID  string
	URL string
}

const commentCreateMutation = `
mutation CommentCreate($issueId: String!, $body: String!) {
  commentCreate(input: { issueId: $issueId, body: $body }) {
    success
    comment { id url }
  }
}`

// CreateComment adds a comment to an existing issue.
func (c *Client) CreateComment(ctx context.Context, issueID, body string) (CreatedComment, error) {
	if issueID == "" || body == "" {
		return CreatedComment{}, fmt.Errorf("create comment: issueId and body are required")
	}
	var resp struct {
		CommentCreate struct {
			Success bool `json:"success"`
			Comment struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			} `json:"comment"`
		} `json:"commentCreate"`
	}
	vars := map[string]any{"issueId": issueID, "body": body}
	if err := c.run(ctx, commentCreateMutation, vars, &resp); err != nil {
		return CreatedComment{}, err
	}
	if !resp.CommentCreate.Success {
		return CreatedComment{}, fmt.Errorf("create comment: Linear reported failure")
	}
	return CreatedComment{ID: resp.CommentCreate.Comment.ID, URL: resp.CommentCreate.Comment.URL}, nil
}

const issueLabelContextQuery = `
query IssueLabelContext($id: String!) {
  issue(id: $id) {
    labels { nodes { id name } }
    team { labels { nodes { id name } } }
  }
}`

const issueUpdateLabelsMutation = `
mutation IssueSetLabels($id: String!, $labelIds: [String!]!) {
  issueUpdate(id: $id, input: { labelIds: $labelIds }) { success }
}`

// AddLabels adds the named labels to an issue, preserving existing labels.
// Label names must already exist on the issue's team; unknown names error.
func (c *Client) AddLabels(ctx context.Context, issueID string, labelNames []string) error {
	if issueID == "" || len(labelNames) == 0 {
		return fmt.Errorf("add labels: issueId and at least one label are required")
	}
	var ctxResp struct {
		Issue struct {
			Labels struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"labels"`
			Team struct {
				Labels struct {
					Nodes []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
			} `json:"team"`
		} `json:"issue"`
	}
	if err := c.run(ctx, issueLabelContextQuery, map[string]any{"id": issueID}, &ctxResp); err != nil {
		return err
	}

	nameToID := map[string]string{}
	for _, l := range ctxResp.Issue.Team.Labels.Nodes {
		nameToID[l.Name] = l.ID
	}
	// Start from existing label ids to preserve them (add, not replace).
	idSet := map[string]struct{}{}
	for _, l := range ctxResp.Issue.Labels.Nodes {
		idSet[l.ID] = struct{}{}
	}
	for _, name := range labelNames {
		id, ok := nameToID[name]
		if !ok {
			return fmt.Errorf("add labels: label %q not found on issue's team", name)
		}
		idSet[id] = struct{}{}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	var updResp struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}
	if err := c.run(ctx, issueUpdateLabelsMutation, map[string]any{"id": issueID, "labelIds": ids}, &updResp); err != nil {
		return err
	}
	if !updResp.IssueUpdate.Success {
		return fmt.Errorf("add labels: Linear reported failure")
	}
	return nil
}
