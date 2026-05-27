package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubMilestoneManager struct {
	token, repository, owner, customActionName string
	client                                     *github.Client
}

func NewGithubMilestoneManager(cfg map[string]string) *GithubMilestoneManager {
	client := github.NewClient(nil).WithAuthToken(cfg["token"])
	return &GithubMilestoneManager{
		client:           client,
		token:            cfg["token"],
		repository:       cfg["repository"],
		owner:            cfg["owner"],
		customActionName: cfg["customActionName"],
	}
}

func (g *GithubMilestoneManager) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		Action          string `json:"action"`
		Repository      string `json:"repository"`
		Owner           string `json:"owner"`
		Title           string `json:"title"`
		Description     string `json:"description"`
		DueDate         string `json:"due_date"`
		MilestoneNumber int    `json:"milestone_number"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	if g.repository != "" && g.owner != "" {
		p.Repository = g.repository
		p.Owner = g.owner
	}

	switch p.Action {
	case "list":
		return g.list(ctx, p.Owner, p.Repository)
	case "create":
		return g.create(ctx, p.Owner, p.Repository, p.Title, p.Description, p.DueDate)
	case "close":
		return g.close(ctx, p.Owner, p.Repository, p.MilestoneNumber)
	default:
		return types.ActionResult{}, fmt.Errorf("unknown action %q, use list, create, or close", p.Action)
	}
}

func (g *GithubMilestoneManager) list(ctx context.Context, owner, repo string) (types.ActionResult, error) {
	milestones, _, err := g.client.Issues.ListMilestones(ctx, owner, repo, &github.MilestoneListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 30},
	})
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error listing milestones: %v", err)}, err
	}

	if len(milestones) == 0 {
		return types.ActionResult{Result: "No open milestones found."}, nil
	}

	var sb strings.Builder
	for _, m := range milestones {
		sb.WriteString(fmt.Sprintf("#%d %s (open: %d, closed: %d)",
			m.GetNumber(), m.GetTitle(), m.GetOpenIssues(), m.GetClosedIssues()))
		if m.DueOn != nil {
			sb.WriteString(fmt.Sprintf(" due: %s", m.DueOn.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}
	return types.ActionResult{Result: sb.String()}, nil
}

func (g *GithubMilestoneManager) create(ctx context.Context, owner, repo, title, description, dueDate string) (types.ActionResult, error) {
	ms := &github.Milestone{
		Title:       &title,
		Description: &description,
	}
	if dueDate != "" {
		t, err := time.Parse("2006-01-02", dueDate)
		if err == nil {
			ts := github.Timestamp{Time: t}
			ms.DueOn = &ts
		}
	}

	created, _, err := g.client.Issues.CreateMilestone(ctx, owner, repo, ms)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error creating milestone: %v", err)}, err
	}
	return types.ActionResult{
		Result: fmt.Sprintf("Created milestone #%d %q in %s/%s", created.GetNumber(), title, owner, repo),
	}, nil
}

func (g *GithubMilestoneManager) close(ctx context.Context, owner, repo string, number int) (types.ActionResult, error) {
	state := "closed"
	_, _, err := g.client.Issues.EditMilestone(ctx, owner, repo, number, &github.Milestone{
		State: &state,
	})
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error closing milestone: %v", err)}, err
	}
	return types.ActionResult{
		Result: fmt.Sprintf("Closed milestone #%d in %s/%s", number, owner, repo),
	}, nil
}

func (g *GithubMilestoneManager) Definition() types.ActionDefinition {
	actionName := "manage_github_milestones"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	desc := "List, create, or close GitHub milestones in a repository."

	props := map[string]jsonschema.Definition{
		"action": {
			Type:        jsonschema.String,
			Description: "Operation: list, create, or close.",
			Enum:        []string{"list", "create", "close"},
		},
		"title": {
			Type:        jsonschema.String,
			Description: "Milestone title (required for create).",
		},
		"description": {
			Type:        jsonschema.String,
			Description: "Milestone description (optional, for create).",
		},
		"due_date": {
			Type:        jsonschema.String,
			Description: "Due date in YYYY-MM-DD format (optional, for create).",
		},
		"milestone_number": {
			Type:        jsonschema.Number,
			Description: "Milestone number (required for close).",
		},
	}
	required := []string{"action"}

	if g.repository == "" || g.owner == "" {
		props["repository"] = jsonschema.Definition{Type: jsonschema.String, Description: "The repository name."}
		props["owner"] = jsonschema.Definition{Type: jsonschema.String, Description: "The repository owner."}
		required = append(required, "repository", "owner")
	}

	return types.ActionDefinition{
		Name: types.ActionDefinitionName(actionName), Description: desc,
		Properties: props, Required: required,
	}
}

func (g *GithubMilestoneManager) Plannable() bool { return true }

func GithubMilestoneManagerConfigMeta() []config.Field {
	return []config.Field{
		{Name: "token", Label: "GitHub Token", Type: config.FieldTypeText, Required: true, HelpText: "GitHub API token with repository access"},
		{Name: "repository", Label: "Repository", Type: config.FieldTypeText, HelpText: "Default repository name"},
		{Name: "owner", Label: "Owner", Type: config.FieldTypeText, HelpText: "Default repository owner"},
		{Name: "customActionName", Label: "Custom Action Name", Type: config.FieldTypeText, HelpText: "Custom name for this action"},
	}
}
