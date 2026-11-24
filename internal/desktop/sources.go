package desktop

import (
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type registeredSource struct {
	bound boundSource
	view  desktopcontract.KnowledgeSource
}
type sourceCatalog interface {
	sources(string, int) ([]registeredSource, string, error)
}

func (a *BridgeAdmission) sources(cursor string, limit int) ([]registeredSource, string, error) {
	rows, next, err := a.service.GuardSources(a.instance, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	result := make([]registeredSource, 0, len(rows))
	for _, item := range rows {
		row := item.Guard
		view := desktopcontract.KnowledgeSource{SourceID: row.SourceID, Kind: "pinax", InstanceID: a.instance, DisplayName: "本机 Pinax 来源", State: "ready", Capabilities: []string{}, BindingRevision: row.Revision}
		lease, err := a.Acquire(item.Root, row.ID, row.Epoch, []string{"reader.metadata"})
		if err == nil {
			err = lease()
		}
		if err != nil {
			view.State = "blocked"
		} else {
			if row.State == "hidden" {
				view.State = "hidden"
			}
			for _, scope := range strings.Split(row.Scopes, ",") {
				if strings.HasPrefix(scope, "reader.") {
					view.Capabilities = append(view.Capabilities, scope)
				}
			}
		}
		result = append(result, registeredSource{bound: boundSource{BindingID: row.ID, SourceID: row.SourceID, Root: item.Root, Epoch: row.Epoch, Scopes: strings.Split(row.Scopes, ",")}, view: view})
	}
	return result, next, nil
}
func (f *facade) listSources(w http.ResponseWriter, r *http.Request) {
	q, err := readerQuery(r, "limit", "cursor")
	if err != nil || r.ContentLength != 0 {
		facadeError(w, "request_invalid")
		return
	}
	limit := 50
	if q.Get("limit") != "" {
		limit, err = strconv.Atoi(q.Get("limit"))
	}
	if err != nil || limit < 1 || limit > 200 || q.Get("cursor") != "" && !facadeKey.MatchString(q.Get("cursor")) {
		facadeError(w, "request_invalid")
		return
	}
	f.preflight.mu.Lock()
	catalog, ok := f.preflight.admission.(sourceCatalog)
	closed := f.preflight.closed
	f.preflight.mu.Unlock()
	if !ok || closed {
		facadeError(w, "desktop_contract_unsupported")
		return
	}
	rows, next, err := catalog.sources(q.Get("cursor"), limit)
	if err != nil {
		facadeError(w, err.Error())
		return
	}
	f.preflight.mu.Lock()
	defer f.preflight.mu.Unlock()
	if f.preflight.closed {
		facadeError(w, "desktop_session_invalid")
		return
	}
	page := desktopcontract.SourcePage{Items: []desktopcontract.KnowledgeSource{}, NextCursor: next, ObservedAt: time.Now().UTC()}
	for _, row := range rows {
		page.Items = append(page.Items, row.view)
		if row.view.State == "ready" || row.view.State == "hidden" {
			if old, exists := f.preflight.bindings[row.bound.BindingID]; exists && old.Epoch == row.bound.Epoch {
				continue
			}
			f.preflight.bindings[row.bound.BindingID] = row.bound
		} else {
			delete(f.preflight.bindings, row.bound.BindingID)
		}
	}
	facadeData(w, page)
}
