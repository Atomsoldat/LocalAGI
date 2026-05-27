package actions

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v69/github"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type GithubIssueBulkLabeler struct {
	token, repository, owner, customActionName string
	client                                     *github.Client
}

func NewGithubIssueBulkLabeler(cfg map[string]string) *GithubIssueBulkLabeler {
	client := github.NewClient(nil).WithAuthToken(cfg["token"])
	return &GithubIssueBulkLabeler{
		client:           client,
		token:            cfg["token"],
		repository:       cfg["repository"],
		owner:            cfg["owner"],
		customActionName: cfg["customActionName"],
	}
}

func (g *GithubIssueBulkLabeler) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	p := struct {
		Repository   string `json:"repository"`
		Owner        string `json:"owner"`
		IssueNumbers string `json:"issue_numbers"`
		Labels       string `json:"labels"`
	}{}
	if err := params.Unmarshal(&p); err != nil {
		return types.ActionResult{}, err
	}

	if g.repository != "" && g.owner != "" {
		p.Repository = g.repository
		p.Owner = g.owner
	}

	labels := strings.Split(p.Labels, ",")
	for i := range labels {
		labels[i] = strings.TrimSpace(labels[i])
	}

	issueNums := strings.Split(p.IssueNumbers, ",")
	var results []string
	var lastErr error

	for _, numStr := range issueNums {
		numStr = strings.TrimSpace(numStr)
		num, err := strconv.Atoi(numStr)
		if err != nil {
			results = append(results, fmt.Sprintf("#%s: invalid number", numStr))
			continue
		}
		_, _, err = g.client.Issues.AddLabelsToIssue(ctx, p.Owner, p.Repository, num, labels)
		if err != nil {
			results = append(results, fmt.Sprintf("#%d: error: %v", num, err))
			lastErr = err
		} else {
			results = append(results, fmt.Sprintf("#%d: labeled [%s]", num, strings.Join(labels, ", ")))
		}
	}

	return types.ActionResult{Result: strings.Join(results, "\n")}, lastErr
}

func (g *GithubIssueBulkLabeler) Definition() types.ActionDefinition {
	actionName := "bulk_label_github_issues"
	if g.customActionName != "" {
		actionName = g.customActionName
	}
	desc := "Add labels to multiple GitHub issues at once."

	props := map[string]jsonschema.Definition{
		"issue_numbers": {
			Type:        jsonschema.String,
			Description: "Comma-separated issue numbers to label (e.g. \"1,2,5\").",
		},
		"labels": {
			Type:        jsonschema.String,
			Description: "Comma-separated label names to add to all specified issues.",
		},
	}
	required := []string{"issue_numbers", "labels"}

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

func (g *GithubIssueBulkLabeler) Plannable() bool { return true }

func GithubIssueBulkLabelerConfigMeta() []config.Field {
	return []config.Field{
		{Name: "token", Label: "GitHub Token", Type: config.FieldTypeText, Required: true, HelpText: "GitHub API token with repository access"},
		{Name: "repository", Label: "Repository", Type: config.FieldTypeText, HelpText: "Default repository name"},
		{Name: "owner", Label: "Owner", Type: config.FieldTypeText, HelpText: "Default repository owner"},
		{Name: "customActionName", Label: "Custom Action Name", Type: config.FieldTypeText, HelpText: "Custom name for this action"},
	}
}
