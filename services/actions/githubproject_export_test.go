package actions

// Export internal types for use in external test package (actions_test).

// GithubProjectBaseForTest wraps the unexported githubProjectBase.
type GithubProjectBaseForTest = githubProjectBase

// GithubProjectClientForTest wraps the unexported githubProjectClient.
type GithubProjectClientForTest = githubProjectClient

// NewGithubProjectBaseForTest creates a githubProjectBase from config.
func NewGithubProjectBaseForTest(cfg map[string]string) githubProjectBase {
	return newGithubProjectBase(cfg)
}

// NewGithubProjectClientForTest creates a githubProjectClient from a base.
func NewGithubProjectClientForTest(base *githubProjectBase) *githubProjectClient {
	return base.projectClient()
}
