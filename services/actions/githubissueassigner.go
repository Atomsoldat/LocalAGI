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

type GithubIssueAssigner struct {
	token, repository, owner, customActionName string
	client                                     *github.Client
}

func NewGithubIssueAssigner(cfg map[string]string) *GithubIssueAssigner {
	client := github.NewClient(nil).WithAuthToken(cfg["token"])
	return &GithubIssueAssigner{
		client:           client,
		token:            cfg["token"],
		repository:       cfg["repository"],
		owner:            cfg["owner"],
		customActionName: cfg["customActionName"],
	}
}

func (g *GithubIssueAssigner) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		Repository  string `json:"repository"`
		Owner       string `json:"owner"`
		IssueNumber int    `json:"issue_number"`
		Assignees   string `json:"assignees"`
		Action      string `json:"action"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	if g.repository != "" && g.owner != "" {
		p.Repository = g.repository
		p.Owner = g.owner
	}
	if p.Action == "" {
		p.Action = "add"
	}

	usernames := strings.Split(p.Assignees, ",")
	for i := range usernames {
		usernames[i] = strings.TrimSpace(usernames[i])
	}

	var resultString string
	var err error

	switch p.Action {
	case "add":
		_, _, err = g.client.Issues.AddAssignees(ctx, p.Owner, p.Repository, p.IssueNumber, usernames)
		resultString = fmt.Sprintf("Added assignees %v to issue #%d in %s/%s", usernames, p.IssueNumber, p.Owner, p.Repository)
	case "remove":
		_, _, err = g.client.Issues.RemoveAssignees(ctx, p.Owner, p.Repository, p.IssueNumber, usernames)
		resultString = fmt.Sprintf("Removed assignees %v from issue #%d in %s/%s", usernames, p.IssueNumber, p.Owner, p.Repository)
	default:
		return types.ActionResult{}, fmt.Errorf("unknown action %q, use 'add' or 'remove'", p.Action)
	}

	if err != nil {
		resultString = fmt.Sprintf("Error %sing assignees on issue #%d in %s/%s: %v", p.Action, p.IssueNumber, p.Owner, p.Repository, err)
	}
	return types.ActionResult{Result: resultString}, err
}

func (g *GithubIssueAssigner) Definition() types.ActionDefinition {
	actionName := "assign_github_issue"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	desc := "Add or remove assignees on a GitHub issue."

	props := map[string]jsonschema.Definition{
		"issue_number": {Type: jsonschema.Number, Description: "The issue number."},
		"assignees":    {Type: jsonschema.String, Description: "Comma-separated GitHub usernames to assign or unassign."},
		"action": {
			Type:        jsonschema.String,
			Description: "Whether to add or remove assignees. Default: add.",
			Enum:        []string{"add", "remove"},
		},
	}
	required := []string{"issue_number", "assignees"}

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

func (g *GithubIssueAssigner) Plannable() bool { return true }

func GithubIssueAssignerConfigMeta() []config.Field {
	return []config.Field{
		{Name: "token", Label: "GitHub Token", Type: config.FieldTypeText, Required: true, HelpText: "GitHub API token with repository access"},
		{Name: "repository", Label: "Repository", Type: config.FieldTypeText, HelpText: "Default repository name"},
		{Name: "owner", Label: "Owner", Type: config.FieldTypeText, HelpText: "Default repository owner"},
		{Name: "customActionName", Label: "Custom Action Name", Type: config.FieldTypeText, HelpText: "Custom name for this action"},
	}
}
