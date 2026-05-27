package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/google/go-github/v69/github"
)

// Response types for the ProjectsV2 REST API.
// go-github v69 has ProjectV2/ProjectV2Item types for webhook events but no
// service methods, so we define richer response structs that include field
// values and use Client.NewRequest / Client.Do directly.

// ProjectV2RichText is a text value with raw and HTML representations.
type ProjectV2RichText struct {
	Raw  string `json:"raw"`
	HTML string `json:"html,omitempty"`
}

// ProjectV2FieldOption is a single option in a single-select field.
type ProjectV2FieldOption struct {
	ID          string             `json:"id"`
	Name        ProjectV2RichText  `json:"name"`
	Description ProjectV2RichText  `json:"description,omitempty"`
	Color       string             `json:"color,omitempty"`
}

// ProjectV2FieldIteration represents an iteration setting.
type ProjectV2FieldIteration struct {
	ID        string            `json:"id"`
	StartDate string            `json:"start_date"`
	Duration  int               `json:"duration"`
	Title     ProjectV2RichText `json:"title,omitempty"`
}

// ProjectV2FieldConfig holds configuration for iteration fields.
type ProjectV2FieldConfig struct {
	StartDay   int                       `json:"start_day,omitempty"`
	Duration   int                       `json:"duration,omitempty"`
	Iterations []ProjectV2FieldIteration `json:"iterations,omitempty"`
}

// ProjectV2FieldDef describes a field on a project.
type ProjectV2FieldDef struct {
	ID            json.Number           `json:"id"`
	NodeID        string                `json:"node_id,omitempty"`
	Name          string                `json:"name"`
	DataType      string                `json:"data_type"`
	Options       []ProjectV2FieldOption `json:"options,omitempty"`
	Configuration *ProjectV2FieldConfig  `json:"configuration,omitempty"`
	DefaultValue  interface{}           `json:"default_value,omitempty"`
	ProjectURL    string                `json:"project_url,omitempty"`
}

// ProjectV2ItemField represents a field value on a project item.
// The schema is loosely defined by GitHub (additionalProperties: true),
// so we capture all keys via a raw map and provide accessor helpers.
type ProjectV2ItemField map[string]interface{}

