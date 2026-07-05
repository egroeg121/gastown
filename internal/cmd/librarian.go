package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/style"
)

// librarianCmd is the root command for inspecting the Librarian's knowledge base.
// The Librarian is a town-scoped role that curates ~/knowledge/ (a separate
// git repo from the town workspace): promoting raw/ notes into wiki/ pages
// and fact-checking existing pages on an hourly cadence.
var librarianCmd = &cobra.Command{
	Use:     "librarian",
	GroupID: GroupAgents,
	Short:   "Manage the Librarian (knowledge base curator)",
	RunE:    requireSubcommand,
	Long: `Manage the Librarian — the town-level knowledge base curator.

The Librarian owns ~/knowledge/, a git repo split into raw/ (freely captured
notes from polecats at 'gt done') and wiki/ (curated, durable pages). It
promotes durable findings into the wiki and periodically fact-checks existing
pages against ground truth.

Subcommands:
  status   Show current knowledge base state (raw backlog, wiki size, last sync)`,
}

var librarianStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show knowledge base status",
	Long: `Show the current state of the ~/knowledge/ repo: how many raw notes are
awaiting curation, how many wiki pages exist, and when the repo was last
committed and pushed.`,
	RunE: runLibrarianStatus,
}

func init() {
	librarianCmd.AddCommand(librarianStatusCmd)
	rootCmd.AddCommand(librarianCmd)
}

// knowledgeRepoDir returns the path to the Librarian's knowledge base repo.
func knowledgeRepoDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "knowledge"
	}
	return filepath.Join(home, "knowledge")
}

func runLibrarianStatus(cmd *cobra.Command, args []string) error {
	knowledgeDir := knowledgeRepoDir()

	fmt.Printf("%s\n", style.Bold.Render("Librarian Status"))
	fmt.Printf("%s %s\n\n", style.Dim.Render("Knowledge repo:"), knowledgeDir)

	if _, err := os.Stat(knowledgeDir); err != nil {
		fmt.Printf("%s Knowledge repo not found at %s.\n", style.Dim.Render("ℹ"), knowledgeDir)
		return nil
	}

	rawDir := filepath.Join(knowledgeDir, "raw")
	wikiDir := filepath.Join(knowledgeDir, "wiki")

	rawCount := countFiles(rawDir)
	wikiCount := countFiles(wikiDir)

	fmt.Printf("%s %d note(s) awaiting curation\n", style.Dim.Render("raw/:"), rawCount)
	fmt.Printf("%s %d page(s)\n", style.Dim.Render("wiki/:"), wikiCount)

	if lastCommit, err := lastGitCommitSummary(knowledgeDir); err == nil && lastCommit != "" {
		fmt.Printf("%s %s\n", style.Dim.Render("last commit:"), lastCommit)
	}

	if rawCount > 0 {
		fmt.Printf("\n%s %d raw note(s) waiting — run a curation pass.\n",
			style.Bold.Render("ℹ"), rawCount)
	}

	return nil
}

func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

func lastGitCommitSummary(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%h %cr: %s").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
