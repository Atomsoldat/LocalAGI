package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubProjectReport struct {
	githubProjectBase
}

func NewGithubProjectReport(cfg map[string]string) *GithubProjectReport {
	return &GithubProjectReport{githubProjectBase: newGithubProjectBase(cfg)}
}

func (g *GithubProjectReport) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		GroupBy          string `json:"group_by"`
		IncludeAssignees bool   `json:"include_assignees"`
		ExcludeClosed    *bool  `json:"exclude_closed"`
	}{}
	_ = params.Unmarshal(&p)

	if p.GroupBy == "" {
		p.GroupBy = "Status"
	}
	// Default to excluding closed issues.
	excludeClosed := true
	if p.ExcludeClosed != nil {
		excludeClosed = *p.ExcludeClosed
	}

	pc := g.projectClient()

	// Fetch all fields to identify the grouping field.
	fields, err := pc.ListFields(ctx)
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("Error fetching fields: %v", err)}, err
	}

	var groupField *ProjectV2FieldDef
	for i := range fields {
		if fields[i].Name == p.GroupBy {
			groupField = &fields[i]
			break
		}
	}
	if groupField == nil {
		return types.ActionResult{Result: fmt.Sprintf("Field %q not found on project", p.GroupBy)}, fmt.Errorf("field %q not found", p.GroupBy)
	}

	// Collect all known option names for ordered output.
	optionNames := []string{}
	for _, o := range groupField.Options {
		optionNames = append(optionNames, o.Name.Raw)
	}

	// Fetch all items (paginate up to 300).
	var allItems []ProjectV2ItemDetail
	var cursor string
	for i := 0; i < 3; i++ {
		items, next, err := pc.ListItems(ctx, &ListItemsOptions{PerPage: 100, After: cursor})
		if err != nil {
			return types.ActionResult{Result: fmt.Sprintf("Error fetching items: %v", err)}, err
		}
		allItems = append(allItems, items...)
		if next == "" || len(items) == 0 {
			break
		}
		cursor = next
	}

	// Look up issue state for items that reference issues, so we can
	// annotate and optionally filter closed ones.
	issueStates := map[int]string{} // issue number -> "open"/"closed"
	if g.repository != "" && g.repoOwner != "" {
		for _, item := range allItems {
			if item.Content == nil || item.Content.Number <= 0 || item.ContentType != "Issue" {
				continue
			}
			issue, _, err := g.client.Issues.Get(ctx, g.repoOwner, g.repository, item.Content.Number)
			if err != nil {
				continue
			}
			issueStates[item.Content.Number] = issue.GetState()
		}
	}

	// Group items by the target field value, optionally filtering closed issues.
	groups := map[string][]ProjectV2ItemDetail{}
	closedCount := 0
	for _, item := range allItems {
		if excludeClosed && item.Content != nil && item.Content.Number > 0 {
			if state, ok := issueStates[item.Content.Number]; ok && state == "closed" {
				closedCount++
				continue
			}
		}
		val := "(none)"
		groupFieldID := groupField.ID.String()
		for _, f := range item.Fields {
			if f.FieldID() == groupFieldID {
				if v := f.FieldValue(); v != nil {
					val = fmt.Sprintf("%v", v)
				}
				break
			}
		}
		groups[val] = append(groups[val], item)
	}

	// Build the report.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Project Status Report (grouped by %s)\n\n", p.GroupBy))
	sb.WriteString("| " + p.GroupBy + " | Count | Items |\n")
	sb.WriteString("|---|---|---|\n")

	total := 0
	// Print known options in order first, then any remaining.
	printed := map[string]bool{}
	for _, name := range optionNames {
		items := groups[name]
		printed[name] = true
		ids := itemIDsWithState(items, issueStates)
		sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", name, len(items), ids))
		total += len(items)
	}
	for name, items := range groups {
		if printed[name] {
			continue
		}
		ids := itemIDsWithState(items, issueStates)
		sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", name, len(items), ids))
		total += len(items)
	}

	sb.WriteString(fmt.Sprintf("\n**Total:** %d items\n", total))
	if closedCount > 0 {
		sb.WriteString(fmt.Sprintf("(%d closed issues excluded)\n", closedCount))
	}

	return types.ActionResult{Result: sb.String()}, nil
}

func itemIDsWithState(items []ProjectV2ItemDetail, issueStates map[int]string) string {
	if len(items) == 0 {
		return "-"
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.Content != nil && item.Content.Number > 0 {
			label := fmt.Sprintf("#%d", item.Content.Number)
			if state, ok := issueStates[item.Content.Number]; ok && state == "closed" {
				label += " (closed)"
			}
			ids = append(ids, label)
		} else {
			ids = append(ids, fmt.Sprintf("ID:%d", item.ID))
		}
	}
	return strings.Join(ids, ", ")
}

func (g *GithubProjectReport) Definition() types.ActionDefinition {
	actionName := "github_project_report"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	return types.ActionDefinition{
		Name:        types.ActionDefinitionName(actionName),
		Description: "Generate a status report of the GitHub Project board, grouped by a field (default: Status). Shows counts per category. Closed issues are excluded by default.",
		Properties: map[string]jsonschema.Definition{
			"group_by": {
				Type:        jsonschema.String,
				Description: "Field name to group by. Default: Status.",
			},
			"include_assignees": {
				Type:        jsonschema.Boolean,
				Description: "Include per-assignee breakdown in the report.",
			},
			"exclude_closed": {
				Type:        jsonschema.Boolean,
				Description: "Exclude closed issues from the report. Default: true.",
			},
		},
		Required: []string{},
	}
}

func (g *GithubProjectReport) Plannable() bool { return true }

func GithubProjectReportConfigMeta() []config.Field {
	return GithubProjectBaseConfigMeta()
}
