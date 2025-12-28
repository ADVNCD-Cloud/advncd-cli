package ui

import (
	"fmt"
	"sort"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

type LoginInstructions struct {
	VerificationURL string
	UserCode        string
	ExpiresIn       int
	Interval        int
}

func PrintLoginInstructions(in LoginInstructions) {
	fmt.Println("To authenticate, visit this URL in your browser:")
	fmt.Printf("  %s\n", in.VerificationURL)
	fmt.Println()
	fmt.Println("Then enter this code:")
	fmt.Printf("  %s\n", in.UserCode)
	fmt.Println()
	fmt.Printf("Code expires in %d seconds. Poll interval: %d seconds.\n", in.ExpiresIn, in.Interval)
}

func PrintError(e *apperr.Error) {
	fmt.Printf("Error %s: %s\n", e.Code, e.Message)

	if len(e.Meta) > 0 {
		fmt.Println()
		fmt.Println("Details:")
		keys := make([]string, 0, len(e.Meta))
		for k := range e.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  - %s: %s\n", k, e.Meta[k])
		}
	}

	if len(e.FixWith) > 0 {
		fmt.Println()
		fmt.Println("Fix:")
		for _, f := range e.FixWith {
			fmt.Printf("  - %s\n", f)
		}
	}
}

func PrintPlainError(err error) {
	fmt.Printf("Error: %v\n", err)
}