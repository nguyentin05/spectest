package mcp

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"github.com/nguyentin05/spectest/internal/platform/buildinfo"
)

// Server implements the local stdio MCP adapter for SpecTest-AI.
type Server struct {
	resolver  *PathResolver
	jobs      *JobManager
	mcpServer *server.MCPServer
}

// NewServer initializes a new SpecTest MCP server constrained within root.
func NewServer(root string) (*Server, error) {
	resolver, err := NewPathResolver(root)
	if err != nil {
		return nil, fmt.Errorf("init path resolver: %w", err)
	}

	mcpSrv := server.NewMCPServer("spectest", buildinfo.App(),
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
	)

	s := &Server{
		resolver:  resolver,
		jobs:      NewJobManager(),
		mcpServer: mcpSrv,
	}

	s.registerReadTools()
	s.registerRunTools()
	s.registerUpdateTools()
	s.registerResources()

	return s, nil
}

// MCPServer exposes the underlying mcp-go server instance.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// Jobs returns the in-memory job manager.
func (s *Server) Jobs() *JobManager {
	return s.jobs
}

// Resolver returns the path resolver.
func (s *Server) Resolver() *PathResolver {
	return s.resolver
}

// ServeStdio launches the JSON-RPC stdio event loop.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}
