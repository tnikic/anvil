// Command generate renders SKILL.md from internal/content and writes it
// to the specified output path. Invoked via `go generate`.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tnikic/anvil/internal/skillgen"
)

func main() {
	outPath := flag.String("o", "SKILL.md", "output path for SKILL.md")
	flag.Parse()

	skill, err := skillgen.Render()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	dir := filepath.Dir(*outPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error: creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	if err := os.WriteFile(*outPath, []byte(skill), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", *outPath, err)
		os.Exit(1)
	}

	fmt.Printf("generated: %s\n", *outPath)
}
