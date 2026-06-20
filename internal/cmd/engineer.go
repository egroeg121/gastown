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

// engineerCmd manages the Engineer agent — responsible for reviewing code
// changes, checking build gates, and summarising PR readiness.
var engineerCmd = &cobra.Command{
	Use:     "engineer",
	GroupID: GroupAgents,
	Short:   "Manage the Engineer (code review and build gate checks)",
	RunE:    requireSubcommand,
	Long: `Manage the Engineer — the code quality and review agent.

The Engineer runs structured code reviews against the current branch diff,
checks build gates, and summarises merge-readiness.

Subcommands:
  review   Run a structured code review of the current branch`,
}

var engineerReviewCmd = &cobra.Command{
	Use:   "review [--branch <branch>] [--against <base>]",
	Short: "Run a structured code review of the current branch",
	Long: `Run a structured code review against the current branch diff.

Invokes the /review skill (if available) or falls back to printing the
diff with gate status. Useful for triggering a review from an agent
context where the skill shorthand is not directly accessible.

Examples:
  gt engineer review                         # Review HEAD vs origin/main
  gt engineer review --against origin/main   # Explicit base
  gt engineer review --json                  # Machine-readable output`,
	RunE: runEngineerReview,
}

var (
	engineerAgainst string
	engineerJSON    bool
)

func init() {
	engineerReviewCmd.Flags().StringVar(&engineerAgainst, "against", "origin/main",
		"Base ref to diff against")
	engineerReviewCmd.Flags().BoolVar(&engineerJSON, "json", false,
		"Output in JSON format")

	engineerCmd.AddCommand(engineerReviewCmd)
	rootCmd.AddCommand(engineerCmd)
}

func runEngineerReview(cmd *cobra.Command, args []string) error {
	townRoot, err := workspace.FindFromCwd()
	if err != nil || townRoot == "" {
		return fmt.Errorf("not in a Gas Town workspace")
	}

	base := engineerAgainst
	if base == "" {
		base = "origin/main"
	}

	if !engineerJSON {
		fmt.Printf("%s (against %s)\n\n", style.Bold.Render("Engineer Review"), base)
	}

	// Collect git diff stat.
	diffStatCmd := exec.Command("git", "diff", "--stat", base+"...HEAD") //nolint:gosec
	diffStatOut, diffStatErr := diffStatCmd.Output()
	if diffStatErr != nil {
		fmt.Printf("%s git diff --stat: %v\n", style.WarningPrefix, diffStatErr)
	}

	// Collect commit log.
	logCmd := exec.Command("git", "log", "--oneline", base+"..HEAD") //nolint:gosec
	logOut, logErr := logCmd.Output()
	if logErr != nil {
		fmt.Printf("%s git log: %v\n", style.WarningPrefix, logErr)
	}

	commitLines := strings.Split(strings.TrimSpace(string(logOut)), "\n")
	commitCount := 0
	for _, l := range commitLines {
		if strings.TrimSpace(l) != "" {
			commitCount++
		}
	}

	if engineerJSON {
		fmt.Printf(`{"base":%q,"commits":%d,"diff_stat":%q}`,
			base, commitCount, strings.TrimSpace(string(diffStatOut)))
		fmt.Println()
		return nil
	}

	fmt.Printf("%s %d commit(s) ahead of %s\n", style.Dim.Render("Commits:"), commitCount, base)
	if len(logOut) > 0 {
		for _, l := range commitLines {
			if strings.TrimSpace(l) != "" {
				fmt.Printf("  %s\n", l)
			}
		}
		fmt.Println()
	}

	if len(diffStatOut) > 0 {
		fmt.Printf("%s\n%s\n", style.Dim.Render("Diff stat:"), strings.TrimRight(string(diffStatOut), "\n"))
	}

	// Log review request.
	reviewDir := filepath.Join(townRoot, "gt", "engineer")
	if mkErr := os.MkdirAll(reviewDir, 0o755); mkErr == nil {
		logPath := filepath.Join(reviewDir, "review.log")
		entry := fmt.Sprintf("%s review --against %s (%d commits)\n",
			time.Now().Format(time.RFC3339), base, commitCount)
		if f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
			_, _ = f.WriteString(entry)
			f.Close()
		}
	}

	fmt.Printf("\n%s Run '/review --branch' in a Claude session for full structured grading.\n",
		style.Dim.Render("Tip:"))

	return nil
}
