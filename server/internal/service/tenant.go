package service

import (
	"net/url"
	"time"
)

// applyTenantProfile records the deployment-as-tenant profile from startup
// parameters. The profile is metadata only: no online surface can modify it;
// changing it means stopping the deployment and starting with new parameters.
func (s *Service) applyTenantProfile() error {
	c := s.Config
	if c.TenantName == "" && c.TenantOrigin == "" && c.TenantContact == "" {
		return nil
	}
	if len(c.TenantName) > 120 || len(c.TenantContact) > 200 {
		return code("invalid_tenant_config")
	}
	if c.TenantOrigin != "" {
		u, e := url.Parse(c.TenantOrigin)
		if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
			return code("invalid_tenant_config")
		}
	}
	profile := TenantProfile{ID: "tenant", Name: c.TenantName, Origin: c.TenantOrigin, Contact: c.TenantContact, Updated: time.Now().UTC()}
	return s.DB.Save(&profile).Error
}

// Tenant returns the deployment tenant profile, or nil when never configured.
func (s *Service) Tenant() *TenantProfile {
	var profile TenantProfile
	if s.DB.First(&profile, "id = ?", "tenant").Error != nil {
		return nil
	}
	return &profile
}
