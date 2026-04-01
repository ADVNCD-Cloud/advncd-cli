package cmd

import "github.com/spf13/cobra"

var (
	serviceDisableYes  bool
	serviceDisableHard bool
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Service emergency actions",
}

var serviceDisableCmd = &cobra.Command{
	Use:   "disable <service>",
	Short: "Disable a service quickly (kill switch)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServiceDisable(args[0], serviceDisableYes, serviceDisableHard)
	},
}

func init() {
	serviceDisableCmd.Flags().BoolVar(&serviceDisableYes, "yes", false, "Skip confirmation prompt")
	serviceDisableCmd.Flags().BoolVar(&serviceDisableHard, "hard", false, "Hard kill switch: delete service instead of removing public access")
	serviceCmd.AddCommand(serviceDisableCmd)
}
