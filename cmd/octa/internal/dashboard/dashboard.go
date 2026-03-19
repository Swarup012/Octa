package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Swarup012/solo/cmd/octa/internal/dashboard/internal/server"
	"github.com/spf13/cobra"
)

//go:embed internal/ui/index.html
var staticFiles embed.FS

func NewDashboardCommand() *cobra.Command {
	var public bool
	var configPath string

	cmd := &cobra.Command{
		Use:   "dashboard [config.json]",
		Short: "Start a web-based configuration editor and dashboard",
		Long:  "Octa Dashboard - A web-based configuration editor that runs locally in your browser.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				configPath = args[0]
			} else {
				configPath = server.DefaultConfigPath()
			}

			absPath, err := filepath.Abs(configPath)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			var addr string
			if public {
				addr = "0.0.0.0:" + server.DefaultPort
			} else {
				addr = "127.0.0.1:" + server.DefaultPort
			}

			mux := http.NewServeMux()
			server.RegisterConfigAPI(mux, absPath)
			server.RegisterAuthAPI(mux, absPath)
			server.RegisterProcessAPI(mux, absPath)

			staticFS, err := fs.Sub(staticFiles, "internal/ui")
			if err != nil {
				return fmt.Errorf("failed to create sub filesystem: %w", err)
			}
			mux.Handle("/", http.FileServer(http.FS(staticFS)))

			// Print startup banner
			fmt.Println("=============================================")
			fmt.Println("  Octa Dashboard")
			fmt.Println("=============================================")
			fmt.Printf("  Config file : %s\n", absPath)
			fmt.Printf("  Listen addr : %s\n\n", addr)
			fmt.Println("  Open the following URL in your browser")
			fmt.Println("  to view and edit the configuration:")
			fmt.Println()
			fmt.Printf("    >> http://localhost:%s <<\n", server.DefaultPort)
			if public {
				if ip := server.GetLocalIP(); ip != "" {
					fmt.Printf("    >> http://%s:%s <<\n", ip, server.DefaultPort)
				}
			}
			fmt.Println()

			go func() {
				// Wait briefly to ensure the server is ready before opening the browser
				time.Sleep(500 * time.Millisecond)
				url := "http://localhost:" + server.DefaultPort
				if err := openBrowser(url); err != nil {
					log.Printf("Warning: Failed to auto-open browser: %v\n", err)
				}
			}()

			return http.ListenAndServe(addr, mux)
		},
	}

	cmd.Flags().BoolVar(&public, "public", false, "Listen on all interfaces (0.0.0.0) instead of localhost only")

	return cmd
}

// openBrowser automatically opens the given URL in the default browser.
func openBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}
