package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awshttp "github.com/aws/smithy-go/transport/http"
	"github.com/yeisme/drivebridge-server/sdk/insights"
)

type S3Config struct {
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	SessionToken string `json:"session_token,omitempty"`
	// VirtualHosted addresses the bucket as a subdomain of Endpoint for
	// providers that reject path-style addressing (e.g. Tencent COS). The
	// zero value keeps the legacy path-style behavior so sealed connections
	// migrate without rewrite.
	VirtualHosted bool `json:"virtual_hosted,omitempty"`
}
type Storage interface {
	Put(context.Context, string, io.Reader, int64) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Check(context.Context) error
	Stat(context.Context, string) error
}
type StorageFactory func(S3Config, bool) (Storage, error)
type s3Store struct {
	client *s3.Client
	cfg    S3Config
}

func NewS3(c S3Config, development bool) (Storage, error) {
	u, e := url.Parse(c.Endpoint)
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Host == "" || (u.Scheme != "https" && !(development && u.Scheme == "http" && loopback(u.Hostname()))) || c.Bucket == "" || strings.ContainsAny(c.Bucket, "/\\") || c.AccessKey == "" || c.SecretKey == "" {
		return nil, code("invalid_storage_config")
	}
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	if c.Prefix != "" {
		if _, e = cleanPath(c.Prefix); e != nil {
			return nil, e
		}
	}
	transport := &http.Transport{Proxy: nil, DialContext: safeDial(development), TLSHandshakeTimeout: 10 * time.Second}
	client := s3.New(s3.Options{Region: c.Region, BaseEndpoint: aws.String(strings.TrimSuffix(c.Endpoint, "/")), UsePathStyle: !c.VirtualHosted, Credentials: credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.SessionToken), HTTPClient: &http.Client{Transport: transport, Timeout: 30 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired, ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired})
	return &s3Store{client, c}, nil
}
func (s *s3Store) key(k string) string {
	if s.cfg.Prefix == "" {
		return k
	}
	return s.cfg.Prefix + "/" + k
}
func (s *s3Store) Put(ctx context.Context, k string, r io.Reader, n int64) error {
	_, e := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(s.key(k)), Body: r, ContentLength: aws.Int64(n)})
	if e != nil {
		return code("storage_write_failed")
	}
	return nil
}
func (s *s3Store) Get(ctx context.Context, k string) (io.ReadCloser, error) {
	out, e := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(s.key(k))})
	if e != nil {
		return nil, code("storage_read_failed")
	}
	return out.Body, nil
}
func (s *s3Store) Delete(ctx context.Context, k string) error {
	_, e := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(s.key(k))})
	if e != nil {
		return code("storage_delete_failed")
	}
	return nil
}

// Stat probes object existence without reading bytes or exposing authority.
func (s *s3Store) Stat(ctx context.Context, k string) error {
	_, e := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(s.key(k))})
	if e != nil {
		return code("object_missing")
	}
	return nil
}
func (s *s3Store) Check(ctx context.Context) error {
	_, e := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.cfg.Bucket)})
	if e != nil {
		return code("storage_unavailable")
	}
	return nil
}
func loopback(h string) bool {
	ip := net.ParseIP(h)
	return h == "localhost" || (ip != nil && ip.IsLoopback())
}
func safeDial(development bool) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		h, p, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, h)
		if e != nil || len(ips) == 0 {
			return nil, code("dns_unavailable")
		}
		for _, v := range ips {
			if development && v.IP.IsLoopback() {
				continue
			}
			if !v.IP.IsGlobalUnicast() || v.IP.IsPrivate() || v.IP.IsLoopback() || v.IP.IsLinkLocalUnicast() {
				return nil, code("endpoint_not_allowed")
			}
		}
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), p))
	}
}
func (s *Service) storageFor(owner, id string) (Storage, error) {
	var conn Connection
	if s.DB.First(&conn, "id = ? AND (owner = ? OR owner = ?)", id, owner, "").Error != nil || conn.Disabled || conn.Kind != "s3" {
		return nil, code("storage_disconnected")
	}
	var c S3Config
	if e := s.unseal(conn.Owner, conn.Secret, &c); e != nil {
		return nil, e
	}
	return s.storage(c, s.Config.Development)
}
func (s *Service) ConnectS3(ctx context.Context, p Principal, name string, c S3Config) (Connection, error) {
	if p.Agent != nil {
		return Connection{}, code("forbidden")
	}
	st, e := s.storage(c, s.Config.Development)
	if e != nil {
		return Connection{}, e
	}
	if e = st.Check(ctx); e != nil {
		return Connection{}, e
	}
	secret, e := s.seal(p.dataOwner(), c)
	if e != nil {
		return Connection{}, e
	}
	// ConnectS3 verified the credential with a real probe before sealing it,
	// so the connection is both configured and observed from creation.
	now := time.Now().UTC()
	v := Connection{ID: ID(), Owner: p.dataOwner(), Kind: "s3", Name: name, Secret: secret, Status: "verified", ObservedAt: &now, ObservedStatus: "verified"}
	if e = s.DB.Create(&v).Error; e != nil {
		return v, code("database_unavailable")
	}
	return v, nil
}
func (s *Service) SetPlatform(ctx context.Context, c S3Config) error {
	if s.Insights != nil {
		if e := s.Insights.Log(insights.AuditEvent{Actor: "local-operator", ActorKind: "instance_operator", Action: "connections.platform", Outcome: "intent", Entry: "cli"}); e != nil {
			return e
		}
	}
	st, e := s.storage(c, s.Config.Development)
	if e != nil {
		return e
	}
	if e = st.Check(ctx); e != nil {
		return e
	}
	secret, e := s.seal("", c)
	if e != nil {
		return e
	}
	observed := time.Now().UTC()
	v := Connection{ID: "platform", Kind: "s3", Name: "平台存储", Secret: secret, Status: "verified", ObservedAt: &observed, ObservedStatus: "verified"}
	var old Connection
	if s.DB.First(&old, "id = ?", "platform").Error == nil {
		return code("platform_storage_already_configured")
	}
	return s.DB.Create(&v).Error
}

