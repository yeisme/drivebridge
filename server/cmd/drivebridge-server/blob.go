package main

import (
	"context"

	"github.com/spf13/cobra"
)

func newBlobCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "blob", Short: "Inspect content-addressed blob records."}
	stat := &cobra.Command{Use: "stat <sha256>", Short: "Report safe metadata for one plaintext digest.", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := c.openService(false, false)
		if err != nil {
			return err
		}
		defer s.Close()
		blobs, err := s.BlobStat(args[0])
		if err != nil {
			return err
		}
		rows := make([]map[string]any, 0, len(blobs))
		for _, b := range blobs {
			rows = append(rows, map[string]any{"sha256": b.SHA256, "size_bytes": b.Size, "ref_count": b.Refs, "encrypted": b.Encrypted, "space_id": b.SpaceID, "object_key": b.ObjectKey})
		}
		return c.mode.emitData("blob.stat", "success", "Blob records carry metadata only; no bytes or credentials are shown.", map[string]any{"blobs": rows, "count": len(rows)})
	}}
	gc := &cobra.Command{Use: "gc", Short: "Report reference-count-zero blobs; deletion requires --yes.", RunE: func(cmd *cobra.Command, _ []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		s, err := c.openService(false, false)
		if err != nil {
			return err
		}
		defer s.Close()
		out, err := s.BlobGC(context.Background(), !yes)
		if err != nil {
			return err
		}
		summary := "Dry run: no objects deleted."
		if yes {
			summary = "Unreferenced blob objects removed."
		}
		return c.mode.emitData("blob.gc", "success", summary, out)
	}}
	gc.Flags().Bool("yes", false, "Actually delete unreferenced objects; without it gc is a dry run")
	root.AddCommand(stat, gc)
	return root
}
