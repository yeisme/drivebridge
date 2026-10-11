package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/glebarez/sqlite"
	"github.com/gofrs/flock"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	RequireExplicitAdminRead   bool   `json:"require_explicit_admin_read,omitempty"`
	ServiceIdentityIssuer      string `json:"service_identity_issuer,omitempty"`
	ServiceIdentityEndpoint    string `json:"service_identity_endpoint,omitempty"`
	ServiceAdmissionCredential string `json:"-"`
	State                      string `json:"-"`
	Origin                     string `json:"origin"`
	Issuer                     string `json:"issuer"`
	ClientID                   string `json:"client_id"`
	ClientSecret               string `json:"-"`
	MicrosoftClientID          string `json:"microsoft_client_id,omitempty"`
	MicrosoftSecret            string `json:"-"`
	GoogleClientID             string `json:"google_client_id,omitempty"`
	GoogleSecret               string `json:"-"`
	TenantName                 string `json:"tenant_name,omitempty"`
	TenantOrigin               string `json:"tenant_origin,omitempty"`
	TenantContact              string `json:"tenant_contact,omitempty"`
	Enabled                    bool   `json:"enabled"`
	Development                bool   `json:"development"`
	DocumentEditing            bool   `json:"document_editing,omitempty"`
	PropertyEditing            bool   `json:"property_editing,omitempty"`
	AutomationManagement       bool   `json:"automation_management,omitempty"`
	ShareReceivers             bool   `json:"share_receivers,omitempty"`
	HoldManagement bool `json:"hold_management,omitempty"`
	LabelManagement bool `json:"label_management,omitempty"`
	// TeamDSN points the metadata database at the team PostgreSQL
	// deployment. It is never serialized, logged, or echoed back.
	TeamDSN string `json:"-"`
	// CloudConnectors enables the unified three-provider cloud directory
	// surface. It defaults to false: new routes and tools report unsupported
	// until the deployment opts in.
	CloudConnectors bool `json:"cloud_connectors,omitempty"`
	// TransferPlans enables the cross-storage transfer surface; it also
	// defaults to false until the deployment opts in.
	TransferPlans bool   `json:"transfer_plans,omitempty"`
	MaxBytes      int64  `json:"max_bytes"`
	WebDir        string `json:"web_dir,omitempty"`
}
type User struct {
	Archived    bool   `json:"archived" gorm:"not null;default:false"`
	Revision    uint64 `json:"revision" gorm:"not null;default:1"`
	Disabled    bool   `json:"disabled"`
	ID          string `json:"id" gorm:"primaryKey"`
	Issuer      string `json:"-" gorm:"uniqueIndex:identity"`
	Subject     string `json:"-" gorm:"uniqueIndex:identity"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarRef   string `json:"avatar_ref,omitempty"`
}
type Space struct {
	TenantID           string `json:"tenant_id,omitempty" gorm:"index"`
	TeamID             string `json:"team_id,omitempty" gorm:"index"`
	ID                 string `json:"id" gorm:"primaryKey"`
	Owner              string `json:"-" gorm:"index"`
	Name               string `json:"name"`
	ConnectionID       string `json:"connection_id"`
	MaxBytes           int64  `json:"max_bytes"`
	Used               int64  `json:"used_bytes"`
	Reserved           int64  `json:"reserved_bytes"`
	Encrypted          bool   `json:"encrypted"`
	TrashRetentionDays int    `json:"trash_retention_days"`
	WrappedDEK         []byte `json:"-"`
}
type Connection struct {
	ID       string `json:"id" gorm:"primaryKey"`
	Owner    string `json:"-" gorm:"index"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Secret   []byte `json:"-"`
	Disabled bool   `json:"disabled"`
	Status   string `json:"status"`
	// ObservedAt/ObservedStatus are the additive readiness observation
	// columns: NULL means "never probed" and never fabricates liveness.
	// Legacy rows stay NULL; rollback only drops these nullable columns.
	ObservedAt     *time.Time `json:"connection_observed_at,omitempty" gorm:"column:observed_at"`
	ObservedStatus string     `json:"connection_observed_status,omitempty" gorm:"column:observed_status"`
}
type File struct {
	ReviewPending bool       `json:"review_pending" gorm:"not null;default:false"`
	Caller        string     `json:"caller,omitempty"`
	TaskRef       string     `json:"task_ref,omitempty"`
	ID            string     `json:"id" gorm:"primaryKey;index:idx_files_catalogue_order,priority:3"`
	Owner         string     `json:"-" gorm:"index;index:idx_files_catalogue_order,priority:1"`
	SpaceID       string     `json:"space_id" gorm:"uniqueIndex:file_path"`
	Path          string     `json:"path" gorm:"uniqueIndex:file_path"`
	VersionID     string     `json:"version_id"`
	Size          int64      `json:"size_bytes"`
	MIME          string     `json:"mime_type"`
	Trashed       bool       `json:"trashed"`
	TrashedAt     time.Time  `json:"trashed_at,omitempty"`
	PurgeAfter    *time.Time `json:"purge_after,omitempty" gorm:"-"`
	// PurgeStartedAt is the two-phase purge mark: set in the first purge
	// transaction and cleared with the row in the second. While set, new
	// retention holds on this file fail with purge_in_progress. Additive
	// nullable column; rollback only drops it.
	PurgeStartedAt *time.Time `json:"-" gorm:"column:purge_started_at"`
	// The additive owner/updated/id index covers catalogue revision and
	// deterministic updated-time pagination, including equal-time ties.
	Updated time.Time `json:"updated_at" gorm:"index:idx_files_owner_updated;index:idx_files_catalogue_order,priority:2"`
}
type Version struct {
	Caller       string    `json:"caller,omitempty"`
	TaskRef      string    `json:"task_ref,omitempty"`
	ID           string    `json:"id" gorm:"primaryKey"`
	Owner        string    `json:"-" gorm:"index"`
	FileID       string    `json:"file_id" gorm:"index"`
	ConnectionID string    `json:"-"`
	ObjectKey    string    `json:"-"`
	BlobSHA256   string    `json:"-" gorm:"index"`
	SHA256       string    `json:"sha256"`
	Size         int64     `json:"size_bytes"`
	MIME         string    `json:"mime_type"`
	Created      time.Time `json:"created_at"`
}

