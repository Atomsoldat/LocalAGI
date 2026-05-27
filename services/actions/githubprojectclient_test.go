package actions_test

import (
	"context"
	"os"
	"strconv"

	"github.com/mudler/LocalAGI/services/actions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GithubProjectClient", Ordered, func() {
	var (
		ctx    context.Context
		config map[string]string
		base   actions.GithubProjectBaseForTest
		pc     *actions.GithubProjectClientForTest
	)

	BeforeAll(func() {
		ctx = context.Background()

		token := os.Getenv("GITHUB_TOKEN")
		owner := os.Getenv("TEST_PROJECT_OWNER")
		ownerType := os.Getenv("TEST_PROJECT_OWNER_TYPE")
		projectNum := os.Getenv("TEST_PROJECT_NUMBER")

		if token == "" || owner == "" || projectNum == "" {
			Skip("Skipping GitHub Project client tests: required environment variables not set (GITHUB_TOKEN, TEST_PROJECT_OWNER, TEST_PROJECT_NUMBER)")
		}

		if ownerType == "" {
			ownerType = "users"
		}

		config = map[string]string{
			"token":         token,
			"owner":         owner,
			"ownerType":     ownerType,
			"projectNumber": projectNum,
		}

		base = actions.NewGithubProjectBaseForTest(config)
		pc = actions.NewGithubProjectClientForTest(&base)
	})

	Describe("GetProject", func() {
		It("should return project details", func() {
			proj, err := pc.GetProject(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(proj).NotTo(BeNil())
			Expect(proj.Title).NotTo(BeEmpty())

			pn, _ := strconv.Atoi(config["projectNumber"])
			Expect(proj.Number).To(Equal(pn))
		})
	})

	Describe("ListFields", func() {
		It("should return at least one field", func() {
			fields, err := pc.ListFields(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(fields).NotTo(BeEmpty())
		})

		It("should include a Status single-select field", func() {
			fields, err := pc.ListFields(ctx)
			Expect(err).NotTo(HaveOccurred())

			var statusField *actions.ProjectV2FieldDef
			for i := range fields {
				if fields[i].Name == "Status" {
					statusField = &fields[i]
					break
				}
			}
			Expect(statusField).NotTo(BeNil(), "Expected a Status field on the project")
			Expect(statusField.DataType).To(Equal("single_select"))
			Expect(statusField.Options).NotTo(BeEmpty())
		})
	})

	Describe("ListItems", func() {
		It("should return items without error", func() {
			items, _, err := pc.ListItems(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			// May be empty on a fresh project, that's fine.
			Expect(items).NotTo(BeNil())
		})
	})
})
