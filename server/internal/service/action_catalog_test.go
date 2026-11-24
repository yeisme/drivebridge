package service

import (
	"testing"
	"time"
)

func TestActionCatalogIsScopedAndActionable(t *testing.T) {
	s, _, human := blobRig(t)
	var space Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&space).Error; err != nil {
		t.Fatal(err)
	}

	catalog, err := s.ActionCatalog(Principal{UserID: human.UserID}, space.ID)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.CatalogVersion != "1.0" || catalog.Context.LocalFileAccess {
		t.Fatalf("unexpected catalog context: %+v", catalog.Context)
	}
	if len(catalog.Actions) < 8 {
		t.Fatalf("catalog too small: %d", len(catalog.Actions))
	}
	for i := 1; i < len(catalog.Actions); i++ {
		if catalog.Actions[i-1].ID >= catalog.Actions[i].ID {
			t.Fatalf("catalog is not stable-sorted: %q before %q", catalog.Actions[i-1].ID, catalog.Actions[i].ID)
		}
	}
	inspect, ok := catalog.Find("files.inspect")
	if !ok || inspect.Tool != "stat" || inspect.SideEffect != "read" || inspect.ExecutionLocation != "server" {
		t.Fatalf("fixed-file inspect contract missing: %+v", inspect)
	}
	if inspect.Availability != "unknown" && inspect.Availability != "available" {
		t.Fatalf("unexpected storage readiness: %+v", inspect)
	}
	archive, ok := catalog.Find("inbox.archive")
	if !ok || archive.Actor != "human" || archive.Availability == "available" {
		t.Fatalf("human-only action must not be presented as an execution permit: %+v", archive)
	}
	if _, ok := catalog.Find("not-a-real-action"); ok {
		t.Fatal("unknown action appeared in catalog")
	}

	connection := Connection{ID: space.ConnectionID, Owner: human.UserID, Kind: "fixture", Name: "Fixture", Status: "verified", ObservedStatus: "verified"}
	observed := time.Now().UTC()
	connection.ObservedAt = &observed
	if err := s.DB.Save(&connection).Error; err != nil {
		t.Fatal(err)
	}
	catalog, err = s.ActionCatalog(Principal{UserID: human.UserID}, space.ID)
	if err != nil {
		t.Fatal(err)
	}
	inspect, _ = catalog.Find("files.inspect")
	if inspect.Availability != "available" || inspect.DependencyStatus != "ok" || inspect.ObservedAt == nil {
		t.Fatalf("observed readiness not projected: %+v", inspect)
	}
}
