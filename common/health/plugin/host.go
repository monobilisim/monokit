// Package plugin provides host glue to load health providers as go-plugin plugins.
package plugin

import (
	"context"
	"fmt"
	"io"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/monobilisim/monokit/common"
	pb "github.com/monobilisim/monokit/common/health/pluginpb/proto"

	// k8sHealthTypes "github.com/monobilisim/monokit/k8sHealth" // Removed for full plugin independence
	"google.golang.org/grpc"
)

// ProviderProxy implements health.Provider and proxies to a plugin.
type ProviderProxy struct {
	name   string
	client pb.HealthProviderClient
	raw    *plugin.Client
}

func (p *ProviderProxy) Name() string {
	return p.name
}
func (p *ProviderProxy) Collect(hostname string) (interface{}, error) {
	req := &pb.CollectRequest{Hostname: hostname}
	resp, err := p.client.Collect(context.Background(), req)
	if err != nil {
		common.LogError(fmt.Sprintf("Plugin RPC call to Collect failed for %s (hostname: %s): %v", p.name, hostname, err))
		return nil, fmt.Errorf("plugin RPC call to Collect failed for %s: %w", p.name, err)
	}

	// The plugin now sends a pre-rendered string.
	// The host will just pass this string through.
	// The `resp.Json` field (bytes) directly contains the rendered UI string.
	return string(resp.Json), nil
}

var (
	pluginClients []*plugin.Client
	pluginMu      sync.Mutex
)

func handshakeConfig() plugin.HandshakeConfig {
	return plugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "MONOKIT_HEALTH",
		MagicCookieValue: "1",
	}
}

// LoadAll scans dir for plugins, loads them, and registers their providers.
func LoadAll(dir string) error {
	common.LogDebug(fmt.Sprintf("Loading plugins from directory: %s", dir))

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			common.LogDebug(fmt.Sprintf("Plugin directory does not exist, skipping: %s", dir))
			return nil // no plugins dir, that's fine
		}
		common.LogError(fmt.Sprintf("Failed to read plugin directory %s: %v", dir, err))
		return err
	}

	// Create a quiet logger for plugin framework
	pluginLogger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: io.Discard, // Suppress all plugin framework logs
		Level:  hclog.Off,
	})

	loadedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		cli := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig: handshakeConfig(),
			Plugins: map[string]plugin.Plugin{
				"provider": &healthPluginGRPC{},
			},
			Cmd:              exec.Command(path),
			AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
			SyncStdout:       os.Stdout,
			SyncStderr:       os.Stderr,
			Logger:           pluginLogger, // Use our quiet logger
		})
		rpcClient, err := cli.Client()
		if err != nil {
			common.LogError(fmt.Sprintf("Error starting plugin '%s': %v", path, err))
			continue
		}
		raw, err := rpcClient.Dispense("provider")
		if err != nil {
			common.LogError(fmt.Sprintf("Error dispensing provider '%s': %v", path, err))
			continue
		}
		grpcClient := raw.(pb.HealthProviderClient)
		// Try to get canonical provider name
		pName := entry.Name()
		if nmResp, err := grpcClient.Name(context.Background(), &pb.Empty{}); err == nil && nmResp != nil {
			pName = nmResp.Name
		}
		provider := &ProviderProxy{
			name:   pName,
			client: grpcClient,
			raw:    cli,
		}
		// Register via global health registry setter (main package must wire this)
		RegisterProviderGlobally(provider)
		pluginMu.Lock()
		pluginClients = append(pluginClients, cli)
		pluginMu.Unlock()

		common.LogDebug(fmt.Sprintf("Successfully loaded plugin: %s", pName))
		loadedCount++
	}

	if loadedCount > 0 {
		common.LogDebug(fmt.Sprintf("Plugin loading completed: %d/%d plugins loaded from %s", loadedCount, len(entries), dir))
	}

	return nil
}

// CleanupAll shuts down all loaded plugins gracefully
func CleanupAll() {
	pluginMu.Lock()
	defer pluginMu.Unlock()

	if len(pluginClients) == 0 {
		return
	}

	common.LogDebug(fmt.Sprintf("Shutting down %d plugins", len(pluginClients)))

	for _, client := range pluginClients {
		client.Kill()
	}

	pluginClients = nil
	common.LogDebug("All plugins shut down")
}

// healthPluginGRPC is the host-side struct that satisfies both plugin.Plugin
// (for netrpc, with dummy implementations) and plugin.GRPCPlugin (for gRPC).
// This is required by plugin.ClientConfig.Plugins map.
type healthPluginGRPC struct{}

// --- plugin.GRPCPlugin interface ---

// GRPCServer is a dummy implementation for the host side.
// The actual server is run by the plugin process.
func (p *healthPluginGRPC) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return fmt.Errorf("host should not be a GRPC server for this plugin")
}

// GRPCClient returns the gRPC client for the HealthProvider service.
func (p *healthPluginGRPC) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pb.NewHealthProviderClient(c), nil
}

// --- plugin.Plugin interface (netrpc stubs) ---

// Server is a dummy netrpc server implementation.
func (p *healthPluginGRPC) Server(*plugin.MuxBroker) (interface{}, error) {
	return nil, fmt.Errorf("netrpc server not implemented for gRPC plugin")
}

// Client is a dummy netrpc client implementation.
func (p *healthPluginGRPC) Client(*plugin.MuxBroker, *rpc.Client) (interface{}, error) {
	return nil, fmt.Errorf("netrpc client not implemented for gRPC plugin")
}

// RegisterProviderGlobally must be set by the main program to route registration to health registry.
// This avoids import cycles between this glue package and main/health.
var RegisterProviderGlobally = func(p interface{}) {
	panic("You must set plugin.RegisterProviderGlobally = health.Register or equivalent prior to LoadAll.")
}
