package actions

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/pkg/config"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// GitPushBranchAction pushes a committed branch to its upstream remote.
// Credentials are stored in action config and never exposed to the LLM.
// The LLM only provides the repo name and branch name.
type GitPushBranchAction struct {
	reposBasePath string
	gitToken      string
}

func NewGitPushBranch(cfg map[string]string) *GitPushBranchAction {
	return &GitPushBranchAction{
		reposBasePath: cfg["repos_base_path"],
		gitToken:      cfg["git_token"],
	}
}

func (a *GitPushBranchAction) Definition() types.ActionDefinition {
	return types.ActionDefinition{
		Name:        "push_branch",
		Description: "Push a committed branch from a local repository to its upstream remote. Use this after an agent has committed changes and you want to publish the branch.",
		Properties: map[string]jsonschema.Definition{
			"repo_name": {
				Type:        jsonschema.String,
				Description: "Name of the repository directory (e.g. 'myrepo').",
			},
			"branch_name": {
				Type:        jsonschema.String,
				Description: "Name of the local branch to push (e.g. 'feature/fix-auth').",
			},
		},
		Required: []string{"repo_name", "branch_name"},
	}
}

func (a *GitPushBranchAction) Run(ctx context.Context, _ *types.AgentSharedState, params types.ActionParams) (types.ActionResult, error) {
	var result struct {
		RepoName   string `json:"repo_name"`
		BranchName string `json:"branch_name"`
	}
	if err := params.Unmarshal(&result); err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to parse params: %w", err)
	}

	repoPath := strings.TrimRight(a.reposBasePath, "/") + "/" + result.RepoName

	// Resolve the remote URL from the repo itself — no hardcoded remote needed.
	remoteCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "remote", "get-url", "origin")
	remoteOut, err := remoteCmd.Output()
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to read remote URL from repo %q: %w", repoPath, err)
	}
	remoteURL := strings.TrimSpace(string(remoteOut))

	// Inject the token into the URL server-side. The token never appears in
	// the repo config or in any message visible to the LLM.
	authenticatedURL, err := injectTokenIntoURL(remoteURL, a.gitToken)
	if err != nil {
		return types.ActionResult{}, fmt.Errorf("failed to construct authenticated remote URL: %w", err)
	}

	// Push using the ephemeral token URL — not stored in the repo's remote config.
	pushCmd := exec.CommandContext(ctx, "git", "-C", repoPath, "push", authenticatedURL, result.BranchName)
	out, err := pushCmd.CombinedOutput()
	if err != nil {
		return types.ActionResult{Result: fmt.Sprintf("push failed:\n%s", string(out))}, nil
	}

	return types.ActionResult{
		Result: fmt.Sprintf("branch '%s' of repo '%s' pushed successfully", result.BranchName, result.RepoName),
	}, nil
}

func injectTokenIntoURL(rawURL, token string) (string, error) {
	if token == "" {
		return rawURL, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid remote URL %q: %w", rawURL, err)
	}
	u.User = url.UserPassword("x-token-auth", token)
	return u.String(), nil
}

func GitPushBranchConfigMeta() []config.Field {
	return []config.Field{
		{
			Name:     "repos_base_path",
			Label:    "Repos Base Path",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "Absolute path in this container where repositories are mounted (e.g. /repos). Must match the shared volume mount point.",
		},
		{
			Name:     "git_token",
			Label:    "Git Token",
			Type:     config.FieldTypeText,
			Required: true,
			HelpText: "Personal access token for authenticating git push. Set here at deploy time — never passed to or visible by the LLM.",
		},
	}
}

func (a *GitPushBranchAction) Plannable() bool { return true }
