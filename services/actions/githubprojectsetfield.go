package actions

import (
	"context"
	"fmt"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubProjectSetField struct {
	githubProjectBase
}

func NewGithubProjectSetField(cfg map[string]string) *GithubProjectSetField {
	return &GithubProjectSetField{githubProjectBase: newGithubProjectBase(cfg)}
}

func (g *GithubProjectSetField) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		ItemID    int64  `json:"item_id"`
		FieldName string `json:"field_name"`
		Value     string `json:"value"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	pc := g.projectClient()

	// Find the field by name.
	fields, err := pc.ListFields(ctx)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error listing fields: %v", err)}, err
	}

	var targetField *ProjectV2FieldDef
	for i := range fields {
		if fields[i].Name == p.FieldName {
			targetField = &fields[i]
			break
		}
	}
	if targetField == nil {
		available := make([]string, 0, len(fields))
		for _, f := range fields {
			available = append(available, f.Name)
		}
		return types.ActionResult{
			Result: fmt.Sprintf("Field %q not found (available: %v)", p.FieldName, available),
		}, fmt.Errorf("field %q not found", p.FieldName)
	}

	// For single_select fields, resolve the value to an option ID.
	var fieldValue interface{} = p.Value
	if targetField.DataType == "single_select" {
		optionID, err := findOptionID(targetField, p.Value)
		if err != nil {
			return types.ActionResult{Result: fmt.Sprintf("Error: %v", err)}, err
		}
		fieldValue = optionID
	}

	err = pc.UpdateItemFields(ctx, p.ItemID, map[string]interface{}{
		targetField.ID.String(): fieldValue,
	})
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error updating field: %v", err)}, err
	}

	return types.ActionResult{
		Result: fmt.Sprintf("Set field %q to %q on item %d", p.FieldName, p.Value, p.ItemID),
	}, nil
}

func (g *GithubProjectSetField) Definition() types.ActionDefinition {
	actionName := "set_github_project_item_field"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	return types.ActionDefinition{
		Name:        types.ActionDefinitionName(actionName),
		Description: "Set a custom field value on a GitHub Project item. For single-select fields, provide the option name (not ID). For text/number/date fields, provide the value directly.",
		Properties: map[string]jsonschema.Definition{
			"item_id": {
				Type:        jsonschema.Number,
				Description: "The project item ID.",
			},
			"field_name": {
				Type:        jsonschema.String,
				Description: "The name of the field to update (e.g. Status, Priority, Size).",
			},
			"value": {
				Type:        jsonschema.String,
				Description: "The value to set. For single-select fields use the option name. To clear, use an empty string.",
			},
		},
		Required: []string{"item_id", "field_name", "value"},
	}
}

func (g *GithubProjectSetField) Plannable() bool { return true }

func GithubProjectSetFieldConfigMeta() []config.Field {
	return GithubProjectBaseConfigMeta()
}
