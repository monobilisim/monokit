//go:build plugin

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/rpc"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hashicorp/go-plugin"
	"github.com/monobilisim/monokit/common"
	proto "github.com/monobilisim/monokit/common/health/pluginpb/proto"
	"github.com/muesli/termenv"
	"google.golang.org/grpc"

	// Import pritunlHealth for the provider implementation and types
	pritunlHealth "github.com/monobilisim/monokit/pritunlHealth"
)

// HealthPlugin is the implementation of plugin.GRPCPlugin and plugin.Plugin
type HealthPlugin struct {
	// GRPCPlugin must be embedded for gRPC communication
	plugin.GRPCPlugin
	// Impl is the actual HealthProvider implementation
	Impl pritunlHealth.PritunlHealthProvider
}

// pritunlHealthGRPCServer is an adapter to make pritunlHealth.PritunlHealthProvider compatible with the gRPC interface.
type pritunlHealthGRPCServer struct {
	// Actual implementation
	Impl                                    pritunlHealth.PritunlHealthProvider
	proto.UnimplementedHealthProviderServer // Embed for forward compatibility
}

// Name implements the gRPC server interface for Name
func (s *pritunlHealthGRPCServer) Name(ctx context.Context, req *proto.Empty) (*proto.NameReply, error) {
	name := s.Impl.Name()
	return &proto.NameReply{Name: name}, nil
}

// Collect implements the gRPC server interface for Collect (returns rendered CLI string)
func (s *pritunlHealthGRPCServer) Collect(ctx context.Context, req *proto.CollectRequest) (*proto.CollectReply, error) {
	// The pritunlHealth.PritunlHealthProvider.Collect returns interface{}, which is *pritunlHealth.PritunlHealthData
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin pritunlHealth Collect failed: %w", err)
	}

	// Assert the type to *pritunlHealth.PritunlHealthData
	healthData, ok := rawData.(*pritunlHealth.PritunlHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin pritunlHealth Collect returned unexpected data type: %T", rawData)
	}

	// Render the health data to a CLI string using the function from pritunlHealth package (ui.go)
	renderedString := pritunlHealth.RenderPritunlHealthCLI(healthData, common.MonokitVersion)

	// The proto definition expects `bytes json` in CollectReply.
	// We are now sending the rendered string as bytes.
	return &proto.CollectReply{Json: []byte(renderedString)}, nil
}

// CollectStructured implements the gRPC server interface for CollectStructured (returns raw JSON data)
func (s *pritunlHealthGRPCServer) CollectStructured(ctx context.Context, req *proto.CollectRequest) (*proto.CollectStructuredReply, error) {
	// Get the raw data from the implementation
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin pritunlHealth CollectStructured failed: %w", err)
	}

	// Assert the type to *pritunlHealth.PritunlHealthData
	healthData, ok := rawData.(*pritunlHealth.PritunlHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin pritunlHealth CollectStructured returned unexpected data type: %T", rawData)
	}

	// Marshal the structured data to JSON for programmatic access
	jsonData, err := json.Marshal(healthData)
	if err != nil {
		return nil, fmt.Errorf("plugin pritunlHealth CollectStructured JSON marshal failed: %w", err)
	}

	return &proto.CollectStructuredReply{Json: jsonData}, nil
}

// GRPCServer registers the HealthProviderServer with the gRPC server
func (p *HealthPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Register the pritunlHealthGRPCServer which wraps p.Impl and adapts its methods
	proto.RegisterHealthProviderServer(s, &pritunlHealthGRPCServer{Impl: p.Impl})
	return nil
}

// GRPCClient returns a new HealthProvider client
func (p *HealthPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return proto.NewHealthProviderClient(c), nil
}

// --- plugin.Plugin interface (netrpc stubs for compatibility) ---

// Server is a dummy netrpc server implementation.
func (p *HealthPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return nil, fmt.Errorf("netrpc server not implemented for gRPC plugin")
}

// Client is a dummy netrpc client implementation.
func (p *HealthPlugin) Client(*plugin.MuxBroker, *rpc.Client) (interface{}, error) {
	return nil, fmt.Errorf("netrpc client not implemented for gRPC plugin")
}

func main() {
	// Initialize common components for the plugin, like logger and config loading.
	// This ensures that pritunlHealth.PritunlHealthConfig can be populated.
	common.ScriptName = "pritunlHealth-plugin"
	common.Init() // Initializes logger, TmpDir etc.

	// Set color profile for consistent rendering, but only if colors are enabled
	noColorEnv := strings.ToLower(os.Getenv("MONOKIT_NOCOLOR"))
	if noColorEnv != "1" && noColorEnv != "true" {
		// Colors are enabled, force TrueColor profile for consistent rendering
		lipgloss.SetColorProfile(termenv.TrueColor)
	}

	if common.ConfExists("pritunl") {
		// Load pritunl-specific config into the global PritunlHealthConfig var from types.go
		// common.ConfInit panics on actual error, so we don't check its return value here.
		// It populates PritunlHealthConfig by reference.
		common.ConfInit("pritunl", &pritunlHealth.PritunlHealthConfig)
	}

	pluginMap := map[string]plugin.Plugin{
		"provider": &HealthPlugin{Impl: pritunlHealth.PritunlHealthProvider{}},
	}

	// Handshake config must match the host app
	var handshake = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "MONOKIT_HEALTH",
		MagicCookieValue: "1",
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
