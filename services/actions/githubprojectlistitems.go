package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubProjectListItems struct {
	githubProjectBase
}

func NewGithubProjectListItems(cfg map[string]string) *GithubProjectListItems {
	return &GithubProjectListItems{githubProjectBase: newGithubProjectBase(cfg)}
}

func (g *GithubProjectListItems) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		Query   string `json:"query"`
		PerPage int    `json:"per_page"`
	}{}
	// params may be empty — that's fine, list everything.
	_ = params.Unmarshal(&p)

	if p.PerPage <= 0 || p.PerPage > 100 {
		p.PerPage = 30
	}

	pc := g.projectClient()

	opts := &ListItemsOptions{
		Query:   p.Query,
		PerPage: p.PerPage,
	}

	items, _, err := pc.ListItems(ctx, opts)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error listing project items: %v", err)}, err
	}

	if len(items) == 0 {
		return types.ActionResult{Result: "No items found on the project board."}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project %d items (%d):\n\n", g.projectNumber, len(items)))
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("- ID:%d type:%s", item.ID, item.ContentType))
		if item.Content != nil && item.Content.Title != "" {
			sb.WriteString(fmt.Sprintf(" #%d %q", item.Content.Number, item.Content.Title))
		}
		// Include field values if present.
		for _, f := range item.Fields {
			name := f.FieldName()
			val := f.FieldValue()
			if name != "" && val != nil {
				sb.WriteString(fmt.Sprintf(" [%s=%v]", name, val))
			}
		}
		sb.WriteString("\n")
	}

	return types.ActionResult{Result: sb.String()}, nil
}

func (g *GithubProjectListItems) Definition() types.ActionDefinition {
	actionName := "list_github_project_items"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	return types.ActionDefinition{
		Name:        types.ActionDefinitionName(actionName),
		Description: "List items on the GitHub Project board. Optionally filter with a search query.",
		Properties: map[string]jsonschema.Definition{
			"query": {
				Type:        jsonschema.String,
				Description: "Optional search/filter query (GitHub search syntax).",
			},
			"per_page": {
				Type:        jsonschema.Number,
				Description: "Number of items to return (1-100, default 30).",
			},
		},
		Required: []string{},
	}
}

func (g *GithubProjectListItems) Plannable() bool { return true }

func GithubProjectListItemsConfigMeta() []config.Field {
	return GithubProjectBaseConfigMeta()
}
