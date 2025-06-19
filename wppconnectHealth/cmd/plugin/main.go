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

	// Import wppconnectHealth for the provider implementation and types
	wppconnectHealth "github.com/monobilisim/monokit/wppconnectHealth"
)

// HealthPlugin is the implementation of plugin.GRPCPlugin and plugin.Plugin
type HealthPlugin struct {
	// GRPCPlugin must be embedded for gRPC communication
	plugin.GRPCPlugin
	// Impl is the actual HealthProvider implementation
	Impl wppconnectHealth.WppConnectHealthProvider
}

// wppconnectHealthGRPCServer is an adapter to make wppconnectHealth.WppConnectHealthProvider compatible with the gRPC interface.
type wppconnectHealthGRPCServer struct {
	// Actual implementation
	Impl                                    wppconnectHealth.WppConnectHealthProvider
	proto.UnimplementedHealthProviderServer // Embed for forward compatibility
}

// Name implements the gRPC server interface for Name
func (s *wppconnectHealthGRPCServer) Name(ctx context.Context, req *proto.Empty) (*proto.NameReply, error) {
	name := s.Impl.Name()
	return &proto.NameReply{Name: name}, nil
}

// Collect implements the gRPC server interface for Collect (returns rendered CLI string)
func (s *wppconnectHealthGRPCServer) Collect(ctx context.Context, req *proto.CollectRequest) (*proto.CollectReply, error) {
	// The wppconnectHealth.WppConnectHealthProvider.Collect returns interface{}, which is *wppconnectHealth.WppConnectHealthData
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin wppconnectHealth Collect failed: %w", err)
	}

	// Assert the type to *wppconnectHealth.WppConnectHealthData
	healthData, ok := rawData.(*wppconnectHealth.WppConnectHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin wppconnectHealth Collect returned unexpected data type: %T", rawData)
	}

	// Render the health data to a CLI string using the function from wppconnectHealth package (ui.go)
	renderedString := wppconnectHealth.RenderWppConnectHealthCLI(healthData, common.MonokitVersion)

	// The proto definition expects `bytes json` in CollectReply.
	// We are now sending the rendered string as bytes.
	return &proto.CollectReply{Json: []byte(renderedString)}, nil
}

// CollectStructured implements the gRPC server interface for CollectStructured (returns raw JSON data)
func (s *wppconnectHealthGRPCServer) CollectStructured(ctx context.Context, req *proto.CollectRequest) (*proto.CollectStructuredReply, error) {
	// Get the raw data from the implementation
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin wppconnectHealth CollectStructured failed: %w", err)
	}

	// Assert the type to *wppconnectHealth.WppConnectHealthData
	healthData, ok := rawData.(*wppconnectHealth.WppConnectHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin wppconnectHealth CollectStructured returned unexpected data type: %T", rawData)
	}

	// Marshal the structured data to JSON for programmatic access
	jsonData, err := json.Marshal(healthData)
	if err != nil {
		return nil, fmt.Errorf("plugin wppconnectHealth CollectStructured JSON marshal failed: %w", err)
	}

	return &proto.CollectStructuredReply{Json: jsonData}, nil
}

// GRPCServer registers the HealthProviderServer with the gRPC server
func (p *HealthPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Register the wppconnectHealthGRPCServer which wraps p.Impl and adapts its methods
	proto.RegisterHealthProviderServer(s, &wppconnectHealthGRPCServer{Impl: p.Impl})
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
	// This ensures that wppconnectHealth.WppConnectHealthConfig can be populated.
	common.ScriptName = "wppconnectHealth-plugin"
	common.Init() // Initializes logger, TmpDir etc.

	// Set color profile for consistent rendering, but only if colors are enabled
	noColorEnv := strings.ToLower(os.Getenv("MONOKIT_NOCOLOR"))
	if noColorEnv != "1" && noColorEnv != "true" {
		// Colors are enabled, force TrueColor profile for consistent rendering
		lipgloss.SetColorProfile(termenv.TrueColor)
		common.LogDebug("[PLUGIN] Colors enabled, set lipgloss to TrueColor profile")
	} else {
		common.LogDebug("[PLUGIN] Colors disabled via MONOKIT_NOCOLOR")
	}

	if common.ConfExists("wppconnect") {
		// Load wppconnect-specific config into the global WppConnectHealthConfig var from types.go
		// common.ConfInit panics on actual error, so we don't check its return value here.
		// It populates WppConnectHealthConfig by reference.
		common.ConfInit("wppconnect", &wppconnectHealth.WppConnectHealthConfig)
		common.LogDebug("[PLUGIN] wppconnect config loaded into wppconnectHealth.WppConnectHealthConfig")
	} else {
		common.LogDebug("wppconnect config not found, plugin will use default/empty WppConnectHealthConfig")
	}

	pluginMap := map[string]plugin.Plugin{
		"provider": &HealthPlugin{Impl: wppconnectHealth.WppConnectHealthProvider{}},
	}

	// Handshake config must match the host app
	var handshake = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "MONOKIT_HEALTH",
		MagicCookieValue: "1",
	}

	common.LogDebug("[PLUGIN] wppconnectHealth plugin starting, serving gRPC plugin...")

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
