package bridge

import (
	"errors"
	"regexp"
	"time"
)

// Provenance is caller-supplied labeling, never authentication or task state.
// Only opaque identifiers are accepted, not prompts, paths or bearer URLs.
type Provenance struct {
	Caller  string `json:"caller,omitempty"`
	TaskRef string `json:"task_ref,omitempty"`
}

var provenanceID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,159}$`)

func (p Provenance) Validate() error {
	for _, v := range []string{p.Caller, p.TaskRef} {
		if v != "" && !provenanceID.MatchString(v) {
			return errors.New("invalid_provenance")
		}
	}
	return nil
}

type SearchOptions struct {
	MIME  string
	After time.Time
}
