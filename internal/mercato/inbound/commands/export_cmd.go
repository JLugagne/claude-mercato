package commands

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// ExportData is the portable format for sharing mct configurations.
type ExportData struct {
	Version int            `json:"version"`
	Markets []ExportMarket `json:"markets"`
	Entries []ExportEntry  `json:"entries"`
}

type ExportMarket struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Branch   string `json:"branch"`
	Trusted  bool   `json:"trusted,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type ExportEntry struct {
	Profile string `json:"profile"`
}

func newExportCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [file]",
		Short: "Export all markets and installed entries to JSON",
		Long:  "Serialize all registered markets, installed entries, and configuration into a portable JSON file.\nIf no file is given, outputs to stdout.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := svc.Config.GetConfig()
			if err != nil {
				return err
			}

			installed, err := svc.Entries.List(service.ListOpts{Installed: true})
			if err != nil {
				return err
			}

			export := ExportData{Version: 1}

			// Entries: group by profile (market/seg1/seg2), only installed ones
			profiles := make(map[string]struct{})
			for _, e := range installed {
				profiles[e.Profile] = struct{}{}
			}

			// Markets: only those referenced by installed entries
			usedMarkets := make(map[string]struct{})
			for p := range profiles {
				if market := domain.MctRef(p).Market(); market != "" {
					usedMarkets[market] = struct{}{}
				}
			}
			for _, mc := range cfg.Markets {
				if _, ok := usedMarkets[mc.Name]; !ok {
					continue
				}
				export.Markets = append(export.Markets, ExportMarket{
					Name:     mc.Name,
					URL:      mc.URL,
					Branch:   mc.Branch,
					Trusted:  mc.Trusted,
					ReadOnly: mc.ReadOnly,
				})
			}
			sorted := make([]string, 0, len(profiles))
			for p := range profiles {
				sorted = append(sorted, p)
			}
			sort.Strings(sorted)
			for _, p := range sorted {
				export.Entries = append(export.Entries, ExportEntry{Profile: p})
			}

			data, err := json.MarshalIndent(export, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal export: %w", err)
			}

			if len(args) == 1 {
				if err := os.WriteFile(args[0], append(data, '\n'), 0644); err != nil {
					return err
				}
				cmd.Printf("  ok  exported to %s\n", args[0])
				return nil
			}

			cmd.Println(string(data))
			return nil
		},
	}
	return cmd
}

func newImportCmd(svc Services, opts *GlobalOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import markets and entries from a JSON export",
		Long:  "Read a previously exported JSON file and register all markets and install all entries.\nMarkets with the same URL are skipped (already registered).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			jsonOut, _ := cmd.Flags().GetBool("json")
			yes, _ := cmd.Flags().GetBool("yes")

			raw, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			var imp ExportData
			if err := json.Unmarshal(raw, &imp); err != nil {
				return fmt.Errorf("invalid export file: %w", err)
			}

			// Load current config to detect already-registered markets (by URL)
			cfg, err := svc.Config.GetConfig()
			if err != nil {
				return err
			}
			existingURLs := make(map[string]string) // normalizedURL -> name
			for _, mc := range cfg.Markets {
				existingURLs[domain.NormalizeURL(mc.URL)] = mc.Name
			}

			type importResult struct {
				Action string `json:"action"`
				Type   string `json:"type"`
				Ref    string `json:"ref"`
				Detail string `json:"detail,omitempty"`
			}
			var results []importResult

			// Import markets
			scanner := bufio.NewScanner(cmd.InOrStdin())
			for _, m := range imp.Markets {
				norm := domain.NormalizeURL(m.URL)
				if existing, ok := existingURLs[norm]; ok {
					results = append(results, importResult{
						Action: "skip",
						Type:   "market",
						Ref:    m.Name,
						Detail: fmt.Sprintf("URL already registered as %q", existing),
					})
					continue
				}

				if dryRun {
					results = append(results, importResult{Action: "add", Type: "market", Ref: m.Name, Detail: "(dry-run)"})
					continue
				}

				if !yes {
					if jsonOut {
						results = append(results, importResult{Action: "skip", Type: "market", Ref: m.Name, Detail: "not registered locally (use --yes to add)"})
						continue
					}
					cmd.Printf("  ?   market %q (%s) is not registered locally. Add it? [y/N] ", m.Name, m.URL)
					scanner.Scan()
					answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
					if answer != "y" && answer != "yes" {
						results = append(results, importResult{Action: "skip", Type: "market", Ref: m.Name, Detail: "declined by user"})
						continue
					}
				}

				_, err := svc.Markets.AddMarket(m.URL, service.AddMarketOpts{
					Branch:   m.Branch,
					Trusted:  m.Trusted,
					ReadOnly: m.ReadOnly,
				})
				if err != nil {
					results = append(results, importResult{Action: "error", Type: "market", Ref: m.Name, Detail: err.Error()})
				} else {
					results = append(results, importResult{Action: "add", Type: "market", Ref: m.Name})
					existingURLs[norm] = m.Name
				}
			}

			// Import entries: each entry is a profile, list all entries in it and install
			for _, e := range imp.Entries {
				profile := e.Profile
				// profile is "market@seg1/seg2", extract market name
				market, relProfile, _ := domain.MctRef(profile).Parse()
				if market == "" {
					results = append(results, importResult{Action: "error", Type: "profile", Ref: profile, Detail: "invalid profile ref"})
					continue
				}

				// Re-index on each profile to reflect installs done in previous iterations
				allIndexed, indexErr := svc.Search.DumpIndex()
				if indexErr != nil {
					results = append(results, importResult{Action: "error", Type: "profile", Ref: profile, Detail: indexErr.Error()})
					continue
				}

				installed := 0
				for _, entry := range allIndexed {
					if entry.Market != market || !strings.HasPrefix(entry.RelPath, relProfile+"/") {
						continue
					}
					if entry.Installed {
						continue
					}

					if dryRun {
						results = append(results, importResult{Action: "add", Type: "entry", Ref: string(entry.Ref), Detail: "(dry-run)"})
						installed++
						continue
					}

					err := svc.Entries.Add(entry.Ref, service.AddOpts{})
					if errors.Is(err, domain.ErrEntryAlreadyInstalled) {
						// already on disk (e.g. auto-installed as a dep by a previous entry)
					} else if err != nil {
						results = append(results, importResult{Action: "error", Type: "entry", Ref: string(entry.Ref), Detail: err.Error()})
					} else {
						results = append(results, importResult{Action: "add", Type: "entry", Ref: string(entry.Ref)})
						installed++
						for _, dep := range entry.RequiresSkills {
							depRef := domain.MctRef(entry.Market + "@" + dep.File)
							results = append(results, importResult{Action: "add", Type: "entry", Ref: string(depRef), Detail: "(dep)"})
							installed++
						}
					}
				}

				if installed == 0 && !dryRun {
					results = append(results, importResult{Action: "skip", Type: "profile", Ref: profile, Detail: "all entries already installed"})
				}
			}

			if jsonOut {
				return printJSON(cmd.OutOrStdout(), results)
			}

			for _, r := range results {
				switch r.Action {
				case "skip":
					cmd.Printf("  --  %s %s: %s\n", r.Type, r.Ref, r.Detail)
				case "add":
					detail := ""
					if r.Detail != "" {
						detail = " " + r.Detail
					}
					cmd.Printf("  ok  %s %s%s\n", r.Type, r.Ref, detail)
				case "error":
					cmd.PrintErrf("  x   %s %s: %s\n", r.Type, r.Ref, r.Detail)
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("dry-run", false, "preview import without changes")
	cmd.Flags().Bool("json", false, "JSON output")
	cmd.Flags().Bool("yes", false, "automatically confirm adding markets not registered locally")
	return cmd
}
