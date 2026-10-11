package fixtures

import (
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestArchivePublicationFaultIsExplicitAndOneShot(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "default-off", true: "one-shot"}[enabled], func(t *testing.T) {
			f, err := StartWithOptions(t.TempDir(), "", "127.0.0.1:0", Options{FailArchivePublishOnce: enabled})
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			create := func(action string) error {
				return f.Service.DB.Transaction(func(tx *gorm.DB) error {
					return tx.Create(&service.Audit{Owner: "fixture", ActorID: "fixture", Action: action, Outcome: "success", At: time.Now().UTC()}).Error
				})
			}
			if err := create("workspace.submission.accept"); err != nil {
				t.Fatal("unrelated audit was faulted")
			}
			err = create("workspace.submission.archived")
			if (err != nil) != enabled {
				t.Fatal("first publication failure did not match explicit option")
			}
			if err := create("workspace.submission.archived"); err != nil {
				t.Fatal("one-shot fault repeated")
			}
			var count int64
			if err := f.Service.DB.Model(&service.Audit{}).Where("action = ?", "workspace.submission.archived").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			expected := int64(2)
			if enabled {
				expected = 1
			}
			if count != expected {
				t.Fatal("failed transaction persisted publication audit")
			}
		})
	}
}
func TestArchivePublicationFaultRejectsNonLoopback(t *testing.T) {
	for _, listen := range []string{"0.0.0.0:0", ":0", "localhost:0", ""} {
		f, err := StartWithOptions(t.TempDir(), "", listen, Options{FailArchivePublishOnce: true})
		if f != nil {
			f.Close()
		}
		if err == nil {
			t.Fatal("fault fixture accepted non-explicit numeric loopback")
		}
	}
}
