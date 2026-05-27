package actions

import (
	"context"
	"fmt"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubProjectUpdateStatus struct {
	githubProjectBase
}

func NewGithubProjectUpdateStatus(cfg map[string]string) *GithubProjectUpdateStatus {
	return &GithubProjectUpdateStatus{githubProjectBase: newGithubProjectBase(cfg)}
}

func (g *GithubProjectUpdateStatus) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		ItemID int64  `json:"item_id"`
		Status string `json:"status"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	pc := g.projectClient()

	// Look up the Status field and the target option ID.
	statusField, err := pc.findStatusField(ctx)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error: %v", err)}, err
	}
	optionID, err := findOptionID(statusField, p.Status)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error: %v", err)}, err
	}

	err = pc.UpdateItemFields(ctx, p.ItemID, map[string]interface{}{
		statusField.ID.String(): optionID,
	})
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error updating status: %v", err)}, err
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Moved item %d to status %q on project %d", p.ItemID, p.Status, g.projectNumber),
	}, nil
}

func (g *GithubProjectUpdateStatus) Definition() types.ActionDefinition {
	actionName := "update_github_project_item_status"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	return types.ActionDefinition{
		Name:        types.ActionDefinitionName(actionName),
		Description: "Move a GitHub Project item to a different status column (e.g. Todo, In Progress, Done).",
		Properties: map[string]jsonschema.Definition{
			"item_id": {
				Type:        jsonschema.Number,
				Description: "The project item ID (returned by list-items or add-item).",
			},
			"status": {
				Type:        jsonschema.String,
				Description: "The target status name. Must match one of the project's configured status options.",
			},
		},
		Required: []string{"item_id", "status"},
	}
}

func (g *GithubProjectUpdateStatus) Plannable() bool { return true }

func GithubProjectUpdateStatusConfigMeta() []config.Field {
	return GithubProjectBaseConfigMeta()
}
