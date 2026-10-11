package sdk

import (
	"context"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

type PropertyMigrationPreview = contract.PropertyMigrationPreview
type PropertyMigrationDecision = contract.PropertyMigrationDecision
type PropertyMigrationLookup = contract.PropertyMigrationLookup
type PropertyMigrationReceipt = contract.PropertyMigrationReceipt
type PropertyMigrationView = contract.PropertyMigrationView
type PropertyMigrationItem = contract.PropertyMigrationItem
type PropertyMigrationCapability = contract.PropertyMigrationCapability

func (c *Client) GetPropertyMigrationCapability(ctx context.Context, spaceID string) (PropertyMigrationCapability, error) {
	var out PropertyMigrationCapability
	err := c.JSON(ctx, "GET", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/property-migrations/capability", "", nil, &out)
	return out, err
}

func (c *Client) PreviewPropertyMigration(ctx context.Context, spaceID string, q PropertyMigrationPreview, key string) (PropertyMigrationReceipt, error) {
	var out PropertyMigrationReceipt
	err := c.JSON(ctx, "POST", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/property-migrations", key, q, &out)
	return out, err
}

func (c *Client) GetPropertyMigration(ctx context.Context, id string) (PropertyMigrationView, error) {
	var out PropertyMigrationView
	err := c.JSON(ctx, "GET", "/api/v1/property-migrations/"+url.PathEscape(id), "", nil, &out)
	return out, err
}

func (c *Client) ConfirmPropertyMigration(ctx context.Context, id string, q PropertyMigrationDecision, key string) (PropertyMigrationReceipt, error) {
	var out PropertyMigrationReceipt
	err := c.JSON(ctx, "POST", "/api/v1/property-migrations/"+url.PathEscape(id)+"/confirm", key, q, &out)
	return out, err
}

func (c *Client) ExecutePropertyMigration(ctx context.Context, id string, q PropertyMigrationDecision, key string) (PropertyMigrationReceipt, error) {
	var out PropertyMigrationReceipt
	err := c.JSON(ctx, "POST", "/api/v1/property-migrations/"+url.PathEscape(id)+"/execute", key, q, &out)
	return out, err
}

func (c *Client) LookupPropertyMigration(ctx context.Context, spaceID string, q PropertyMigrationLookup) (PropertyMigrationReceipt, error) {
	var out PropertyMigrationReceipt
	err := c.JSON(ctx, "POST", "/api/v1/spaces/"+url.PathEscape(spaceID)+"/property-migrations/lookup", "", q, &out)
	return out, err
}
