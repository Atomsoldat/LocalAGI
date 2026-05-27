package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubProjectGetFields struct {
	githubProjectBase
}

func NewGithubProjectGetFields(cfg map[string]string) *GithubProjectGetFields {
	return &GithubProjectGetFields{githubProjectBase: newGithubProjectBase(cfg)}
}

func (g *GithubProjectGetFields) Run(ctx context.Context, _ *types.AgentSharedState, _ types.ActionParams) (types.ActionResult, error) {
	pc := g.projectClient()
	fields, err := pc.ListFields(ctx)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error listing project fields: %v", err)}, err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project %d fields:\n\n", g.projectNumber))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf("- %s (type: %s, id: %s)", f.Name, f.DataType, f.ID))
		if len(f.Options) > 0 {
			opts := make([]string, 0, len(f.Options))
			for _, o := range f.Options {
				opts = append(opts, o.Name.Raw)
			}
			sb.WriteString(fmt.Sprintf(" options: [%s]", strings.Join(opts, ", ")))
		}
		sb.WriteString("\n")
	}
	return types.ActionResult{Result: sb.String()}, nil
}

func (g *GithubProjectGetFields) Definition() types.ActionDefinition {
	actionName := "get_github_project_fields"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	return types.ActionDefinition{
		Name:        types.ActionDefinitionName(actionName),
		Description: "List all fields (columns, custom fields) configured on the GitHub Project board, including their available options.",
		Properties:  map[string]jsonschema.Definition{},
		Required:    []string{},
	}
}

func (g *GithubProjectGetFields) Plannable() bool { return true }

func GithubProjectGetFieldsConfigMeta() []config.Field {
	return GithubProjectBaseConfigMeta()
}
