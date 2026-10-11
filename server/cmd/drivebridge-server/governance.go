package main

import (
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func newGovernanceCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "governance", Short: "Manage deployment-side tenant governance."}
	var user string
	bootstrap := &cobra.Command{Use: "bootstrap", Short: "Migrate existing data to the default tenant and explicitly appoint a platform administrator.", RunE: func(cmd *cobra.Command, args []string) error {
		s, e := c.openService(false, false)
		if e != nil {
			return e
		}
		defer s.Close()
		if e = s.BootstrapGovernance(user); e != nil {
			return e
		}
		return c.mode.emitData("governance.bootstrap", "success", "Governance enabled; legacy administrator flags do not grant platform authority.", map[string]any{"tenant_id": s.Instance, "platform_admin": user})
	}}
	bootstrap.Flags().StringVar(&user, "user", "", "Existing user ID to appoint (also restores a suspended platform role)")
	_ = bootstrap.MarkFlagRequired("user")
	root.AddCommand(bootstrap)
	var binding service.ServiceIdentityBinding
	link := &cobra.Command{Use: "identity-link", Short: "Link an exact unified identity to existing local data ownership.", RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := c.openService(false, false)
		if err != nil {
			return err
		}
		defer s.Close()
		if err := s.BindServiceIdentity(binding); err != nil {
			return err
		}
		return c.mode.emitData("governance.identity-link", "success", "Unified identity mapping recorded.", map[string]any{"user_id": binding.UserID, "tenant_id": binding.TenantID, "cutover": binding.Cutover})
	}}
	link.Flags().StringVar(&binding.Issuer, "issuer", "", "Exact HTTPS identity issuer")
	link.Flags().StringVar(&binding.Subject, "subject", "", "Stable identity subject reference")
	link.Flags().StringVar(&binding.IdentityTenant, "identity-tenant", "", "Identity Platform tenant reference")
	link.Flags().StringVar(&binding.UserID, "user", "", "Existing local user ID")
	link.Flags().StringVar(&binding.TenantID, "tenant", "", "Existing local tenant ID")
	link.Flags().StringVar(&binding.AgentID, "agent-id", "", "Existing restricted Agent ID for a service-account mapping")
	link.Flags().BoolVar(&binding.Cutover, "cutover", false, "Reject legacy credentials after all tenant members are mapped")
	for _, name := range []string{"issuer", "subject", "identity-tenant", "user", "tenant"} {
		_ = link.MarkFlagRequired(name)
	}
	root.AddCommand(link)
	return root
}
