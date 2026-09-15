package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	dbconfig "github.com/yeisme/drivebridge/internal/config"
)

func configSetCommand(c *cli) *cobra.Command {
	return &cobra.Command{Use: "set <key> <value>", Short: "Persist a supported non-secret configuration value.", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !slices.Contains(dbconfig.Keys, args[0]) {
			return fmt.Errorf("unknown_config_key")
		}
		target := c.effective.Path
		if target == "" {
			home, err := os.UserConfigDir()
			if err != nil {
				return err
			}
			target = filepath.Join(home, "drivebridge", "config.yaml")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		v := viper.New()
		for k, value := range c.file.Values {
			v.Set(k, value)
		}
		v.Set(args[0], args[1])
		ext := filepath.Ext(target)
		v.SetConfigType(ext[1:])
		f, err := os.CreateTemp(filepath.Dir(target), ".config-*"+ext)
		if err != nil {
			return err
		}
		name := f.Name()
		f.Close()
		defer os.Remove(name)
		if err = v.WriteConfigAs(name); err != nil {
			return fmt.Errorf("config_write_failed")
		}
		if err = os.Chmod(name, 0600); err != nil {
			return err
		}
		if err = os.Rename(name, target); err != nil {
			return err
		}
		return c.mode.write("config.set", "success", "Configuration updated.", map[string]any{"path": target, "key": args[0]})
	}}
}
