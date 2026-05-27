package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v69/github"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubIssueList struct {
	token, repository, owner, customActionName string
	client                                     *github.Client
}

func NewGithubIssueList(cfg map[string]string) *GithubIssueList {
	client := github.NewClient(nil).WithAuthToken(cfg["token"])
	return &GithubIssueList{
		client:           client,
		token:            cfg["token"],
		repository:       cfg["repository"],
		owner:            cfg["owner"],
		customActionName: cfg["customActionName"],
	}
}

func (g *GithubIssueList) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		Repository string `json:"repository"`
		Owner      string `json:"owner"`
		State      string `json:"state"`
		Labels     string `json:"labels"`
		Assignee   string `json:"assignee"`
		Milestone  string `json:"milestone"`
		Sort       string `json:"sort"`
		Limit      int    `json:"limit"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	if g.repository != "" && g.owner != "" {
		p.Repository = g.repository
		p.Owner = g.owner
	}
	if p.State == "" {
		p.State = "open"
	}
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 30
	}
	if p.Sort == "" {
		p.Sort = "created"
	}

	opts := &github.IssueListByRepoOptions{
		State:       p.State,
		Sort:        p.Sort,
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: p.Limit},
	}
	if p.Labels != "" {
		opts.Labels = strings.Split(p.Labels, ",")
	}
	if p.Assignee != "" {
		opts.Assignee = p.Assignee
	}
	if p.Milestone != "" {
		opts.Milestone = p.Milestone
	}

	issues, _, err := g.client.Issues.ListByRepo(ctx, p.Owner, p.Repository, opts)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error listing issues: %v", err)}, err
	}

	var sb strings.Builder
	count := 0
	for _, issue := range issues {
		// Skip pull requests (GitHub API returns PRs in issue endpoints).
		if issue.PullRequestLinks != nil {
			continue
		}
		count++

		labels := make([]string, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, l.GetName())
		}
		assignees := make([]string, 0, len(issue.Assignees))
		for _, a := range issue.Assignees {
			assignees = append(assignees, a.GetLogin())
		}

		sb.WriteString(fmt.Sprintf("#%d %s [%s]", issue.GetNumber(), issue.GetTitle(), issue.GetState()))
		if len(labels) > 0 {
			sb.WriteString(fmt.Sprintf(" labels:[%s]", strings.Join(labels, ",")))
		}
		if len(assignees) > 0 {
			sb.WriteString(fmt.Sprintf(" assignees:[%s]", strings.Join(assignees, ",")))
		}
		if issue.Milestone != nil {
			sb.WriteString(fmt.Sprintf(" milestone:%s", issue.Milestone.GetTitle()))
		}
		sb.WriteString("\n")
	}

	if count == 0 {
		sb.WriteString("No issues found matching the given filters.\n")
	} else {
		sb.WriteString(fmt.Sprintf("\nTotal: %d issues\n", count))
	}

	return types.ActionResult{Result: sb.String()}, nil
}

func (g *GithubIssueList) Definition() types.ActionDefinition {
	actionName := "list_github_issues"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	desc := "List issues in a GitHub repository with optional filters for state, labels, assignee, and milestone."

	props := map[string]jsonschema.Definition{
		"state": {
			Type:        jsonschema.String,
			Description: "Issue state: open, closed, or all. Default: open.",
			Enum:        []string{"open", "closed", "all"},
		},
		"labels": {
			Type:        jsonschema.String,
			Description: "Comma-separated label names to filter by.",
		},
		"assignee": {
			Type:        jsonschema.String,
			Description: "Username to filter by assignee. Use * for any, none for unassigned.",
		},
		"milestone": {
			Type:        jsonschema.String,
			Description: "Milestone number to filter by. Use * for any, none for no milestone.",
		},
		"sort": {
			Type:        jsonschema.String,
			Description: "Sort by: created, updated, or comments. Default: created.",
			Enum:        []string{"created", "updated", "comments"},
		},
		"limit": {
			Type:        jsonschema.Number,
			Description: "Maximum number of issues to return (1-100, default 30).",
		},
	}

	if g.repository != "" && g.owner != "" {
		return types.ActionDefinition{
			Name: types.ActionDefinitionName(actionName), Description: desc,
			Properties: props,
			Required:   []string{},
		}
	}

	props["repository"] = jsonschema.Definition{
		Type: jsonschema.String, Description: "The repository name.",
	}
	props["owner"] = jsonschema.Definition{
		Type: jsonschema.String, Description: "The repository owner.",
	}
	return types.ActionDefinition{
		Name: types.ActionDefinitionName(actionName), Description: desc,
		Properties: props,
		Required:   []string{"repository", "owner"},
	}
}

func (g *GithubIssueList) Plannable() bool { return true }

func GithubIssueListConfigMeta() []config.Field {
	return []config.Field{
		{Name: "token", Label: "GitHub Token", Type: config.FieldTypeText, Required: true, HelpText: "GitHub API token with repository access"},
		{Name: "repository", Label: "Repository", Type: config.FieldTypeText, HelpText: "Default repository name"},
		{Name: "owner", Label: "Owner", Type: config.FieldTypeText, HelpText: "Default repository owner"},
		{Name: "customActionName", Label: "Custom Action Name", Type: config.FieldTypeText, HelpText: "Custom name for this action"},
	}
}

