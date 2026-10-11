package service

import (
	"bytes"
	"context"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const workspaceLinkBytes = 2 * 1024 * 1024

// Parse ordinary CommonMark links, not Pinax canonical relationships. The AST
// excludes code/comments/reference definitions from link evidence; no URL is
// fetched. Both input bytes and emitted destinations have independent budgets.
func markdownLinkDestinations(ctx context.Context, source []byte) ([]string, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if len(source) > workspaceLinkBytes || !utf8.Valid(source) || bytes.IndexByte(source, 0) >= 0 { return nil, code("query_budget_exceeded") }
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	out := []string{}
	err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if err := ctx.Err(); err != nil { return ast.WalkStop, err }
		if !entering { return ast.WalkContinue, nil }
		var destination []byte
		switch node := node.(type) {
		case *ast.Image:
			return ast.WalkSkipChildren, nil
		case *ast.Link:
			destination = node.Destination
		case *ast.AutoLink:
			destination = node.URL(source)
		}
		if destination != nil {
			if len(destination) > 4096 || len(out) >= workspaceMetadataBudget { return ast.WalkStop, code("query_budget_exceeded") }
			out = append(out, string(destination))
		}
		return ast.WalkContinue, nil
	})
	if err != nil { return nil, err }
	return out, nil
}
