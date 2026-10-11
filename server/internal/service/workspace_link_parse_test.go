package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestWorkspaceLinkParserBoundaries(t *testing.T) {
	source := "[inline](a.md#one) [with title](<b.md> \"title\") [reference][r] [r][] [r]\n\n[r]: c.md \"C\"\n\n![image](hidden.md) ` [code](hidden.md) `\n~~~\n[code](hidden.md)\n~~~\n\n    [indented](hidden.md)\n\n<!-- [comment](hidden.md) -->\n\n[paren](a(b).md)"
	got, err := markdownLinkDestinations(t.Context(), []byte(source))
	want := []string{"a.md#one", "b.md", "c.md", "c.md", "c.md", "a(b).md"}
	if err != nil || !reflect.DeepEqual(got, want) { t.Fatal(got, err) }
	for _, text := range []string{strings.Repeat("x", workspaceLinkBytes + 1), strings.Repeat("[x](x.md) ", workspaceMetadataBudget + 1), "bad\x00text", string([]byte{0xff})} {
		if _, err := markdownLinkDestinations(t.Context(), []byte(text)); err == nil { t.Fatal("unbounded/invalid source accepted") }
	}
	if ordinaryLinkDocument(File{Path: "vault/.pinax/canonical.md"}) || ordinaryLinkDocument(File{Path: "note.txt"}) || !ordinaryLinkDocument(File{Path: "note.markdown"}) { t.Fatal("ordinary Markdown boundary changed") }
}

func TestWorkspaceLinkParserExcludesNestedCodeAndDefinitions(t *testing.T) {
	for _, source := range []string{
		">     [code](hidden.md)",
		"`\n[r]: hidden.md\n`\n[r]",
		"<!--\n[r]: hidden.md\n-->\n[r]",
		"> ```\n> [code](hidden.md)\n> ```",
		"- item\n\n      [code](hidden.md)",
		strings.Repeat("<", workspaceLinkBytes),
	} {
		got, err := markdownLinkDestinations(t.Context(), []byte(source))
		if err != nil || len(got) != 0 { t.Fatalf("excluded source became link: %d bytes, %#v, %v", len(source), got, err) }
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := markdownLinkDestinations(ctx, []byte("[link](target.md)")); !errors.Is(err, context.Canceled) { t.Fatal("canceled parse ran", err) }
}
