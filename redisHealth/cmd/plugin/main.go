//go:build plugin && linux

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

	// Import redisHealth for the provider implementation and types
	redisHealth "github.com/monobilisim/monokit/redisHealth"
)

// HealthPlugin is the implementation of plugin.GRPCPlugin and plugin.Plugin
type HealthPlugin struct {
	// GRPCPlugin must be embedded for gRPC communication
	plugin.GRPCPlugin
	// Impl is the actual HealthProvider implementation
	// RedisHealthProvider is defined in the redisHealth package
	Impl redisHealth.RedisHealthProvider // The actual implementation
}

// redisHealthGRPCServer is an adapter to make redisHealth.RedisHealthProvider compatible with the gRPC interface.
type redisHealthGRPCServer struct {
	// Actual implementation
	Impl                                    redisHealth.RedisHealthProvider
	proto.UnimplementedHealthProviderServer // Embed for forward compatibility if new methods are added to proto
}

// Name implements the gRPC server interface for Name
func (s *redisHealthGRPCServer) Name(ctx context.Context, req *proto.Empty) (*proto.NameReply, error) {
	name := s.Impl.Name()
	return &proto.NameReply{Name: name}, nil
}

// Collect implements the gRPC server interface for Collect (returns rendered CLI string)
func (s *redisHealthGRPCServer) Collect(ctx context.Context, req *proto.CollectRequest) (*proto.CollectReply, error) {
	// The redisHealth.RedisHealthProvider.Collect returns interface{}, which is *redisHealth.RedisHealthData
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin redisHealth Collect failed: %w", err)
	}

	// Assert the type to *redisHealth.RedisHealthData
	healthData, ok := rawData.(*redisHealth.RedisHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin redisHealth Collect returned unexpected data type: %T", rawData)
	}

	// Render the health data to a CLI string using the function from redisHealth package (ui.go)
	// The version "plugin" will be used in the title.
	renderedString := redisHealth.RenderRedisHealthCLI(healthData, common.MonokitVersion)

	// The proto definition expects `bytes json` in CollectReply.
	// We are now sending the rendered string as bytes.
	return &proto.CollectReply{Json: []byte(renderedString)}, nil
}

// CollectStructured implements the gRPC server interface for CollectStructured (returns raw JSON data)
func (s *redisHealthGRPCServer) CollectStructured(ctx context.Context, req *proto.CollectRequest) (*proto.CollectStructuredReply, error) {
	// Get the raw data from the implementation
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin redisHealth CollectStructured failed: %w", err)
	}

	// Assert the type to *redisHealth.RedisHealthData
	healthData, ok := rawData.(*redisHealth.RedisHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin redisHealth CollectStructured returned unexpected data type: %T", rawData)
	}

	// Marshal the structured data to JSON for programmatic access
	jsonData, err := json.Marshal(healthData)
	if err != nil {
		return nil, fmt.Errorf("plugin redisHealth CollectStructured JSON marshal failed: %w", err)
	}

	return &proto.CollectStructuredReply{Json: jsonData}, nil
}

// GRPCServer registers the HealthProviderServer with the gRPC server
func (p *HealthPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Register the redisHealthGRPCServer which wraps p.Impl and adapts its methods
	proto.RegisterHealthProviderServer(s, &redisHealthGRPCServer{Impl: p.Impl})
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
	// This ensures that redisHealth.RedisHealthConfig can be populated.
	common.ScriptName = "redisHealth-plugin"
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

	if common.ConfExists("redis") {
		// Load redis-specific config into the global RedisHealthConfig var from types.go
		// common.ConfInit panics on actual error, so we don't check its return value here.
		// It populates RedisHealthConfig by reference.
		common.ConfInit("redis", &redisHealth.RedisHealthConfig)
		common.LogDebug("[PLUGIN] redis config loaded into redisHealth.RedisHealthConfig")
	} else {
		common.LogDebug("redis config not found, plugin will use default/empty RedisHealthConfig")
	}

	pluginMap := map[string]plugin.Plugin{
		"provider": &HealthPlugin{Impl: redisHealth.RedisHealthProvider{}},
	}

	// Handshake config must match the host app
	var handshake = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "MONOKIT_HEALTH",
		MagicCookieValue: "1",
	}

	common.LogDebug("[PLUGIN] redisHealth plugin starting, serving gRPC plugin...")

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
