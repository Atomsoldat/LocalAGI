package actions

import (
	"strconv"

	"github.com/google/go-github/v69/github"
	"github.com/mudler/LocalAGI/pkg/config"
)

// githubProjectBase holds the shared configuration for all ProjectsV2 actions.
type githubProjectBase struct {
	client           *github.Client
	token            string
	owner            string // user or org name
	ownerType        string // "users" or "orgs"
	projectNumber    int
	repository       string // default repo for issue operations
	repoOwner        string // default repo owner (often same as owner)
	customActionName string
}

func newGithubProjectBase(cfg map[string]string) githubProjectBase {
	client := github.NewClient(nil).WithAuthToken(cfg["token"])

	pn, _ := strconv.Atoi(cfg["projectNumber"])

	repoOwner := cfg["repoOwner"]
	if repoOwner == "" {
		repoOwner = cfg["owner"]
	}

	ownerType := cfg["ownerType"]
	if ownerType == "" {
		ownerType = "users"
	}

	return githubProjectBase{
		client:           client,
		token:            cfg["token"],
		owner:            cfg["owner"],
		ownerType:        ownerType,
		projectNumber:    pn,
		repository:       cfg["repository"],
		repoOwner:        repoOwner,
		customActionName: cfg["customActionName"],
	}
}

func (b *githubProjectBase) projectClient() *githubProjectClient {
	return &githubProjectClient{
		client:        b.client,
		owner:         b.owner,
		ownerType:     b.ownerType,
		projectNumber: b.projectNumber,
	}
}

// GithubProjectBaseConfigMeta returns the common configuration fields for all
// GitHub Project actions.
func GithubProjectBaseConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "token",
			Label:    "GitHub Token",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "GitHub PAT with project and repo scope",
		},
		{
			Name:     "owner",
			Label:    "Project Owner",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "User or organization that owns the project",
		},
		{
			Name:     "ownerType",
			Label:    "Owner Type",
			Type:     config.FieldTypeSelect,
			Required: true,
			Options: []config.FieldOption{
				{Label: "User", Value: "users"},
				{Label: "Organization", Value: "orgs"},
			},
			DefaultValue: "users",
			HelpText:     "Whether the project owner is a user or an organization",
		},
		{
			Name:     "projectNumber",
			Label:    "Project Number",
			Type:     config.FieldTypeNumber,
			Required: true,
			HelpText: "The project number (visible in the project URL)",
		},
		{
			Name:     "repository",
			Label:    "Default Repository",
			Type:     config.FieldTypeText,
			HelpText: "Default repository name for issue operations",
		},
		{
			Name:     "repoOwner",
			Label:    "Repository Owner",
			Type:     config.FieldTypeText,
			HelpText: "Repository owner (defaults to project owner if empty)",
		},
		{
			Name:     "customActionName",
			Label:    "Custom Action Name",
			Type:     config.FieldTypeText,
			HelpText: "Override the default tool name exposed to the LLM",
		},
	}
}
