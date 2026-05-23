package cmd

import (
	"testing"

	"github.com/steveyegge/gastown/internal/polecat"
)

func TestPolecatNukeTargetChanged(t *testing.T) {
	base := &polecat.Polecat{Branch: "polecat/vault/gt-abc", Issue: "gt-abc", ClonePath: "/tmp/polecat"}

	if polecatNukeTargetChanged(base, &polecat.Polecat{Branch: base.Branch, Issue: base.Issue, ClonePath: base.ClonePath}) {
		t.Fatal("identical nuke target should not be considered changed")
	}
	if !polecatNukeTargetChanged(base, &polecat.Polecat{Branch: "polecat/other", Issue: base.Issue, ClonePath: base.ClonePath}) {
		t.Fatal("branch change must abort destructive nuke sequence")
	}
	if !polecatNukeTargetChanged(base, &polecat.Polecat{Branch: base.Branch, Issue: "gt-other", ClonePath: base.ClonePath}) {
		t.Fatal("issue change must abort destructive nuke sequence")
	}
	if !polecatNukeTargetChanged(base, &polecat.Polecat{Branch: base.Branch, Issue: base.Issue, ClonePath: "/tmp/other"}) {
		t.Fatal("clone path change must abort destructive nuke sequence")
	}
}
