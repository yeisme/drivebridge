package desktop

import "github.com/yeisme/drivebridge-server/pkg/desktopcontract"

// boundSource is the session-local identity of one confirmed binding. Root never
// crosses the loopback contract; SourceID is the only handle a renderer may use.
type boundSource struct {
	BindingID string
	SourceID  string
	Root      string
	Epoch     uint64
	Scopes    []string
	// Released keeps the root after release so a lost release response can be
	// reconciled with the original key. A released source never resolves reads.
	Released bool
	// confirmKey and receipt replay one successful confirmation after the
	// preview is consumed. A different key cannot mint a second binding.
	confirmKey string
	receipt    desktopcontract.BindingMutationReceipt
}
