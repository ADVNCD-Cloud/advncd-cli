package cmd

import (
	"testing"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
)

func TestRootIncludesStableSprintOneCommands(t *testing.T) {
	t.Parallel()

	for _, name := range contracts.StableCLICommands() {
		found, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatalf("command %q not found: %v", name, err)
		}
		if found == nil || found.Name() != name {
			t.Fatalf("command %q resolved incorrectly", name)
		}
	}
}
