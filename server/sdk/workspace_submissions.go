package sdk

import (
	"context"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

type SubmissionListOptions struct {
	SpaceID string
	State   string
	AgentID string
	Cursor  string
	Limit   int
}

type LabelSuggestionListOptions struct {
	VersionID string
	Cursor    string
	Limit     int
}

// CreateInboxSubmission allocates staging through the authenticated Agent. The
// returned original operation owns part upload/completion; no destination or
// grant identity is accepted from the caller.
func (c *Client) CreateInboxSubmission(ctx context.Context, input CreateSubmissionRequest, key string) (SubmissionUploadView, error) {
	var out SubmissionUploadView
	err := c.JSON(ctx, "POST", "/api/v1/inbox-submissions", key, input, &out)
	return out, err
}

func (c *Client) LookupInboxSubmission(ctx context.Context, key string) (SubmissionUploadView, error) {
	var out SubmissionUploadView
	err := c.JSON(ctx, "POST", "/api/v1/inbox-submissions/lookup", "", contract.IdempotencyLookupRequest{IdempotencyKey: key}, &out)
	return out, err
}

func (c *Client) ListInboxSubmissions(ctx context.Context, options SubmissionListOptions) (Page[SubmissionView], error) {
	q := url.Values{}
	workspaceString(q, "space_id", options.SpaceID)
	workspaceString(q, "state", options.State)
	workspaceString(q, "agent_id", options.AgentID)
	var out Page[SubmissionView]
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/inbox-submissions", q, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}

func (c *Client) GetInboxSubmission(ctx context.Context, submissionID string) (SubmissionView, error) {
	var out SubmissionView
	err := c.JSON(ctx, "GET", "/api/v1/inbox-submissions/"+url.PathEscape(submissionID), "", nil, &out)
	return out, err
}

// DecideInboxSubmission is human-only and preserves the revision/version CAS.
func (c *Client) DecideInboxSubmission(ctx context.Context, submissionID string, input SubmissionDecisionRequest, key string) (SubmissionView, error) {
	var out SubmissionView
	err := c.JSON(ctx, "POST", "/api/v1/inbox-submissions/"+url.PathEscape(submissionID)+"/decision", key, input, &out)
	return out, err
}

// ArchiveInboxSubmission is human-only. A nil FileRef is not proof of a
// completed archive; use OperationRef to read the original operation.
func (c *Client) ArchiveInboxSubmission(ctx context.Context, submissionID string, input ArchiveSubmissionRequest, key string) (ArchiveSubmissionView, error) {
	var out ArchiveSubmissionView
	err := c.JSON(ctx, "POST", "/api/v1/inbox-submissions/"+url.PathEscape(submissionID)+"/archive", key, input, &out)
	return out, err
}

func (c *Client) CreateLabelSuggestion(ctx context.Context, fileID string, input LabelSuggestionRequest, key string) (LabelSuggestionView, error) {
	var out LabelSuggestionView
	err := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/label-suggestions", key, input, &out)
	return out, err
}

func (c *Client) ListLabelSuggestions(ctx context.Context, fileID string, options LabelSuggestionListOptions) (Page[LabelSuggestionView], error) {
	q := url.Values{}
	workspaceString(q, "version_id", options.VersionID)
	var out Page[LabelSuggestionView]
	err := c.JSON(ctx, "GET", workspaceQuery("/api/v1/files/"+url.PathEscape(fileID)+"/label-suggestions", q, options.Cursor, options.Limit), "", nil, &out)
	return out, err
}

// DecideLabelSuggestion is human-only; suggesting never grants approval rights.
func (c *Client) DecideLabelSuggestion(ctx context.Context, suggestionID string, input SuggestionDecisionRequest, key string) (LabelSuggestionView, error) {
	var out LabelSuggestionView
	err := c.JSON(ctx, "POST", "/api/v1/label-suggestions/"+url.PathEscape(suggestionID)+"/decision", key, input, &out)
	return out, err
}
