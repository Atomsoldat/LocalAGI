package actions_test

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mudler/LocalAGI/core/types"
	"github.com/mudler/LocalAGI/services/actions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These tests exercise the full MVP action set against a real GitHub Project.
// They run sequentially because each step builds on the previous one.
//
// Required env vars:
//   GITHUB_TOKEN, TEST_REPO_OWNER, TEST_REPOSITORY,
//   TEST_PROJECT_OWNER, TEST_PROJECT_OWNER_TYPE, TEST_PROJECT_NUMBER
//
// The tests create a temporary issue, add it to the project, manipulate it,
// and clean up at the end.

var _ = Describe("GitHub Project MVP Actions", Ordered, func() {
	var (
		ctx            context.Context
		issueConfig    map[string]string
		projectConfig  map[string]string
		createdIssueNo  int
		createdIssueNo2 int
		projectItemID   int64
		createdMilestone int
	)

	BeforeAll(func() {
		ctx = context.Background()

		token := os.Getenv("GITHUB_TOKEN")
		owner := os.Getenv("TEST_REPO_OWNER")
		repo := os.Getenv("TEST_REPOSITORY")
		projOwner := os.Getenv("TEST_PROJECT_OWNER")
		projOwnerType := os.Getenv("TEST_PROJECT_OWNER_TYPE")
		projNumber := os.Getenv("TEST_PROJECT_NUMBER")

		if token == "" || owner == "" || repo == "" || projOwner == "" || projNumber == "" {
			Skip("Skipping GitHub Project MVP tests: required environment variables not set")
		}

		if projOwnerType == "" {
			projOwnerType = "users"
		}

		issueConfig = map[string]string{
			"token":      token,
			"owner":      owner,
			"repository": repo,
		}
		projectConfig = map[string]string{
			"token":         token,
			"owner":         projOwner,
			"ownerType":     projOwnerType,
			"projectNumber": projNumber,
			"repository":    repo,
			"repoOwner":     owner,
		}
	})

	// --- Issue List ---
	Describe("github-issue-list", func() {
		It("should list open issues without error", func() {
			action := actions.NewGithubIssueList(issueConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"state": "open",
				"limit": 5,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).NotTo(BeEmpty())
		})

		It("should expose correct definition", func() {
			action := actions.NewGithubIssueList(issueConfig)
			def := action.Definition()
			Expect(def.Name.String()).To(Equal("list_github_issues"))
			Expect(def.Properties).To(HaveKey("state"))
			Expect(def.Properties).To(HaveKey("labels"))
			// owner/repo should be hidden since they're in config
			Expect(def.Properties).NotTo(HaveKey("repository"))
		})
	})

	// --- Create a test issue (using existing opener) ---
	Describe("create test issue", func() {
		It("should create a test issue for subsequent steps", func() {
			opener := actions.NewGithubIssueOpener(issueConfig)
			result, err := opener.Run(ctx, nil, types.ActionParams{
				"title": "[Test] Project MVP integration test",
				"text":  "Automated test issue — will be cleaned up.",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Created issue"))

			// Extract issue number from result string.
			var num int
			_, scanErr := fmt.Sscanf(result.Result, "Created issue %d", &num)
			Expect(scanErr).NotTo(HaveOccurred())
			Expect(num).To(BeNumerically(">", 0))
			createdIssueNo = num
		})
	})

	// --- Project Get Fields ---
	Describe("github-project-get-fields", func() {
		It("should return fields including Status", func() {
			action := actions.NewGithubProjectGetFields(projectConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Status"))
			Expect(result.Result).To(ContainSubstring("single_select"))
		})
	})

	// --- Add Issue to Project ---
	Describe("github-project-add-item", func() {
		It("should add the test issue to the project", func() {
			action := actions.NewGithubProjectAddItem(projectConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"issue_number": createdIssueNo,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Added item to project"))

			// Extract item ID.
			var ignoredPN, id int64
			_, scanErr := fmt.Sscanf(result.Result, "Added item to project %d (item ID: %d)", &ignoredPN, &id)
			Expect(scanErr).NotTo(HaveOccurred())
			Expect(id).To(BeNumerically(">", 0))
			projectItemID = id
		})
	})

	// --- List Project Items ---
	Describe("github-project-list-items", func() {
		It("should list items and find our test item", func() {
			// Brief delay — GitHub's API has eventual consistency for project items.
			time.Sleep(3 * time.Second)

			action := actions.NewGithubProjectListItems(projectConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"per_page": 100,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring(fmt.Sprintf("ID:%d", projectItemID)))
		})
	})

	// --- Update Item Status ---
	Describe("github-project-update-item-status", func() {
		It("should move item to a non-default status", func() {
			// First discover what statuses exist on this project.
			fieldsAction := actions.NewGithubProjectGetFields(projectConfig)
			fieldsResult, err := fieldsAction.Run(ctx, nil, types.ActionParams{})
			Expect(err).NotTo(HaveOccurred())

			// Pick the second status option (first is usually the default/landing column).
			// The fields output looks like: "Status ... options: [Backlog, Ready, In progress, ...]"
			// We use the project client directly to get the actual option names.
			base := actions.NewGithubProjectBaseForTest(projectConfig)
			pc := actions.NewGithubProjectClientForTest(&base)
			fields, err := pc.ListFields(ctx)
			Expect(err).NotTo(HaveOccurred())

			var targetStatus string
			for _, f := range fields {
				if f.Name == "Status" && len(f.Options) > 1 {
					targetStatus = f.Options[1].Name.Raw
					break
				}
			}
			Expect(targetStatus).NotTo(BeEmpty(), "Need at least 2 status options; got: "+fieldsResult.Result)

			action := actions.NewGithubProjectUpdateStatus(projectConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"item_id": projectItemID,
				"status":  targetStatus,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring(targetStatus))
		})
	})

	// --- Issue Assigner ---
	Describe("github-issue-assigner", func() {
		It("should assign and unassign a user", func() {
			// Use the authenticated user. We'll assign the repo owner.
			assigner := actions.NewGithubIssueAssigner(issueConfig)
			result, err := assigner.Run(ctx, nil, types.ActionParams{
				"issue_number": createdIssueNo,
				"assignees":    issueConfig["owner"],
				"action":       "add",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Added assignees"))

			// Now remove.
			result, err = assigner.Run(ctx, nil, types.ActionParams{
				"issue_number": createdIssueNo,
				"assignees":    issueConfig["owner"],
				"action":       "remove",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Removed assignees"))
		})
	})

	// --- Project Report ---
	Describe("github-project-report", func() {
		It("should generate a status report with counts", func() {
			action := actions.NewGithubProjectReport(projectConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("## Project Status Report"))
			Expect(result.Result).To(ContainSubstring("**Total:**"))
		})
	})

	// ==================== Stage 2 Tests ====================

	// --- Set Field (generic) ---
	Describe("github-project-set-field", func() {
		It("should set a single-select field on an item", func() {
			// Use the Status field — set it to the first option (reset).
			base := actions.NewGithubProjectBaseForTest(projectConfig)
			pc := actions.NewGithubProjectClientForTest(&base)
			fields, err := pc.ListFields(ctx)
			Expect(err).NotTo(HaveOccurred())

			var firstStatus string
			for _, f := range fields {
				if f.Name == "Status" && len(f.Options) > 0 {
					firstStatus = f.Options[0].Name.Raw
					break
				}
			}
			Expect(firstStatus).NotTo(BeEmpty())

			action := actions.NewGithubProjectSetField(projectConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"item_id":    projectItemID,
				"field_name": "Status",
				"value":      firstStatus,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring(firstStatus))
		})
	})

	// --- Milestone Manager ---
	Describe("github-milestone-manager", func() {
		It("should create a milestone", func() {
			action := actions.NewGithubMilestoneManager(issueConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"action": "create",
				"title":  "[Test] Project MVP milestone test",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Created milestone"))

			var msNum int
			_, scanErr := fmt.Sscanf(result.Result, "Created milestone #%d", &msNum)
			Expect(scanErr).NotTo(HaveOccurred())
			createdMilestone = msNum
		})

		It("should list milestones", func() {
			action := actions.NewGithubMilestoneManager(issueConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"action": "list",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("[Test] Project MVP milestone test"))
		})

		It("should close the test milestone", func() {
			action := actions.NewGithubMilestoneManager(issueConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"action":           "close",
				"milestone_number": createdMilestone,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Closed milestone"))
		})
	})

	// --- Bulk Labeler ---
	Describe("github-issue-bulk-labeler", func() {
		It("should create a second test issue for bulk labeling", func() {
			opener := actions.NewGithubIssueOpener(issueConfig)
			result, err := opener.Run(ctx, nil, types.ActionParams{
				"title": "[Test] Bulk label test issue",
				"text":  "Automated test issue for bulk labeling.",
			})
			Expect(err).NotTo(HaveOccurred())

			var num int
			_, scanErr := fmt.Sscanf(result.Result, "Created issue %d", &num)
			Expect(scanErr).NotTo(HaveOccurred())
			createdIssueNo2 = num
		})

		It("should label both test issues", func() {
			action := actions.NewGithubIssueBulkLabeler(issueConfig)
			issues := fmt.Sprintf("%d,%d", createdIssueNo, createdIssueNo2)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"issue_numbers": issues,
				"labels":        "bug",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring(fmt.Sprintf("#%d: labeled", createdIssueNo)))
			Expect(result.Result).To(ContainSubstring(fmt.Sprintf("#%d: labeled", createdIssueNo2)))
		})
	})

	// --- Remove Item from Project ---
	Describe("github-project-remove-item", func() {
		It("should remove item from project", func() {
			action := actions.NewGithubProjectRemoveItem(projectConfig)
			result, err := action.Run(ctx, nil, types.ActionParams{
				"item_id": projectItemID,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Removed item"))
		})
	})

	// --- Cleanup ---
	Describe("cleanup", func() {
		It("should close test issues", func() {
			closeConfig := map[string]string{
				"token":      issueConfig["token"],
				"owner":      issueConfig["owner"],
				"repository": issueConfig["repository"],
			}
			closer := actions.NewGithubIssueCloser(closeConfig)

			result, err := closer.Run(ctx, nil, types.ActionParams{
				"issue_number": createdIssueNo,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Result).To(ContainSubstring("Closed"))

			if createdIssueNo2 > 0 {
				result, err = closer.Run(ctx, nil, types.ActionParams{
					"issue_number": createdIssueNo2,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Result).To(ContainSubstring("Closed"))
			}
		})
	})
})
