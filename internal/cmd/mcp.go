package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/basecamp/hey-cli/internal/mcpserver"
)

// mcpTransport is a seam so tests can drive the server over in-memory
// transports instead of the process's stdin/stdout.
var mcpTransport = func() mcp.Transport { return &mcp.StdioTransport{} }

type mcpCommand struct {
	cmd      *cobra.Command
	readOnly bool
	domains  []string
}

func newMCPCommand() *mcpCommand {
	mcpCommand := &mcpCommand{}
	mcpCommand.cmd = &cobra.Command{
		Use:   "mcp",
		Short: "Serve HEY to MCP clients over stdio",
		Long: "Run an MCP (Model Context Protocol) server on stdin/stdout, serving HEY mail,\n" +
			"contacts, and todos as tools backed by your signed-in account.\n\n" +
			"Register it with an MCP client as a stdio server, e.g.:\n\n" +
			"  claude mcp add hey -- hey mcp",
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"agent_notes": "Long-running server; stdout speaks the MCP wire protocol. Not for interactive use.",
		},
		RunE: mcpCommand.run,
	}

	mcpCommand.cmd.Flags().BoolVar(&mcpCommand.readOnly, "read-only", false, "Serve only read-only actions")
	mcpCommand.cmd.Flags().StringSliceVar(&mcpCommand.domains, "domains", nil, "Narrow to specific domains (comma-separated; default all)")

	return mcpCommand
}

func (c *mcpCommand) run(cmd *cobra.Command, args []string) error {
	if err := requireAuth(); err != nil {
		return err
	}

	srv, err := mcpserver.New(sdk, mcpserver.Config{ReadOnly: c.readOnly, Domains: c.domains})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Log to stderr: stdout belongs to the MCP wire.
	logger := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), nil))
	session, err := srv.BuildMCPServer(logger).Connect(ctx, mcpTransport(), nil)
	if err != nil {
		return err
	}
	logger.Info("MCP server running on stdio", "tools", len(srv.Domains()), "read_only", c.readOnly)

	return session.Wait()
}
