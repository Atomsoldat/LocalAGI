package actions

import (
	"context"
	"fmt"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubProjectRemoveItem struct {
	githubProjectBase
}

func NewGithubProjectRemoveItem(cfg map[string]string) *GithubProjectRemoveItem {
	return &GithubProjectRemoveItem{githubProjectBase: newGithubProjectBase(cfg)}
}

func (g *GithubProjectRemoveItem) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		ItemID int64 `json:"item_id"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	pc := g.projectClient()
	err := pc.DeleteItem(ctx, p.ItemID)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error removing item %d from project: %v", p.ItemID, err)}, err
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Removed item %d from project %d", p.ItemID, g.projectNumber),
	}, nil
}

func (g *GithubProjectRemoveItem) Definition() types.ActionDefinition {
	actionName := "remove_github_project_item"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	return types.ActionDefinition{
		Name:        types.ActionDefinitionName(actionName),
		Description: "Remove an item from the GitHub Project board. This does not close the underlying issue.",
		Properties: map[string]jsonschema.Definition{
			"item_id": {
				Type:        jsonschema.Number,
				Description: "The project item ID to remove (returned by list-items or add-item).",
			},
		},
		Required: []string{"item_id"},
	}
}

func (g *GithubProjectRemoveItem) Plannable() bool { return true }

func GithubProjectRemoveItemConfigMeta() []config.Field {
	return GithubProjectBaseConfigMeta()
}
