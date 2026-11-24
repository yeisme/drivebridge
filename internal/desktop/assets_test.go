package desktop

import (
	"github.com/yeisme/pinax/pkg/pinaxclient"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAssetHandlesBindFixedSourceAndExpire(t *testing.T) {
	f := &facade{preflight: NewPreflight()}
	doc := pinaxclient.ReaderDocumentPage{ObjectID: "note", Revision: "sha256:" + strings.Repeat("a", 64), Nodes: []pinaxclient.ReaderDocumentNode{{Type: "paragraph", Children: []pinaxclient.ReaderDocumentNode{{Type: "image", Ref: "pinax-asset://owner-asset"}}}}}
	if err := f.documentAssets("source", &doc); err != nil {
		t.Fatal(err)
	}
	handle := strings.TrimPrefix(doc.Nodes[0].Children[0].Ref, "drivebridge-asset://asset/")
	asset, ok := f.assets[handle]
	if !ok || asset.source != "source" || asset.object != "note" || asset.revision != doc.Revision || asset.ref != "owner-asset" {
		t.Fatal("asset identity not bound")
	}
	other := &facade{preflight: NewPreflight()}
	response := httptest.NewRecorder()
	other.readAsset(response, httptest.NewRequest("GET", "/", nil), handle)
	if response.Code != 404 {
		t.Fatal("handle crossed facade sessions")
	}
	asset.expires = time.Now().Add(-time.Second)
	f.assets[handle] = asset
	response = httptest.NewRecorder()
	f.readAsset(response, httptest.NewRequest("GET", "/", nil), handle)
	if response.Code != 404 {
		t.Fatal("expired handle admitted")
	}
}
func TestAssetRangeRejectsAmbiguityBeforeOwnerAccess(t *testing.T) {
	f := &facade{preflight: NewPreflight(), assets: map[string]assetHandle{"asset": {expires: time.Now().Add(time.Minute)}}}
	for _, value := range []string{"bytes=0-", "bytes=-1", "bytes=2-1", "bytes=0-8388608", "bytes=0-1,3-4", "bytes=999999999999999999999-9999999999999999999999"} {
		request := httptest.NewRequest("GET", "/", nil)
		request.Header.Set("Range", value)
		response := httptest.NewRecorder()
		f.readAsset(response, request, "asset")
		if response.Code != 416 {
			t.Fatalf("range not rejected: %d", response.Code)
		}
	}
}
