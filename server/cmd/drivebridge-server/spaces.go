package main

import "github.com/spf13/cobra"

func newSpacesCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "spaces", Short: "Maintain spaces on a stopped deployment"}
	var ids []string
	var stopped bool
	retire := &cobra.Command{Use: "retire", Short: "Remove exact spaces from active use, revoke access and cancel pending uploads; retain recoverable bytes", RunE: func(cmd *cobra.Command, _ []string) error {
		if !stopped {
			return fail("offline_required", "Stop the HTTP service and workers, take a backup, then pass --offline.")
		}
		s, err := c.openService(false, false)
		if err != nil {
			return err
		}
		defer s.Close()
		if err := s.RetireSpaces(ids); err != nil {
			return err
		}
		return c.mode.emitData("spaces.retire", "success", "Spaces removed from active use. Version bytes and audit history are retained for recovery.", map[string]any{"space_ids": ids, "purged": false})
	}}
	retire.Flags().StringSliceVar(&ids, "id", nil, "Exact space IDs to retire")
	retire.Flags().BoolVar(&stopped, "offline", false, "Confirm the service and workers are stopped and a backup exists")
	_ = retire.MarkFlagRequired("id")
	root.AddCommand(retire)
	return root
}
