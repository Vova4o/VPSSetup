package main

import (
	"fmt"

	"github.com/Vova4o/VPSSetup/api"
	"github.com/spf13/cobra"
)

// runWeb starts the web API server
func runWeb(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	if port == 0 {
		port = 8080
	}

	fmt.Printf("🌐 Starting VPSSetup Web Server\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	server := api.NewServer(cfg, port)
	return server.Start()
}

func init() {
	webCmd.Flags().Int("port", 8080, "Port to run the web server on")
}
