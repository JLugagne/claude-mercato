package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JLugagne/agents-mercato/internal/mercato"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	configPath := filepath.Join(home, ".config", "mct", "config.yml")
	cacheDir := filepath.Join(home, ".cache", "mct")
	rootCmd := mercato.NewApp(configPath, cacheDir)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}
