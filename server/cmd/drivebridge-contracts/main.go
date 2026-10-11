// Command drivebridge-contracts emits deterministic public contract schemas.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

func run(args []string, out io.Writer) error {
	profile := "personal-workspace"
	if len(args) == 3 && args[1] == "--profile" {
		profile, args = args[2], args[:1]
	}
	if len(args) != 1 || (profile != "personal-workspace" && profile != "desktop") {
		return fmt.Errorf("usage: drivebridge-contracts response-schemas|openapi|fixtures [--profile personal-workspace|desktop]")
	}
	if profile == "desktop" {
		data, err := desktopcontract.Encode(args[0])
		if err != nil {
			return err
		}
		_, err = out.Write(append(data, '\n'))
		return err
	}
	var data []byte
	var err error
	switch args[0] {
	case "response-schemas":
		data, err = contract.EncodeResponseSchemas()
	case "openapi":
		data, err = contract.EncodeOpenAPI()
	case "fixtures":
		data, err = contract.EncodeResponseFixtures()
	default:
		return fmt.Errorf("unknown contract output: %s", args[0])
	}
	if err != nil {
		return err
	}
	_, err = out.Write(append(data, '\n'))
	return err
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
