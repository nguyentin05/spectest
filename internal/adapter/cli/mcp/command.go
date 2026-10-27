package mcp

import (
	"fmt"

	adaptermcp "github.com/nguyentin05/spectest/internal/adapter/mcp"
	"github.com/spf13/cobra"
)

// Options holds CLI flags for the MCP server.
type Options struct {
	Root string
}

// NewCommand creates the `spectest mcp` command tree.
func NewCommand() *cobra.Command {
	var opts Options

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol (MCP) server for AI coding assistants",
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local stdio MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := adaptermcp.NewServer(opts.Root)
			if err != nil {
				return fmt.Errorf("initialize mcp server: %w", err)
			}
			return srv.ServeStdio()
		},
	}

	serveCmd.Flags().StringVar(&opts.Root, "root", ".", "Root workspace directory to isolate the MCP server")

	cmd.AddCommand(serveCmd)
	return cmd
}
