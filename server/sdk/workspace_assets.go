package sdk

import (
	"context"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// Document, property and resource mutations below are human-only. Capability
// reads do not enable a disabled editor or execute resource checks implicitly.
func (c *Client) GetDocumentCapability(ctx context.Context, fileID string) (DocumentCapability, error) {
	var out DocumentCapability
	err := c.JSON(ctx, "GET", "/api/v1/files/"+url.PathEscape(fileID)+"/document-capability", "", nil, &out)
	return out, err
}

func (c *Client) SaveDocument(ctx context.Context, fileID string, input SaveDocumentRequest, key string) (DocumentSaveView, error) {
	var out DocumentSaveView
	err := c.JSON(ctx, "PUT", "/api/v1/files/"+url.PathEscape(fileID)+"/document", key, input, &out)
	return out, err
}

func (c *Client) LookupDocumentSave(ctx context.Context, fileID, key string) (DocumentLookupView, error) {
	var out DocumentLookupView
	err := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/document/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

func (c *Client) GetPropertyDefinitions(ctx context.Context, spaceID string) (PropertyDefinitionView, error) {
	var out PropertyDefinitionView
	err := c.JSON(ctx, "GET", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/property-definitions", "", nil, &out)
	return out, err
}

func (c *Client) UpdatePropertyDefinitions(ctx context.Context, spaceID string, input PropertyDefinitionUpdate, key string) (PropertyDefinitionReceipt, error) {
	var out PropertyDefinitionReceipt
	err := c.JSON(ctx, "PUT", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/property-definitions", key, input, &out)
	return out, err
}

func (c *Client) LookupPropertyDefinitionUpdate(ctx context.Context, spaceID, key string) (PropertyDefinitionReceipt, error) {
	var out PropertyDefinitionReceipt
	err := c.JSON(ctx, "POST", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/property-definitions/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

func (c *Client) GetFilePropertySidecar(ctx context.Context, fileID, versionID string) (PropertySidecarView, error) {
	var out PropertySidecarView
	q := url.Values{"version_id": {versionID}}
	err := c.JSON(ctx, "GET", "/api/v1/files/"+url.PathEscape(fileID)+"/properties?"+q.Encode(), "", nil, &out)
	return out, err
}

func (c *Client) PatchFilePropertySidecar(ctx context.Context, fileID string, input PropertySidecarPatch, key string) (PropertySidecarReceipt, error) {
	var out PropertySidecarReceipt
	err := c.JSON(ctx, "PATCH", "/api/v1/files/"+url.PathEscape(fileID)+"/properties", key, input, &out)
	return out, err
}

func (c *Client) LookupFilePropertySidecar(ctx context.Context, fileID string, input PropertySidecarLookup) (PropertySidecarReceipt, error) {
	var out PropertySidecarReceipt
	err := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/properties/lookup", "", input, &out)
	return out, err
}

type ResourceListOptions struct {
	SpaceID string
	Kind    string
	Cursor  string
	Limit   int
}

func (c *Client) RegisterResource(ctx context.Context, input RegisterResourceRequest, key string) (ResourceView, error) {
	var out ResourceView
	err := c.JSON(ctx, "POST", "/api/v1/resource-descriptors", key, input, &out)
	return out, err
}

func (c *Client) GetResource(ctx context.Context, descriptorID string) (ResourceView, error) {
	var out ResourceView
	err := c.JSON(ctx, "GET", "/api/v1/resource-descriptors/"+url.PathEscape(descriptorID), "", nil, &out)
	return out, err
}

func (c *Client) ListResources(ctx context.Context, options ResourceListOptions) (Page[ResourceView], error) {
	q := url.Values{}
	workspaceString(q, "space_id", options.SpaceID)
	workspaceString(q, "kind", options.Kind)
	var out Page[ResourceView]
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/resource-descriptors", q, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}

func (c *Client) LookupResourceRegistration(ctx context.Context, key string) (ResourceRegistrationReceipt, error) {
	var out ResourceRegistrationReceipt
	err := c.JSON(ctx, "POST", "/api/v1/resource-descriptors/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

// CheckResourceIntegrity executes the allowlisted encoding/integrity check, not
// installation, skill safety validation or permission to execute the resource.
func (c *Client) CheckResourceIntegrity(ctx context.Context, descriptorID string, input ResourceCheckRequest) (ResourceIntegrityReceipt, error) {
	var out ResourceIntegrityReceipt
	err := c.JSON(ctx, "POST", "/api/v1/resource-descriptors/"+url.PathEscape(descriptorID)+"/checks", "", input, &out)
	return out, err
}

func (c *Client) LookupResourceIntegrity(ctx context.Context, descriptorID string, input ResourceCheckRequest) (ResourceIntegrityReceipt, error) {
	var out ResourceIntegrityReceipt
	err := c.JSON(ctx, "POST", "/api/v1/resource-descriptors/"+url.PathEscape(descriptorID)+"/checks/lookup", "", input, &out)
	return out, err
}
