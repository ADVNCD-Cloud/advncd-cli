package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/dashboard"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start local Advncd dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		srv := dashboard.NewServer()

		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = openBrowser("http://" + srv.Addr)
		}()

		fmt.Printf("Dashboard running at http://%s\n", srv.Addr)
		return http.ListenAndServe(srv.Addr, srv.Handler)
	},
}