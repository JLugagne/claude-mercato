package commands

import (
	"bufio"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JLugagne/agents-mercato/internal/mercato/domain/service"
)

// restoreFlags carries the precomputed CLI/CI flags that drive the prompt.
type restoreFlags struct {
	all  bool
	none bool
	ci   bool
}

// chooseFilesToRestore returns the subset of `deleted` that the user wants
// reinstalled. Resolution order:
//   - flag --restore-none, or CI mode without --restore-all → return nil
//   - flag --restore-all → return everything
//   - otherwise → prompt per file with [r]estore / [k]eep-deleted / [a]ll / [n]one
//
// The prompt reads from cmd.InOrStdin so tests can drive it deterministically.
func chooseFilesToRestore(cmd *cobra.Command, deleted []service.DeletedFile, flags restoreFlags) []service.DeletedFile {
	if flags.none {
		return nil
	}
	if flags.all {
		return deleted
	}
	if flags.ci {
		// Silent default in CI: do not restore. Operator must opt in.
		return nil
	}

	cmd.Printf("\nDetected %d locally-deleted file(s) that mct previously installed:\n", len(deleted))
	reader := bufio.NewReader(cmd.InOrStdin())

	var picked []service.DeletedFile
	keepAll := false
	for i, f := range deleted {
		if keepAll {
			break
		}
		cmd.Printf("  [%d/%d] %s in %s\n", i+1, len(deleted), f.RelPath, f.Location)
		cmd.Print("    [r]estore / [k]eep-deleted / [a]ll-restore / [n]one: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			// Treat EOF / read error as "keep deleted, stop prompting".
			break
		}
		switch strings.TrimSpace(strings.ToLower(line)) {
		case "r", "restore":
			picked = append(picked, f)
		case "k", "keep", "":
			// skip
		case "a", "all":
			picked = append(picked, deleted[i:]...)
			return picked
		case "n", "none":
			return picked
		default:
			cmd.Printf("    (unrecognized %q — treating as keep)\n", strings.TrimSpace(line))
		}
	}
	return picked
}
