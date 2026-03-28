package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
)

// ExportData is the portable format for sharing mct configurations.
type ExportData struct {
	Version int                      `json:"version"`
	Markets []ExportMarket           `json:"markets"`
	Entries []ExportEntry            `json:"entries"`
	Config  ExportConfig             `json:"config"`
}

type ExportMarket struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Branch   string `json:"branch"`
	Trusted  bool   `json:"trusted,omitempty"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type ExportEntry struct {
	Ref          string `json:"ref"`
	Pin          string `json:"pin,omitempty"`
	DriftAllowed bool   `json:"drift_allowed,omitempty"`
	Managed      bool   `json:"managed,omitempty"`
	ManagedBy    string `json:"managed_by,omitempty"`
}

type ExportConfig struct {
	LocalPath      string `json:"local_path,omitempty"`
	ConflictPolicy string `json:"conflict_policy,omitempty"`
	DriftPolicy    string `json:"drift_policy,omitempty"`
	SSHEnabled     *bool  `json:"ssh_enabled,omitempty"`
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

			export := ExportData{Version: 1}

			// Markets
			for _, mc := range cfg.Markets {
				export.Markets = append(export.Markets, ExportMarket{
					Name:     mc.Name,
					URL:      mc.URL,
					Branch:   mc.Branch,
					Trusted:  mc.Trusted,
					ReadOnly: mc.ReadOnly,
				})
			}

			// Entries
			managed := make(map[string]domain.ManagedSkillConfig)
			for _, ms := range cfg.ManagedSkills {
				managed[string(ms.Ref)] = ms
			}
			for _, ec := range cfg.Entries {
				export.Entries = append(export.Entries, ExportEntry{
					Ref:          string(ec.Ref),
					Pin:          ec.Pin,
					DriftAllowed: ec.DriftAllowed,
				})
			}
			for _, ms := range cfg.ManagedSkills {
				export.Entries = append(export.Entries, ExportEntry{
					Ref:       string(ms.Ref),
					Managed:   true,
					ManagedBy: string(ms.ManagedBy),
				})
			}

			// Config
			export.Config = ExportConfig{
				LocalPath:      cfg.LocalPath,
				ConflictPolicy: cfg.ConflictPolicy,
				DriftPolicy:    cfg.DriftPolicy,
				SSHEnabled:     cfg.SSHEnabled,
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
				existingURLs[normalizeMarketURL(mc.URL)] = mc.Name
			}

			type importResult struct {
				Action string `json:"action"`
				Type   string `json:"type"`
				Ref    string `json:"ref"`
				Detail string `json:"detail,omitempty"`
			}
			var results []importResult

			// Import markets
			for _, m := range imp.Markets {
				norm := normalizeMarketURL(m.URL)
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

				_, err := svc.Markets.AddMarket(m.Name, m.URL, service.AddMarketOpts{
					Branch:   m.Branch,
					Trusted:  m.Trusted,
					ReadOnly: m.ReadOnly,
				})
				if err != nil {
					results = append(results, importResult{Action: "error", Type: "market", Ref: m.Name, Detail: err.Error()})
				} else {
					results = append(results, importResult{Action: "add", Type: "market", Ref: m.Name})
				}
			}

			// Import entries (skip managed, they'll be auto-installed via deps)
			for _, e := range imp.Entries {
				if e.Managed {
					continue
				}

				if dryRun {
					results = append(results, importResult{Action: "add", Type: "entry", Ref: e.Ref, Detail: "(dry-run)"})
					continue
				}

				err := svc.Entries.Add(domain.MctRef(e.Ref), service.AddOpts{
					Pin: e.Pin,
				})
				if err != nil {
					results = append(results, importResult{Action: "error", Type: "entry", Ref: e.Ref, Detail: err.Error()})
				} else {
					results = append(results, importResult{Action: "add", Type: "entry", Ref: e.Ref})
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
	return cmd
}

// normalizeMarketURL normalizes a git URL for comparison.
func normalizeMarketURL(u string) string {
	u = strings.TrimSpace(u)
	if idx := strings.Index(u, "://"); idx >= 0 {
		u = u[idx+3:]
	} else if at := strings.Index(u, "@"); at >= 0 {
		u = u[at+1:]
		u = strings.Replace(u, ":", "/", 1)
	}
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimSuffix(u, "/")
	return strings.ToLower(u)
}
