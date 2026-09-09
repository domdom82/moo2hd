// Command lbxextract prints a table of records in LBX archive files and
// optionally extracts their payloads to disk.
//
// Usage:
//
//	lbxextract [--extract] <file.lbx> [file2.lbx ...]
//
// Flags:
//
//	--extract   Write each record's payload to disk as <archive>_<index>.<ext>
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/domdom82/moo2hd/internal/lbx"
)

func main() {
	extract := flag.Bool("extract", false, "write record payloads to disk")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lbxextract [--extract] <file.lbx> [...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	exitCode := 0
	for _, path := range flag.Args() {
		if err := processFile(path, *extract); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func processFile(path string, doExtract bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	arc, err := lbx.Parse(data)
	if err != nil {
		return err
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	fmt.Printf("=== %s  (%s, %d records, moo2=%v) ===\n",
		filepath.Base(path), arc.ArchiveType, len(arc.Records), arc.IsMoO2)
	fmt.Printf("%-5s  %-6s  %-8s  %-10s  %s\n", "IDX", "TYPE", "SIZE", "NAME", "DESC")
	fmt.Println(strings.Repeat("-", 60))

	for _, r := range arc.Records {
		fmt.Printf("%-5d  %-6s  %-8d  %-10s  %s\n",
			r.Index, r.Type, len(r.Data), r.Name, r.Desc)
	}
	fmt.Println()

	if !doExtract {
		return nil
	}

	for _, r := range arc.Records {
		outName := fmt.Sprintf("%s_%03d.%s", base, r.Index, strings.ToLower(r.Type.String()))
		if err := os.WriteFile(outName, r.Data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outName, err)
		}
		fmt.Printf("  wrote %s (%d bytes)\n", outName, len(r.Data))
	}

	return nil
}
