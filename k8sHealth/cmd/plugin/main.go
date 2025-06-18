//go:build plugin

package main

import (
	"context" // Added for GRPCClient
	"encoding/json"
	"fmt"     // For fmt.Errorf in stubs
	"net/rpc" // For plugin.Plugin interface (for netrpc stubs)
	"os"      // Add for os.Getenv
	"strings" // Add for strings.ToLower

	"github.com/charmbracelet/lipgloss" // Add for color profile setting
	"github.com/hashicorp/go-plugin"
	"github.com/monobilisim/monokit/common"                             // Added for common.Init and ConfInit
	proto "github.com/monobilisim/monokit/common/health/pluginpb/proto" // Added proto alias
	"github.com/muesli/termenv"                                         // Add for color profile constants
	"google.golang.org/grpc"

	// Import k8sHealth for the provider implementation and types
	k8sHealth "github.com/monobilisim/monokit/k8sHealth"
)

// HealthPlugin is the implementation of plugin.GRPCPlugin and plugin.Plugin
type HealthPlugin struct {
	// GRPCPlugin must be embedded for gRPC communication
	plugin.GRPCPlugin
	// Impl is the actual HealthProvider implementation
	// K8sHealthProvider is defined in the k8sHealth package
	Impl k8sHealth.K8sHealthProvider // The actual implementation
}

// k8sHealthGRPCServer is an adapter to make k8sHealth.K8sHealthProvider compatible with the gRPC interface.
type k8sHealthGRPCServer struct {
	// Actual implementation
	Impl                                    k8sHealth.K8sHealthProvider
	proto.UnimplementedHealthProviderServer // Embed for forward compatibility if new methods are added to proto
}

// Name implements the gRPC server interface for Name
func (s *k8sHealthGRPCServer) Name(ctx context.Context, req *proto.Empty) (*proto.NameReply, error) {
	name := s.Impl.Name()
	return &proto.NameReply{Name: name}, nil
}

// Collect implements the gRPC server interface for Collect (returns rendered CLI string)
func (s *k8sHealthGRPCServer) Collect(ctx context.Context, req *proto.CollectRequest) (*proto.CollectReply, error) {
	// The k8sHealth.K8sHealthProvider.Collect returns interface{}, which is *k8sHealth.K8sHealthData
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin k8sHealth Collect failed: %w", err)
	}

	// Assert the type to *k8sHealth.K8sHealthData
	healthData, ok := rawData.(*k8sHealth.K8sHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin k8sHealth Collect returned unexpected data type: %T", rawData)
	}

	// Render the health data to a CLI string using the function from k8sHealth package (ui.go)
	// The version "plugin" will be used in the title.
	renderedString := k8sHealth.RenderK8sHealthCLI(healthData, common.MonokitVersion)

	// The proto definition expects `bytes json` in CollectReply.
	// We are now sending the rendered string as bytes.
	return &proto.CollectReply{Json: []byte(renderedString)}, nil
}

// CollectStructured implements the gRPC server interface for CollectStructured (returns raw JSON data)
func (s *k8sHealthGRPCServer) CollectStructured(ctx context.Context, req *proto.CollectRequest) (*proto.CollectStructuredReply, error) {
	// Get the raw data from the implementation
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin k8sHealth CollectStructured failed: %w", err)
	}

	// Assert the type to *k8sHealth.K8sHealthData
	healthData, ok := rawData.(*k8sHealth.K8sHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin k8sHealth CollectStructured returned unexpected data type: %T", rawData)
	}

	// Marshal the structured data to JSON for programmatic access
	jsonData, err := json.Marshal(healthData)
	if err != nil {
		return nil, fmt.Errorf("plugin k8sHealth CollectStructured JSON marshal failed: %w", err)
	}

	return &proto.CollectStructuredReply{Json: jsonData}, nil
}

// GRPCServer registers the HealthProviderServer with the gRPC server
func (p *HealthPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Register the k8sHealthGRPCServer which wraps p.Impl and adapts its methods
	proto.RegisterHealthProviderServer(s, &k8sHealthGRPCServer{Impl: p.Impl})
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
	// This ensures that k8sHealth.K8sHealthConfig can be populated.
	common.ScriptName = "k8sHealth-plugin"
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

	if common.ConfExists("k8s") {
		// Load k8s-specific config into the global K8sHealthConfig var from types.go
		// common.ConfInit panics on actual error, so we don't check its return value here.
		// It populates K8sHealthConfig by reference.
		common.ConfInit("k8s", &k8sHealth.K8sHealthConfig)
		common.LogDebug("[PLUGIN] k8s config loaded into k8sHealth.K8sHealthConfig")
	} else {
		common.LogDebug("k8s config not found, plugin will use default/empty K8sHealthConfig")
	}

	pluginMap := map[string]plugin.Plugin{
		"provider": &HealthPlugin{Impl: k8sHealth.K8sHealthProvider{}},
	}

	// Handshake config must match the host app
	var handshake = plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "MONOKIT_HEALTH",
		MagicCookieValue: "1",
	}

	common.LogDebug("[PLUGIN] k8sHealth plugin starting, serving gRPC plugin...")

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
