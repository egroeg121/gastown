package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/workspace"
)

// accountantCmd is the root command for managing accountant agents.
// An accountant tracks costs, quotas, and resource usage across the town.
var accountantCmd = &cobra.Command{
	Use:     "accountant",
	GroupID: GroupAgents,
	Short:   "Manage the Accountant (cost and quota tracker)",
	RunE:    requireSubcommand,
	Long: `Manage the Accountant — the town-level cost and quota monitor.

The Accountant tracks token usage, API costs, and quota consumption
across all agents in the town. It can halt agents when budgets are
exceeded and resume them when headroom is restored.

Subcommands:
  status   Show current cost and quota summary
  halt     Pause agents to prevent runaway spend
  resume   Resume halted agents`,
}

var accountantStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cost and quota status",
	Long: `Show current cost and quota consumption across all agents.

Displays per-agent token usage, cumulative cost estimates, and
remaining quota headroom for the current billing period.`,
	RunE: runAccountantStatus,
}

var accountantHaltCmd = &cobra.Command{
	Use:   "halt [--rig <rig>]",
	Short: "Halt agents to stop runaway spend",
	Long: `Halt one or all rigs to prevent runaway token spend.

When a rig is halted, new polecats are not spawned and existing
polecats receive a DND signal. Use 'gt accountant resume' to restore.

Examples:
  gt accountant halt              # Halt all rigs (town-wide)
  gt accountant halt --rig myrig  # Halt a specific rig`,
	RunE: runAccountantHalt,
}

var accountantResumeCmd = &cobra.Command{
	Use:   "resume [--rig <rig>]",
	Short: "Resume halted agents",
	Long: `Resume agents previously halted by 'gt accountant halt'.

Examples:
  gt accountant resume              # Resume all rigs
  gt accountant resume --rig myrig  # Resume a specific rig`,
	RunE: runAccountantResume,
}

var (
	accountantRig string
)

func init() {
	accountantHaltCmd.Flags().StringVar(&accountantRig, "rig", "", "Specific rig to halt (default: all)")
	accountantResumeCmd.Flags().StringVar(&accountantRig, "rig", "", "Specific rig to resume (default: all)")

	accountantCmd.AddCommand(accountantStatusCmd)
	accountantCmd.AddCommand(accountantHaltCmd)
	accountantCmd.AddCommand(accountantResumeCmd)
	rootCmd.AddCommand(accountantCmd)
}

func runAccountantStatus(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwd()
	if err != nil || townRoot == "" {
		return fmt.Errorf("not in a Gas Town workspace")
	}

	fmt.Printf("%s\n", style.Bold.Render("Accountant Status"))
	fmt.Printf("%s %s\n\n", style.Dim.Render("Town:"), townRoot)

	// Show per-rig cost state files if they exist.
	costGlob := filepath.Join(townRoot, "gt", "rigs", "*", "costs.jsonl")
	matches, _ := filepath.Glob(costGlob)
	if len(matches) == 0 {
		fmt.Printf("\n%s No cost logs found. Cost tracking requires agents to report usage.\n",
			style.Dim.Render("ℹ"))
	} else {
		fmt.Printf("\n%s\n", style.Bold.Render("Cost Logs:"))
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil {
				continue
			}
			rig := filepath.Base(filepath.Dir(m))
			age := time.Since(info.ModTime()).Round(time.Second)
			fmt.Printf("  %-20s  %s  (updated %s ago)\n", rig, style.Dim.Render(m), age)
		}
	}

	// Halt flag sentinel.
	haltSentinel := filepath.Join(townRoot, "gt", "accountant-halt")
	if _, err := os.Stat(haltSentinel); err == nil {
		fmt.Printf("\n%s Town is HALTED — run 'gt accountant resume' to restore.\n",
			style.Bold.Render("⛔"))
	}

	return nil
}

func runAccountantHalt(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwd()
	if err != nil || townRoot == "" {
		return fmt.Errorf("not in a Gas Town workspace")
	}

	gtDir := filepath.Join(townRoot, "gt")
	if err := os.MkdirAll(gtDir, 0o755); err != nil {
		return fmt.Errorf("create gt dir: %w", err)
	}

	if accountantRig != "" {
		// Rig-scoped halt: write sentinel into the rig directory.
		sentinel := filepath.Join(townRoot, "gt", "rigs", accountantRig, "accountant-halt")
		if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
			return fmt.Errorf("create rig dir: %w", err)
		}
		if err := os.WriteFile(sentinel, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644); err != nil {
			return fmt.Errorf("write halt sentinel: %w", err)
		}
		fmt.Printf("%s Rig %s halted. Run 'gt accountant resume --rig %s' to restore.\n",
			style.Bold.Render("⛔"), accountantRig, accountantRig)
		return nil
	}

	// Town-wide halt.
	sentinel := filepath.Join(gtDir, "accountant-halt")
	if err := os.WriteFile(sentinel, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write halt sentinel: %w", err)
	}
	fmt.Printf("%s Town halted. New polecats will not be spawned.\n", style.Bold.Render("⛔"))
	fmt.Printf("  Run 'gt accountant resume' to restore.\n")
	return nil
}

func runAccountantResume(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwd()
	if err != nil || townRoot == "" {
		return fmt.Errorf("not in a Gas Town workspace")
	}

	if accountantRig != "" {
		sentinel := filepath.Join(townRoot, "gt", "rigs", accountantRig, "accountant-halt")
		if err := os.Remove(sentinel); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove halt sentinel: %w", err)
		}
		fmt.Printf("%s Rig %s resumed.\n", style.Bold.Render("✓"), accountantRig)
		return nil
	}

	sentinel := filepath.Join(townRoot, "gt", "accountant-halt")
	if err := os.Remove(sentinel); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove halt sentinel: %w", err)
	}
	fmt.Printf("%s Town resumed. Polecats may be spawned again.\n", style.Bold.Render("✓"))
	return nil
}
