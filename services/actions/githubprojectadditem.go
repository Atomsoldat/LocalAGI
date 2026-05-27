package actions

import (
	"context"
	"fmt"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubProjectAddItem struct {
	githubProjectBase
}

func NewGithubProjectAddItem(cfg map[string]string) *GithubProjectAddItem {
	return &GithubProjectAddItem{githubProjectBase: newGithubProjectBase(cfg)}
}

func (g *GithubProjectAddItem) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		IssueNumber int    `json:"issue_number"`
		Repository  string `json:"repository"`
		Owner       string `json:"owner"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	if p.Repository == "" {
		p.Repository = g.repository
	}
	if p.Owner == "" {
		p.Owner = g.repoOwner
	}
	if p.IssueNumber <= 0 {
		return types.ActionResult{}, fmt.Errorf("issue_number is required")
	}

	pc := g.projectClient()
	item, err := pc.AddItemByNumber(ctx, p.Owner, p.Repository, p.IssueNumber, "Issue")
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error adding item to project: %v", err)}, err
	}

	result := fmt.Sprintf("Added item to project %d (item ID: %d)", g.projectNumber, item.ID)
	return types.ActionResult{Result: result}, nil
}

func (g *GithubProjectAddItem) Definition() types.ActionDefinition {
	actionName := "add_item_to_github_project"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	desc := "Add an existing GitHub issue to the project board by its issue number."

	props := map[string]jsonschema.Definition{
		"issue_number": {
			Type:        jsonschema.Number,
			Description: "The issue number to add to the project.",
		},
	}
	required := []string{"issue_number"}

	if g.repository == "" {
		props["repository"] = jsonschema.Definition{
			Type: jsonschema.String, Description: "The repository containing the issue.",
		}
		props["owner"] = jsonschema.Definition{
			Type: jsonschema.String, Description: "The repository owner.",
		}
		required = append(required, "repository", "owner")
	}

	return types.ActionDefinition{
		Name: types.ActionDefinitionName(actionName), Description: desc,
		Properties: props, Required: required,
	}
}

func (g *GithubProjectAddItem) Plannable() bool { return true }

func GithubProjectAddItemConfigMeta() []config.Field {
	return GithubProjectBaseConfigMeta()
}
