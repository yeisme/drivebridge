package service_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
)

// Real-provider integration, opt-in only: credentials arrive exclusively via
// COS_ACCESS_KEY/COS_SECRET_KEY and never appear in source, logs, or evidence.
// COS_*_KEY, COS_ENDPOINT, COS_BUCKET, COS_REGION, COS_PREFIX may be set;
// defaults target a Tencent COS bucket in ap-guangzhou that rejects
// path-style addressing, exercising the virtual_hosted path.
func TestCOSPlatformStorageReal(t *testing.T) {
	ak, sk := os.Getenv("COS_ACCESS_KEY"), os.Getenv("COS_SECRET_KEY")
	if ak == "" || sk == "" {
		t.Skip("COS_ACCESS_KEY/COS_SECRET_KEY not set; real-provider integration is opt-in")
	}
	endpoint := envDefault("COS_ENDPOINT", "https://cos.ap-guangzhou.myqcloud.com")
	bucket := envDefault("COS_BUCKET", "drivebridge-1384001538")
	region := envDefault("COS_REGION", "ap-guangzhou")
	prefix := envDefault("COS_PREFIX", "_cos-test")
	cfg := service.S3Config{Endpoint: endpoint, Region: region, Bucket: bucket, Prefix: prefix, AccessKey: ak, SecretKey: sk, VirtualHosted: true}
	st, e := service.NewS3(cfg, false)
	if e != nil {
		t.Fatalf("NewS3: %v", e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if e = st.Check(ctx); e != nil {
		t.Fatalf("HeadBucket via virtual-hosted addressing: %v", e)
	}
	key := "cos-storage-test/" + time.Now().UTC().Format("20060102T150405Z") + ".txt"
	body := "drivebridge cos integration\n"
	if e = st.Put(ctx, key, strings.NewReader(body), int64(len(body))); e != nil {
		t.Fatalf("Put: %v", e)
	}
	if e = st.Stat(ctx, key); e != nil {
		t.Fatalf("Stat: %v", e)
	}
	r, e := st.Get(ctx, key)
	if e != nil {
		t.Fatalf("Get: %v", e)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if string(got) != body {
		t.Fatalf("read-back mismatch: %q", got)
	}
	if _, e = st.(prober).ProbeIsolation(ctx); e != nil {
		t.Fatalf("ProbeIsolation against real provider: %v", e)
	}
	if e = st.Delete(ctx, key); e != nil {
		t.Fatalf("Delete: %v", e)
	}
}

func envDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
