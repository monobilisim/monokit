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

	// Import zimbraHealth for the provider implementation and types
	zimbraHealth "github.com/monobilisim/monokit/zimbraHealth"
)

// HealthPlugin is the implementation of plugin.GRPCPlugin and plugin.Plugin
type HealthPlugin struct {
	// GRPCPlugin must be embedded for gRPC communication
	plugin.GRPCPlugin
	// Impl is the actual HealthProvider implementation
	// ZimbraHealthProvider is defined in the zimbraHealth package
	Impl zimbraHealth.ZimbraHealthProvider // The actual implementation
}

// zimbraHealthGRPCServer is an adapter to make zimbraHealth.ZimbraHealthProvider compatible with the gRPC interface.
type zimbraHealthGRPCServer struct {
	// Actual implementation
	Impl                                    zimbraHealth.ZimbraHealthProvider
	proto.UnimplementedHealthProviderServer // Embed for forward compatibility if new methods are added to proto
}

// Name implements the gRPC server interface for Name
func (s *zimbraHealthGRPCServer) Name(ctx context.Context, req *proto.Empty) (*proto.NameReply, error) {
	name := s.Impl.Name()
	return &proto.NameReply{Name: name}, nil
}

// Collect implements the gRPC server interface for Collect (returns rendered CLI string)
func (s *zimbraHealthGRPCServer) Collect(ctx context.Context, req *proto.CollectRequest) (*proto.CollectReply, error) {
	// The zimbraHealth.ZimbraHealthProvider.Collect returns interface{}, which is *zimbraHealth.ZimbraHealthData
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin zimbraHealth Collect failed: %w", err)
	}

	// Assert the type to *zimbraHealth.ZimbraHealthData
	healthData, ok := rawData.(*zimbraHealth.ZimbraHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin zimbraHealth Collect returned unexpected data type: %T", rawData)
	}

	// Render the health data to a CLI string using the function from zimbraHealth package (ui.go)
	// The version "plugin" will be used in the title.
	renderedString := zimbraHealth.RenderZimbraHealthCLI(healthData, common.MonokitVersion)

	// The proto definition expects `bytes json` in CollectReply.
	// We are now sending the rendered string as bytes.
	return &proto.CollectReply{Json: []byte(renderedString)}, nil
}

// CollectStructured implements the gRPC server interface for CollectStructured (returns raw JSON data)
func (s *zimbraHealthGRPCServer) CollectStructured(ctx context.Context, req *proto.CollectRequest) (*proto.CollectStructuredReply, error) {
	// Get the raw data from the implementation
	rawData, err := s.Impl.Collect(req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("plugin zimbraHealth CollectStructured failed: %w", err)
	}

	// Assert the type to *zimbraHealth.ZimbraHealthData
	healthData, ok := rawData.(*zimbraHealth.ZimbraHealthData)
	if !ok {
		return nil, fmt.Errorf("plugin zimbraHealth CollectStructured returned unexpected data type: %T", rawData)
	}

	// Marshal the structured data to JSON for programmatic access
	jsonData, err := json.Marshal(healthData)
	if err != nil {
		return nil, fmt.Errorf("plugin zimbraHealth CollectStructured JSON marshal failed: %w", err)
	}

	return &proto.CollectStructuredReply{Json: jsonData}, nil
}

// GRPCServer registers the HealthProviderServer with the gRPC server
func (p *HealthPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	// Register the zimbraHealthGRPCServer which wraps p.Impl and adapts its methods
	proto.RegisterHealthProviderServer(s, &zimbraHealthGRPCServer{Impl: p.Impl})
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
	// This ensures that zimbraHealth.ZimbraHealthConfig can be populated.
	common.ScriptName = "zimbraHealth-plugin"
	// Plugins should not create/use the global lockfile; skip it to avoid handshake issues
	common.IgnoreLockfile = true
	common.Init() // Initializes logger, TmpDir etc.

	// Set color profile for consistent rendering, but only if colors are enabled
	noColorEnv := strings.ToLower(os.Getenv("MONOKIT_NOCOLOR"))
	if noColorEnv != "1" && noColorEnv != "true" {
		// Colors are enabled, force TrueColor profile for consistent rendering
		lipgloss.SetColorProfile(termenv.TrueColor)
	}

	if common.ConfExists("mail") {
		// Load mail-specific config into the global ZimbraHealthConfig var from provider.go
		// common.ConfInit panics on actual error, so we don't check its return value here.
		// It populates ZimbraHealthConfig by reference.
		common.ConfInit("mail", &zimbraHealth.ZimbraHealthConfig)
		// Mirror into the package-wide MailHealthConfig used by checks when running as plugin
		zimbraHealth.MailHealthConfig = zimbraHealth.ZimbraHealthConfig
	}

	pluginMap := map[string]plugin.Plugin{
		"provider": &HealthPlugin{Impl: zimbraHealth.ZimbraHealthProvider{}},
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