// Blob is the content-addressed object record shared by versions of one data
// owner. Plaintext blobs are keyed by digest alone (scope ""); encrypted blobs
// add the space dimension. Refs is a logical reference count: reaching zero
// never deletes the object by itself.
type Blob struct {
	ID           string    `json:"-" gorm:"primaryKey"`
	SHA256       string    `json:"sha256" gorm:"index"`
	Owner        string    `json:"-" gorm:"index"`
	SpaceID      string    `json:"space_id,omitempty"`
	ConnectionID string    `json:"-"`
	ObjectKey    string    `json:"-"`
	Size         int64     `json:"size_bytes"`
	MIME         string    `json:"mime_type,omitempty"`
	Refs         int64     `json:"ref_count"`
	Encrypted    bool      `json:"encrypted"`
	CreatedAt    time.Time `json:"created_at"`
}
type Operation struct {
	// Workspace staging is admitted only by dedicated grant-bound services.
	Purpose              string `json:"purpose,omitempty" gorm:"not null;default:''"`
	WorkspaceGrantID     string `json:"-" gorm:"not null;default:'';index"`
	ActorID              string `json:"actor_id,omitempty"`
	ReviewPending        bool   `json:"review_pending,omitempty" gorm:"not null;default:false"`
	ArchiveFileID        string `json:"source_id,omitempty"`
	ArchiveVersionID     string `json:"source_version,omitempty"`
	ArchiveHeadVersion   string `json:"-"`
	RenameOnConflict     bool   `json:"-"`
	Caller               string `json:"caller,omitempty"`
	TaskRef              string `json:"task_ref,omitempty"`
	ExternalConnectionID string `json:"external_connection_id,omitempty"`
	ID                   string `json:"id" gorm:"primaryKey"`
	Owner                string `json:"-" gorm:"uniqueIndex:operation_key"`
	AgentID              string `json:"agent_id,omitempty"`
	Key                  string `json:"-" gorm:"uniqueIndex:operation_key"`
	Fingerprint          string `json:"-"`
	Kind                 string `json:"kind"`
	SpaceID              string `json:"space_id"`
	Path                 string `json:"path"`
	FileID               string `json:"file_id,omitempty"`
	ExpectedVersion      string `json:"-"`
	VersionID            string `json:"version_id,omitempty"`
	Size                 int64  `json:"size_bytes"`
	SHA256               string `json:"sha256"`
	MIME                 string `json:"mime_type"`
	Status               string `json:"status"`
	Transferred          int64  `json:"bytes_transferred"`
	BlobReused           bool   `json:"blob_reused,omitempty"`
	Error                string `json:"error,omitempty"`
	ConnectionID         string `json:"-"`
	SourceID             string `json:"-"`
	SourceVersion        string `json:"-"`
	SourceSpaceID        string `json:"source_space_id,omitempty"`
	SourcePath           string `json:"source_path,omitempty"`
	ProgressFiles        int    `json:"progress_files,omitempty"`
	TotalFiles           int    `json:"total_files,omitempty"`
	// BatchID/ItemKey link an upload operation to its organizing batch item.
	// Additive nullable columns: legacy rows keep empty strings and every
	// legacy query path ignores them.
	BatchID string    `json:"batch_id,omitempty" gorm:"index"`
	ItemKey string    `json:"item_key,omitempty"`
	Nonce   []byte    `json:"-"`
	Updated time.Time `json:"updated_at"`
}
type Part struct {
	OperationID string `gorm:"primaryKey"`
	Index       int    `gorm:"primaryKey"`
	Size        int64
	SHA256      string
}
type Agent struct {
	RestrictionProfile string    `json:"-" gorm:"not null;default:''"`
	ArchiveSuspended   bool      `json:"archive_suspended" gorm:"not null;default:false"`
	Archived           bool      `json:"archived" gorm:"not null;default:false"`
	ID                 string    `json:"id" gorm:"primaryKey"`
	Owner              string    `json:"-" gorm:"index"`
	Name               string    `json:"name"`
	SpaceID            string    `json:"space_id"`
	Prefix             string    `json:"prefix"`
	Scopes             string    `json:"scopes"`
	ConnectionID       string    `json:"connection_id,omitempty"`
	TokenHash          string    `json:"-" gorm:"uniqueIndex"`
	Expires            time.Time `json:"expires_at"`
	Revoked            bool      `json:"revoked"`
}
type UserKey struct {
	SealedKey []byte    `json:"-"`
	Archived  bool      `json:"archived" gorm:"not null;default:false"`
	Disabled  bool      `json:"disabled" gorm:"not null;default:false"`
	ID        string    `json:"id" gorm:"primaryKey"`
	Owner     string    `json:"-" gorm:"index"`
	Name      string    `json:"name"`
	TokenHash string    `json:"-" gorm:"uniqueIndex"`
	Admin     bool      `json:"admin"`
	Created   time.Time `json:"created_at" gorm:"column:created_at"`
	Expires   time.Time `json:"expires_at" gorm:"column:expires_at"`
	Revoked   bool      `json:"revoked"`
}
type Session struct {
	AuthenticatedAt time.Time
	Hash            string `gorm:"primaryKey"`
	Owner           string
	CSRF            string
	Admin           bool
	// OriginKey ties a key-exchange session to its issuing UserKey so
	// revocation cascades on the next request; empty for OIDC sessions.
	OriginKey string `gorm:"column:origin_key"`
	Expires   time.Time
}
type Flow struct {
	Hash    string `gorm:"primaryKey"`
	Owner   string
	Purpose string
	Secret  []byte
	Expires time.Time
}
type Audit struct {
	ActorID  string    `json:"actor_id,omitempty"`
	TenantID string    `json:"tenant_id,omitempty"`
	ID       uint      `json:"id" gorm:"primaryKey"`
	Owner    string    `json:"-" gorm:"index"`
	AgentID  string    `json:"agent_id,omitempty"`
	Action   string    `json:"action"`
	Resource string    `json:"resource"`
	Outcome  string    `json:"outcome"`
	At       time.Time `json:"at"`
}
type KeyMaterial struct {
	Version int    `gorm:"primaryKey"`
	Wrapped []byte `json:"-"`
}
type AccessGrant struct {
	ActorID   string
	TenantID  string
	Hash      string `gorm:"primaryKey"`
	Owner     string `gorm:"index"`
	FileID    string
	VersionID string
	Expires   time.Time
}
type HandoffGrant struct {
	BindingID      string     `json:"binding_id,omitempty"`
	Initiator      string     `json:"-"`
	IdempotencyKey *string    `json:"-" gorm:"uniqueIndex"`
	Fingerprint    string     `json:"-"`
	RequestRef     string     `json:"consumer_request_ref,omitempty"`
	RequestExpires *time.Time `json:"request_expires_at,omitempty"`
	Cancelled      bool       `json:"cancelled"`
	ID             string     `gorm:"primaryKey" json:"grant_id"`
	Owner          string     `gorm:"index" json:"-"`
	FileID         string     `gorm:"index" json:"file_id"`
	VersionID      string     `json:"version_id"`
	Consumer       string     `json:"consumer"`
	ProjectRef     string     `json:"project_ref"`
	Purpose        string     `json:"purpose"`
	Digest         string     `json:"sha256"`
	Expires        time.Time  `json:"expires_at"`
	Consumed       bool       `json:"consumed"`
}
type TenantProfile struct {
	ID      string    `gorm:"primaryKey" json:"-"`
	Name    string    `json:"name"`
	Origin  string    `json:"origin,omitempty"`
	Contact string    `json:"contact,omitempty"`
	Updated time.Time `json:"updated_at"`
}
type Principal struct {
	ShareID         string
	AccessGrantHash string
	ExternalRead    bool
	ServiceIdentity *serviceIdentity
	TenantID        string
	DataOwner       string
	ScopeSpace      string
	ExecutionID     string
	SessionHash     string
	UserID          string
	Agent           *Agent
	Key             *UserKey
	Admin           bool
	CSRF            string
}
type Service struct {
	governance        atomic.Bool
	creativeMu        sync.Mutex
	creativeOnce      sync.Once
	creativeInputsMu  sync.Mutex
	governanceMu      sync.Mutex
	previewCtx        context.Context
	previewCancel     context.CancelFunc
	previewWG         sync.WaitGroup
	previewSlots      chan struct{}
	sweepCancel       context.CancelFunc
	sweepWG           sync.WaitGroup
	Insights          *insights.Store
	insightOnce       sync.Once
	insightCancel     context.CancelFunc
	insightWG         sync.WaitGroup
	runtimeCancel     context.CancelFunc
	runtimeWG         sync.WaitGroup
	activeMu          sync.Mutex
	active            map[string]context.CancelFunc
	graphHTTP         *http.Client
	gdriveHTTP        *http.Client
	sensitiveTransfer func(context.Context, string, string, io.Reader, map[string]string) (*http.Response, error)
	DB                *gorm.DB
	Config            Config
	Instance          string
	crypt             cipher.AEAD
	key               []byte
	lock              *flock.Flock
	mu                sync.Mutex
	shareChallenges   map[string]shareChallengeState
	archiveMu         sync.Mutex
	opLocks           [32]sync.Mutex
	sourceLocks       [32]sync.Mutex
	oidc              *loginProvider
	serviceVerifier   *oidc.IDTokenVerifier
	storage           StorageFactory
	connectionLocks   [32]sync.Mutex
}

