package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/url"
	"os"
)

// CloneDevelopmentPlatform is an operator-only recovery step on a checkpointed
// development state. It preserves object keys and verifies every version before
// changing the platform connection. It is never exposed through HTTP.
func (s *Service) CloneDevelopmentPlatform(ctx context.Context, cfg S3Config, reuse bool) (int, error) {
	if !s.Config.Development {
		return 0, code("forbidden")
	}
	var old Connection
	if e := s.DB.First(&old, "id = ?", "platform").Error; e != nil {
		return 0, e
	}
	var previous S3Config
	if e := s.unseal(old.Owner, old.Secret, &previous); e != nil {
		return 0, e
	}
	for _, endpoint := range []string{previous.Endpoint, cfg.Endpoint} {
		u, e := url.Parse(endpoint)
		if e != nil || !loopback(u.Hostname()) {
			return 0, code("endpoint_not_allowed")
		}
	}
	source, e := NewS3(previous, true)
	if e != nil {
		return 0, e
	}
	target, e := NewS3(cfg, true)
	if e != nil {
		return 0, e
	}
	if reuse {
		source = target
	}
	var versions []Version
	if e = s.DB.Find(&versions).Error; e != nil {
		return 0, e
	}
	for _, v := range versions {
		if v.ConnectionID != "platform" {
			return 0, code("checkpoint_platform_only")
		}
		body, err := source.Get(ctx, v.ObjectKey)
		if err != nil {
			return 0, err
		}
		f, err := os.CreateTemp(s.Config.State, "checkpoint-object-")
		if err != nil {
			body.Close()
			return 0, err
		}
		n, err := io.Copy(f, body)
		body.Close()
		if err == nil {
			_, err = f.Seek(0, 0)
		}
		if err == nil {
			err = target.Put(ctx, v.ObjectKey, f, n)
		}
		f.Close()
		os.Remove(f.Name())
		if err != nil {
			return 0, err
		}
		restored, err := target.Get(ctx, v.ObjectKey)
		if err != nil {
			return 0, err
		}
		var file File
		var space Space
		if s.DB.First(&file, "id = ?", v.FileID).Error != nil || s.DB.First(&space, "id = ?", file.SpaceID).Error != nil {
			restored.Close()
			return 0, code("not_found")
		}
		var plain io.Reader = restored
		if space.Encrypted {
			aead, e := s.spaceContentAEAD(space)
			if e != nil {
				restored.Close()
				return 0, e
			}
			plain, e = newContentOpener(aead, restored, v.Size)
			if e != nil {
				restored.Close()
				return 0, e
			}
		}
		digest := sha256.New()
		size, err := io.Copy(digest, plain)
		restored.Close()
		if err != nil || size != v.Size || hex.EncodeToString(digest.Sum(nil)) != v.SHA256 {
			return 0, code("checkpoint_content_mismatch")
		}
	}
	secret, e := s.seal(old.Owner, cfg)
	if e != nil {
		return 0, e
	}
	if e = s.DB.Model(&old).Update("secret", secret).Error; e != nil {
		return 0, e
	}
	return len(versions), nil
}
