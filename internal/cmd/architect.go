package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// architectCmd manages the Architect agent — responsible for scanning rigs
// and decomposing large tasks into workable beads.
var architectCmd = &cobra.Command{
	Use:     "architect",
	GroupID: GroupAgents,
	Short:   "Manage the Architect (task decomposition and rig scanning)",
	RunE:    requireSubcommand,
	Long: `Manage the Architect — the strategic task decomposition agent.

The Architect scans rigs for open work and decomposes large tasks into
smaller, parallelisable beads that polecats can pick up independently.

Subcommands:
  scan        Scan a rig for open, blocked, or oversized work
  decompose   Break a large bead into parallelisable sub-beads`,
}

var architectScanCmd = &cobra.Command{
	Use:   "scan [<rig>]",
	Short: "Scan a rig for open and oversized work",
	Long: `Scan a rig's beads for open, blocked, or oversized work items.

When no rig is specified, scans the current rig detected from the
working directory.

Outputs a summary of:
  - Open beads without an assignee
  - Beads marked blocked and their blockers
  - Oversized beads that may need decomposition

Examples:
  gt architect scan              # Scan current rig
  gt architect scan greenplace   # Scan a named rig`,
	Args: cobra.MaximumNArgs(1),
	RunE: runArchitectScan,
}

var architectDecomposeCmd = &cobra.Command{
	Use:   "decompose <bead-id>",
	Short: "Decompose a large bead into parallelisable sub-beads",
	Long: `Decompose a large or complex bead into smaller, independent sub-beads.

The Architect analyses the bead description and existing sub-beads,
then produces a decomposition plan. In dry-run mode (default) the
plan is printed for review; with --apply the sub-beads are created
via 'bd create --parent'.

Examples:
  gt architect decompose hq-abc           # Show decomposition plan
  gt architect decompose hq-abc --apply   # Print sub-bead creation commands`,
	Args: cobra.ExactArgs(1),
	RunE: runArchitectDecompose,
}

var architectApply bool

func init() {
	architectDecomposeCmd.Flags().BoolVar(&architectApply, "apply", false,
		"Print bd create commands to create sub-beads (does not execute them)")

	architectCmd.AddCommand(architectScanCmd)
	architectCmd.AddCommand(architectDecomposeCmd)
	rootCmd.AddCommand(architectCmd)
}

func runArchitectScan(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwd()
	if err != nil || townRoot == "" {
		return fmt.Errorf("not in a Gas Town workspace")
	}

	rigName := ""
	if len(args) > 0 {
		rigName = args[0]
	}

	fmt.Printf("%s\n", style.Bold.Render("Architect Scan"))
	if rigName != "" {
		fmt.Printf("%s %s\n\n", style.Dim.Render("Rig:"), rigName)
	} else {
		fmt.Printf("%s (current)\n\n", style.Dim.Render("Rig:"))
	}

	// Run `bd ready` to surface available work.
	bdArgs := []string{"ready"}
	if rigName != "" {
		bdArgs = append(bdArgs, "--rig", rigName)
	}
	out, bdErr := bdOutput(bdArgs...)
	if bdErr != nil {
		fmt.Printf("%s bd ready: %v\n", style.WarningPrefix, bdErr)
	} else {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		openCount := 0
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				openCount++
			}
		}
		fmt.Printf("%s %d open bead(s) ready for work\n", style.Dim.Render("Open:"), openCount)
		if openCount > 0 {
			fmt.Println()
			fmt.Println(out)
		}
	}

	// Record last-scan timestamp.
	scanDir := filepath.Join(townRoot, "gt", "architect")
	if mkErr := os.MkdirAll(scanDir, 0o755); mkErr == nil {
		_ = os.WriteFile(filepath.Join(scanDir, "last-scan"),
			[]byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
	}

	return nil
}

func runArchitectDecompose(cmd *cobra.Command, args []string) error {
	beadID := args[0]

	townRoot, err := workspace.FindFromCwd()
	if err != nil || townRoot == "" {
		return fmt.Errorf("not in a Gas Town workspace")
	}

	fmt.Printf("%s %s\n\n", style.Bold.Render("Architect Decompose:"), beadID)

	// Fetch bead details.
	out, bdErr := bdOutput("show", beadID)
	if bdErr != nil {
		return fmt.Errorf("bd show %s: %w", beadID, bdErr)
	}
	fmt.Println(out)

	if !architectApply {
		fmt.Printf("%s re-run with --apply to see sub-bead creation commands.\n", style.Dim.Render("Dry-run:"))
		fmt.Printf("\n%s for judgment-heavy decomposition, sling a jasper polecat:\n", style.Dim.Render("Tip:"))
		fmt.Printf("  gt sling %s <rig> --agent jasper\n", beadID)
		return nil
	}

	// --apply: print the commands to create sub-beads.
	fmt.Printf("%s\n", style.Bold.Render("Sub-bead creation template (edit titles as needed):"))
	fmt.Printf("  bd create --title 'Sub-task 1: <description>' --parent %s\n", beadID)
	fmt.Printf("  bd create --title 'Sub-task 2: <description>' --parent %s\n", beadID)
	fmt.Printf("\nFor automated decomposition, sling a jasper (Opus) polecat:\n")
	fmt.Printf("  gt sling %s <rig> --agent jasper\n", beadID)

	// Record decompose attempt.
	scanDir := filepath.Join(townRoot, "gt", "architect")
	if mkErr := os.MkdirAll(scanDir, 0o755); mkErr == nil {
		logPath := filepath.Join(scanDir, "decompose.log")
		entry := fmt.Sprintf("%s decompose %s\n", time.Now().Format(time.RFC3339), beadID)
		if f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
			_, _ = f.WriteString(entry)
			f.Close()
		}
	}

	return nil
}

// bdOutput runs a bd subcommand and returns combined stdout as a string.
func bdOutput(args ...string) (string, error) {
	c := exec.Command("bd", args...) //nolint:gosec // G204: bd is a trusted internal tool
	out, err := c.Output()
	return strings.TrimRight(string(out), "\n"), err
}
