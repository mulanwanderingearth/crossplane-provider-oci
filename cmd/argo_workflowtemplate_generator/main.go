package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/oracle/provider-oci/internal/argo/workflowtemplategenerator"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var services stringSliceFlag
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	outputDir := fs.String("output-dir", "", "Override output directory for generated templates")
	fs.Var(&services, "service", "Service to generate (repeatable)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: go run main.go [flags] <version>")
	}

	version := strings.TrimSpace(fs.Arg(0))
	if version == "" {
		return fmt.Errorf("version must not be empty")
	}

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg := workflowtemplategenerator.NewConfig(rootDir, version)
	cfg.Services = services
	if out := strings.TrimSpace(*outputDir); out != "" {
		if !filepath.IsAbs(out) {
			out = filepath.Join(rootDir, out)
		}
		cfg.OutputDir = out
	}

	return workflowtemplategenerator.Run(cfg)
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*s = append(*s, value)
	return nil
}