// Inventory enumerates only the owner-selected prefix. Provider version history
// and account billing are deliberately outside this observation.
func (s *s3Store) Inventory(ctx context.Context, prefix, cursor string) (insights.InventoryPage, error) {
	in := &s3.ListObjectsV2Input{Bucket: aws.String(s.cfg.Bucket), Prefix: aws.String(s.key(prefix)), MaxKeys: aws.Int32(1000)}
	if cursor != "" {
		in.ContinuationToken = aws.String(cursor)
	}
	out, err := s.client.ListObjectsV2(ctx, in)
	if err != nil {
		return insights.InventoryPage{}, code("inventory_unavailable")
	}
	page := insights.InventoryPage{Entries: int64(len(out.Contents)), Resumable: true}
	for _, item := range out.Contents {
		if item.Size != nil && *item.Size > 0 {
			page.Bytes += *item.Size
		}
	}
	if aws.ToBool(out.IsTruncated) {
		page.Next = aws.ToString(out.NextContinuationToken)
		if page.Next == "" {
			return page, code("invalid_inventory")
		}
	}
	return page, nil
}

// isolationProber is implemented by stores that can verify whether
// provider-side policy narrows the credential to this deployment's own object
// namespace. Keeping it optional means test fakes of Storage stay unchanged.
type isolationProber interface {
	ProbeIsolation(context.Context) (bool, error)
}

// VerifyStorageIsolation reports whether a storage credential is narrowed to
// the deployment namespace by provider policy. It writes only inside reserved
// probe namespaces and deletes anything it manages to create.
func (s *Service) VerifyStorageIsolation(ctx context.Context, c S3Config) (bool, error) {
	st, e := s.storage(c, s.Config.Development)
	if e != nil {
		return false, e
	}
	p, ok := st.(isolationProber)
	if !ok {
		return false, code("isolation_probe_unavailable")
	}
	return p.ProbeIsolation(ctx)
}

var isolationProbeBody = []byte("drivebridge isolation probe\n")

// ProbeIsolation attempts two writes outside this deployment's object
// namespace and expects both denied: one in the bucket-root reserved
// namespace, one sibling-shaped under the configured prefix. An unexpected
// success is deleted immediately and reported as over-scoped; no real tenant
// namespace is ever touched.
func (s *s3Store) ProbeIsolation(ctx context.Context) (bool, error) {
	sibling := ID()
	if s.cfg.Prefix != "" {
		sibling = s.cfg.Prefix + "/" + sibling
	}
	for _, key := range []string{"_isolation-probe/" + ID(), sibling + "/" + ID()} {
		_, e := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), Body: bytes.NewReader(isolationProbeBody), ContentLength: aws.Int64(int64(len(isolationProbeBody)))})
		if e == nil {
			if _, d := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)}); d != nil {
				return false, code("probe_cleanup_failed")
			}
			return false, nil
		}
		var re *awshttp.ResponseError
		if !errors.As(e, &re) {
			return false, e
		}
		if sc := re.HTTPStatusCode(); sc != 401 && sc != 403 {
			return false, e
		}
	}
	return true, nil
}

func (s *s3Store) GetRange(ctx context.Context, key string, start, end int64) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(s.key(key)), Range: aws.String(fmt.Sprintf("bytes=%d-%d", start, end))})
	if err != nil {
		return nil, code("storage_read_failed")
	}
	return out.Body, nil
}