// FieldID returns the "id" key as a string, if present.
func (f ProjectV2ItemField) FieldID() string {
	if v, ok := f["id"]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// FieldValue returns the "value" key, if present.
func (f ProjectV2ItemField) FieldValue() interface{} {
	return f["value"]
}

// FieldName returns the "name" key, if present.
func (f ProjectV2ItemField) FieldName() string {
	if v, ok := f["name"]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// ProjectV2ItemContent represents the content linked to a project item.
type ProjectV2ItemContent struct {
	ID     json.Number `json:"id,omitempty"`
	NodeID string      `json:"node_id,omitempty"`
	Number int         `json:"number,omitempty"`
	Title  string      `json:"title,omitempty"`
	URL    string      `json:"url,omitempty"`
}

// ProjectV2ItemDetail is a project item as returned by the REST API.
type ProjectV2ItemDetail struct {
	ID          int64                 `json:"id"`
	NodeID      string                `json:"node_id,omitempty"`
	ContentType string                `json:"content_type,omitempty"`
	Content     *ProjectV2ItemContent `json:"content,omitempty"`
	CreatedAt   string                `json:"created_at,omitempty"`
	UpdatedAt   string                `json:"updated_at,omitempty"`
	ArchivedAt  string                `json:"archived_at,omitempty"`
	Fields      []ProjectV2ItemField  `json:"fields,omitempty"`
	ItemURL     string                `json:"item_url,omitempty"`
	ProjectURL  string                `json:"project_url,omitempty"`
	Creator     *github.User          `json:"creator,omitempty"`
}

// ProjectV2Detail is the top-level project object from the REST API.
type ProjectV2Detail struct {
	ID               int64  `json:"id"`
	NodeID           string `json:"node_id,omitempty"`
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	ShortDescription string `json:"short_description,omitempty"`
	Public           bool   `json:"public"`
	Number           int    `json:"number"`
	CreatedAt        string `json:"created_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

// githubProjectClient wraps github.Client to provide ProjectsV2 REST calls.
type githubProjectClient struct {
	client        *github.Client
	owner         string
	ownerType     string // "users" or "orgs"
	projectNumber int
}

func (c *githubProjectClient) basePath() string {
	return fmt.Sprintf("%s/%s/projectsV2/%d", c.ownerType, c.owner, c.projectNumber)
}

// GetProject retrieves the project details.
func (c *githubProjectClient) GetProject(ctx context.Context) (*ProjectV2Detail, error) {
	path := c.basePath()
	req, err := c.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	var proj ProjectV2Detail
	_, err = c.client.Do(ctx, req, &proj)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	return &proj, nil
}

// ListFields returns all fields configured on the project.
func (c *githubProjectClient) ListFields(ctx context.Context) ([]ProjectV2FieldDef, error) {
	path := fmt.Sprintf("%s/fields", c.basePath())
	req, err := c.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	var fields []ProjectV2FieldDef
	_, err = c.client.Do(ctx, req, &fields)
	if err != nil {
		return nil, fmt.Errorf("listing fields: %w", err)
	}
	return fields, nil
}

// ListItemsOptions configures the ListItems call.
type ListItemsOptions struct {
	Query   string
	PerPage int
	After   string // pagination cursor
}

// ListItems returns items on the project, optionally filtered.
func (c *githubProjectClient) ListItems(ctx context.Context, opts *ListItemsOptions) ([]ProjectV2ItemDetail, string, error) {
	path := fmt.Sprintf("%s/items", c.basePath())

	params := url.Values{}
	if opts != nil {
		if opts.Query != "" {
			params.Set("q", opts.Query)
		}
		if opts.PerPage > 0 {
			params.Set("per_page", strconv.Itoa(opts.PerPage))
		}
		if opts.After != "" {
			params.Set("after", opts.After)
		}
	}
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	req, err := c.client.NewRequest("GET", path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}
	var items []ProjectV2ItemDetail
	resp, err := c.client.Do(ctx, req, &items)
	if err != nil {
		return nil, "", fmt.Errorf("listing items: %w", err)
	}

	// Extract next page cursor from Link header if present.
	var nextCursor string
	if resp != nil && resp.NextPage != 0 {
		nextCursor = strconv.Itoa(resp.NextPage)
	}

	return items, nextCursor, nil
}

// AddItemRequest is the body for adding an item to a project.
// Supports two modes: by database ID, or by owner/repo/number.
type AddItemRequest struct {
	Type   string `json:"type"`             // "Issue" or "PullRequest"
	ID     int64  `json:"id,omitempty"`     // The database ID of the issue or PR
	Owner  string `json:"owner,omitempty"`  // Repository owner (alternative to ID)
	Repo   string `json:"repo,omitempty"`   // Repository name (alternative to ID)
	Number int    `json:"number,omitempty"` // Issue/PR number (alternative to ID)
}

// AddItemByID adds an issue or PR to the project by its database ID.
func (c *githubProjectClient) AddItemByID(ctx context.Context, itemDatabaseID int64, itemType string) (*ProjectV2ItemDetail, error) {
	path := fmt.Sprintf("%s/items", c.basePath())
	body := &AddItemRequest{
		Type: itemType,
		ID:   itemDatabaseID,
	}
	return c.doAddItem(ctx, path, body)
}

// AddItemByNumber adds an issue or PR to the project by owner/repo/number.
func (c *githubProjectClient) AddItemByNumber(ctx context.Context, owner, repo string, number int, itemType string) (*ProjectV2ItemDetail, error) {
	path := fmt.Sprintf("%s/items", c.basePath())
	body := &AddItemRequest{
		Type:   itemType,
		Owner:  owner,
		Repo:   repo,
		Number: number,
	}
	return c.doAddItem(ctx, path, body)
}

func (c *githubProjectClient) doAddItem(ctx context.Context, path string, body *AddItemRequest) (*ProjectV2ItemDetail, error) {
	req, err := c.client.NewRequest("POST", path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	var item ProjectV2ItemDetail
	_, err = c.client.Do(ctx, req, &item)
	if err != nil {
		return nil, fmt.Errorf("adding item: %w", err)
	}
	return &item, nil
}

// UpdateItemFieldEntry is a single field update in the PATCH request.
type UpdateItemFieldEntry struct {
	ID    json.Number `json:"id"`
	Value interface{} `json:"value"`
}

// UpdateItemFieldsRequest is the body for updating field values on an item.
type UpdateItemFieldsRequest struct {
	Fields []UpdateItemFieldEntry `json:"fields"`
}

// UpdateItemFields sets field values on a project item.
// The fields map keys are field IDs (as strings), values depend on field type.
func (c *githubProjectClient) UpdateItemFields(ctx context.Context, itemID int64, fields map[string]interface{}) error {
	path := fmt.Sprintf("%s/items/%d", c.basePath(), itemID)

	entries := make([]UpdateItemFieldEntry, 0, len(fields))
	for id, val := range fields {
		entries = append(entries, UpdateItemFieldEntry{
			ID:    json.Number(id),
			Value: val,
		})
	}

	body := &UpdateItemFieldsRequest{Fields: entries}
	req, err := c.client.NewRequest("PATCH", path, body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	_, err = c.client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("updating item fields: %w", err)
	}
	return nil
}

// DeleteItem removes an item from the project.
func (c *githubProjectClient) DeleteItem(ctx context.Context, itemID int64) error {
	path := fmt.Sprintf("%s/items/%d", c.basePath(), itemID)
	req, err := c.client.NewRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	_, err = c.client.Do(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("deleting item: %w", err)
	}
	return nil
}

// findStatusField searches the project fields for the "Status" single-select
// field and returns its definition. Returns an error if not found.
func (c *githubProjectClient) findStatusField(ctx context.Context) (*ProjectV2FieldDef, error) {
	fields, err := c.ListFields(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range fields {
		if f.Name == "Status" && f.DataType == "single_select" {
			return &f, nil
		}
	}
	return nil, fmt.Errorf("Status field not found on project %d", c.projectNumber)
}

// findOptionID looks up the option ID for a given option name within a
// single-select field.
func findOptionID(field *ProjectV2FieldDef, optionName string) (string, error) {
	if len(field.Options) == 0 {
		return "", fmt.Errorf("field %q has no options", field.Name)
	}
	for _, opt := range field.Options {
		if opt.Name.Raw == optionName {
			return opt.ID, nil
		}
	}
	names := make([]string, 0, len(field.Options))
	for _, opt := range field.Options {
		names = append(names, opt.Name.Raw)
	}
	return "", fmt.Errorf("option %q not found in field %q (available: %v)", optionName, field.Name, names)
}