func ID() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func code(s string) error  { return errors.New(s) }
func privateDir(dir string) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return code("state_unavailable")
	}
	i, e := os.Lstat(dir)
	if e != nil || !i.IsDir() || i.Mode().Perm()&0077 != 0 {
		return code("state_not_private")
	}
	return nil
}
func Open(c Config) (*Service, error) {
	if c.MaxBytes <= 0 {
		c.MaxBytes = 1 << 30
	}
	if e := privateDir(c.State); e != nil {
		return nil, e
	}
	lock := flock.New(filepath.Join(c.State, "process.lock"))
	ok, e := lock.TryLock()
	if e != nil || !ok {
		return nil, code("state_in_use")
	}
	success := false
	defer func() {
		if !success {
			lock.Unlock()
		}
	}()
	keyPath := filepath.Join(c.State, "instance.key")
	key, e := os.ReadFile(keyPath)
	if os.IsNotExist(e) {
		if _, statErr := os.Stat(filepath.Join(c.State, "metadata.sqlite")); statErr == nil {
			return nil, code("instance_key_missing_restore_required")
		}
		key = make([]byte, 32)
		rand.Read(key)
		e = os.WriteFile(keyPath, key, 0600)
	}
	if e != nil || len(key) != 32 {
		return nil, code("instance_key_unavailable")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	aead, _ := cipher.NewGCM(block)
	databaseExisted := false
	var db *gorm.DB
	if c.TeamDSN != "" {
		// Team deployments run the metadata database on PostgreSQL with the
		// pure-Go driver; the DSN never appears in logs or status output.
		// Object bytes and the instance key stay in the state directory.
		db, e = gorm.Open(gormpostgres.Open(c.TeamDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if e != nil {
			return nil, code("database_unavailable")
		}
	} else {
		_, metadataErr := os.Stat(filepath.Join(c.State, "metadata.sqlite"))
		databaseExisted = metadataErr == nil
		db, e = gorm.Open(sqlite.Open(filepath.Join(c.State, "metadata.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if e != nil {
			return nil, code("database_unavailable")
		}
	}
	sqlDB, _ := db.DB()
	if c.TeamDSN == "" {
		// SQLite keeps the single-writer model; PostgreSQL allows the team
		// worker pool with SKIP LOCKED queue claims.
		sqlDB.SetMaxOpenConns(1)
	}
	if e = db.AutoMigrate(&User{}, &Space{}, &Connection{}, &File{}, &Version{}, &Operation{}, &Part{}, &Agent{}, &UserKey{}, &Session{}, &Flow{}, &Audit{}, &KeyMaterial{}, &AccessGrant{}, &HandoffGrant{}, &HandoffBinding{}, &TenantProfile{}, &Directory{}, &Preview{}, &Share{}, &ShareItem{}, &CreativeConnection{}, &CreativeReceipt{}, &CreativeAsset{}, &GovernanceState{}, &Tenant{}, &Membership{}, &PlatformRole{}, &Team{}, &TeamInvitation{}, &TeamStorage{}, &TeamMember{}, &SpacePermission{}, &GovernanceEvent{}, &Notification{}, &AccessRequest{}, &RecoveryClaim{}, &RecoveryCode{}, &Enrollment{}); e != nil {
		sqlDB.Close()
		return nil, code("database_unavailable")
	}
	os.Chmod(filepath.Join(c.State, "metadata.sqlite"), 0600)
	instancePath := filepath.Join(c.State, "instance.id")
	instance, e := os.ReadFile(instancePath)
	if os.IsNotExist(e) {
		if databaseExisted {
			sqlDB.Close()
			return nil, code("instance_metadata_missing_restore_required")
		}
		instance = []byte(ID())
		e = os.WriteFile(instancePath, instance, 0600)
	}
	if e != nil {
		sqlDB.Close()
		return nil, e
	}
	if len(instance) != 48 {
		sqlDB.Close()
		return nil, code("invalid_instance_identity")
	}
	if _, decodeErr := hex.DecodeString(string(instance)); decodeErr != nil {
		sqlDB.Close()
		return nil, code("invalid_instance_identity")
	}
	s := &Service{DB: db, Config: c, Instance: string(instance), crypt: aead, key: key, lock: lock, storage: NewS3, sensitiveTransfer: transferHTTP, active: map[string]context.CancelFunc{}}
	var legacyTeamSpaces []Space
	if err := db.Where("team_id <> ? AND connection_id = ?", "", "platform").Find(&legacyTeamSpaces).Error; err != nil {
		return nil, err
	}
	for _, sp := range legacyTeamSpaces {
		a := TeamStorage{TeamID: sp.TeamID, TenantID: sp.TenantID, ConnectionID: sp.ConnectionID}
		if err := db.Where("team_id = ? AND connection_id = ?", a.TeamID, a.ConnectionID).FirstOrCreate(&a).Error; err != nil {
			return nil, err
		}
	}
	if e = db.AutoMigrate(&ServiceIdentityBinding{}, &ServiceAdmission{}, &Blob{}, &UploadBatch{}, &UploadBatchItem{}, &UploadSession{}, &RetentionHold{}, &RetentionPolicy{}, &RetentionPlan{}, &RetentionItem{}, &FileThread{}, &ThreadMessage{}, &PrivateNote{}, &DiscussionRevision{}, &CloudRoot{}, &ExternalEntry{}, &TransferPlan{}, &TransferItem{}, &AnalyticsSnapshot{}, &ContentPolicy{}, &ContentBinding{}, &DeliveryManifest{}, &DeliveryManifestItem{}, &DeliveryReceipt{}, &OrganizationPolicy{}, &OrganizationPlan{}, &OrganizationPlanItem{}, &ChangeSubscription{}, &AutomationRule{}, &AutomationIntent{}, &FileLabelAssignment{}, &FileCollection{}, &CollectionMember{}, &CollectionSnapshot{}, &CollectionSnapshotEntry{}, &SavedFileQuery{}); e != nil {
		s.Close()
		return nil, code("database_unavailable")
	}
	if e = db.AutoMigrate(&ShareSession{}, &ShareDevice{}, &ShareDownload{}); e != nil {
		s.Close()
		return nil, code("database_unavailable")
	}
	if e = migrateAutomationIntentIdentity(db); e != nil {
		s.Close()
		return nil, code("database_unavailable")
	}
	if e = migrateWorkspaceModels(db); e != nil {
		s.Close()
		return nil, code("database_unavailable")
	}
	if e = s.ConfigureServiceIdentity(); e != nil {
		s.Close()
		return nil, e
	}
	var governance GovernanceState
	if err := db.First(&governance, 1).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.Close()
		return nil, code("database_unavailable")
	}
	s.governance.Store(governance.Active)
	if err := db.Model(&AccessRequest{}).Where("status = ?", "executing").Update("status", "interrupted").Error; err != nil {
		s.Close()
		return nil, err
	}
	s.previewCtx, s.previewCancel = context.WithCancel(context.Background())
	s.previewSlots = make(chan struct{}, 2)
	db.Model(&Preview{}).Where("status = ?", "processing").Updates(map[string]any{"status": "failed", "error": "preview_interrupted"})
	s.Insights, e = insights.Open(db)
	if e != nil {
		s.Close()
		return nil, e
	}
	if e = s.applyTenantProfile(); e != nil {
		s.Close()
		return nil, e
	}
	if e = db.Model(&Operation{}).Where("status IN ?", []string{"verifying", "transferring"}).Updates(map[string]any{"status": "interrupted", "error": "resume_required"}).Error; e != nil {
		s.Close()
		return nil, e
	}
	if e = db.Model(&Operation{}).Where("status = ? AND kind IN ?", "running", []string{"directory_copy", "directory_export"}).Updates(map[string]any{"status": "interrupted", "error": "resume_required"}).Error; e != nil {
		s.Close()
		return nil, e
	}
	if e = s.repairParts(); e != nil {
		s.Close()
		return nil, e
	}
	if e = s.reconcileServiceAdmissions(); e != nil {
		s.Close()
		return nil, e
	}
	if e = s.migrateTeamRuntime(); e != nil {
		s.Close()
		return nil, e
	}
	if e = s.reconcileTeamRuntime(); e != nil {
		s.Close()
		return nil, e
	}
	if e = s.backfillTrashRetention(); e != nil {
		s.Close()
		return nil, e
	}
	s.startTrashSweep()
	s.startRuntimeWorker()
	success = true
	return s, nil
}
func (s *Service) Close() {
	if s.runtimeCancel != nil {
		s.runtimeCancel()
	}
	s.runtimeWG.Wait()
	if s.sweepCancel != nil {
		s.sweepCancel()
	}
	s.sweepWG.Wait()
	if s.previewCancel != nil {
		s.previewCancel()
	}
	s.previewWG.Wait()
	if s.insightCancel != nil {
		s.insightCancel()
	}
	s.insightWG.Wait()
	if s.Insights != nil {
		s.Insights.Close()
	}
	db, _ := s.DB.DB()
	db.Close()
	s.lock.Unlock()
}
func (s *Service) seal(owner string, v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, s.crypt.NonceSize())
	rand.Read(nonce)
	return s.crypt.Seal(nonce, nonce, b, []byte(owner)), nil
}
func (s *Service) unseal(owner string, b []byte, v any) error {
	n := s.crypt.NonceSize()
	if len(b) < n {
		return code("credential_unavailable")
	}
	plain, e := s.crypt.Open(nil, b[:n], b[n:], []byte(owner))
	if e != nil {
		return code("credential_unavailable")
	}
	if json.Unmarshal(plain, v) != nil {
		return code("credential_unavailable")
	}
	return nil
}
func cleanPath(p string) (string, error) {
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") || len(p) > 1024 {
		return "", code("invalid_path")
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return "", code("invalid_path")
		}
		for _, r := range part {
			if r < 32 {
				return "", code("invalid_path")
			}
		}
	}
	return path.Clean(p), nil
}
func contains(prefix, p string) bool {
	return prefix == "" || p == prefix || strings.HasPrefix(p, prefix+"/")
}
func (s *Service) allow(p Principal, space, filepath, action string) error {
	// Pool-bound default; code running inside an open transaction must use
	// allowOn with that transaction instead (single-connection SQLite
	// deadlocks if a pooled query interleaves an open transaction).
	return s.allowOn(s.DB, p, space, filepath, action)
}

// allowOn is the transaction-bound authorization check: every lookup it
// needs runs on the caller's handle so it can never wait on a connection
// the caller already holds.
func (s *Service) allowOn(db *gorm.DB, p Principal, space, filepath, action string) error {
	// Staging is not part of a legacy Agent's inherited read/write scope.
	if workspaceStagingPath(filepath) && (p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "") {
		return code("forbidden")
	}
	if p.ShareID != "" {
		var grant Share
		if db.First(&grant, "id = ? AND revoked = ?", p.ShareID, false).Error != nil || !shareActive(grant) {
			return code("share_unavailable")
		}
	}
	if p.AccessGrantHash != "" {
		var grant AccessGrant
		if db.First(&grant, "hash = ?", p.AccessGrantHash).Error != nil || !time.Now().Before(grant.Expires) {
			return code("unauthorized")
		}
	}

	if err := s.governanceAllow(db, p, space, filepath, action); err != nil {
		return err
	}
	if p.UserID == "" {
		return code("unauthorized")
	}
	if p.Agent != nil {
		if err := rejectWorkspaceLegacyAccess(db, *p.Agent); err != nil {
			return err
		}
		var current Agent
		if db.First(&current, "id = ? AND owner = ?", p.Agent.ID, p.UserID).Error != nil || current.RestrictionProfile != "" || current.Revoked || !time.Now().Before(current.Expires) || current.SpaceID != space || !contains(current.Prefix, filepath) || !strings.Contains(","+current.Scopes+",", ","+action+",") {
			return code("forbidden")
		}
	}
	return nil
}
func (s *Service) space(p Principal, id string) (Space, error) {
	var v Space
	if s.DB.First(&v, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return v, code("not_found")
	}
	if err := s.governanceAllow(s.DB, p, id, "", "read"); err != nil {
		return v, err
	}
	return v, nil
}
func (s *Service) file(p Principal, id, action string) (File, error) {
	var f File
	if s.DB.First(&f, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return f, code("not_found")
	}
	if e := s.allow(p, f.SpaceID, f.Path, action); e != nil {
		return File{}, e
	}
	return f, nil
}
func (s *Service) operation(p Principal, id, action string) (Operation, error) {
	var op Operation
	if s.DB.First(&op, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return op, code("not_found")
	}
	if op.Purpose != "" || op.WorkspaceGrantID != "" {
		return Operation{}, code("forbidden")
	}
	if s.governanceActive() && action != "read" && op.ActorID != "" && op.ActorID != p.UserID {
		return Operation{}, code("forbidden")
	}
	if action == "transfer" {
		action = "write"
		if op.Kind == "export" {
			action = "export"
			if p.Agent != nil && p.Agent.ConnectionID != op.ExternalConnectionID {
				return Operation{}, code("forbidden")
			}
		}
	}
	if e := s.allow(p, op.SpaceID, op.Path, action); e != nil {
		return Operation{}, e
	}
	return op, nil
}
func (s *Service) audit(p Principal, action, id, outcome string) {
	if len(id) != 48 {
		id = ""
	} else if _, err := hex.DecodeString(id); err != nil {
		id = ""
	}
	agent := ""
	if p.Agent != nil {
		agent = p.Agent.ID
	}
	s.DB.Create(&Audit{ActorID: p.UserID, TenantID: p.TenantID, Owner: p.dataOwner(), AgentID: agent, Action: action, Resource: id, Outcome: outcome, At: time.Now().UTC()})
}
func (s *Service) FileRef(f File) string {
	return "drivebridge://" + s.Instance + "/file/" + f.ID + "/version/" + f.VersionID
}

// resolveRef parses a drivebridge://<instance>/file/<id>/version/<version>
// reference for local use. References carrying a foreign instance identity
// are rejected without revealing whether anything exists here or there.
func (s *Service) resolveRef(ref string) (string, string, error) {
	if ref == "" {
		return "", "", code("invalid_ref")
	}
	u, e := url.Parse(ref)
	if e != nil || u.Scheme != "drivebridge" || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", code("invalid_ref")
	}
	if u.Host != s.Instance {
		return "", "", code("cross_instance_rejected")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "file" || parts[2] != "version" || len(parts[1]) != 48 || len(parts[3]) != 48 {
		return "", "", code("invalid_ref")
	}
	if _, e = hex.DecodeString(parts[1]); e != nil {
		return "", "", code("invalid_ref")
	}
	if _, e = hex.DecodeString(parts[3]); e != nil {
		return "", "", code("invalid_ref")
	}
	return parts[1], parts[3], nil
}
