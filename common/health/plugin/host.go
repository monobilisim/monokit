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
	pb "github.com/monobilisim/monokit/common/health/pluginpb/proto"

	"github.com/rs/zerolog/log"
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
		log.Error().Str("name", p.name).Str("hostname", hostname).Err(err).Msg("Plugin RPC call to Collect failed")
		return nil, fmt.Errorf("plugin RPC call to Collect failed for %s: %w", p.name, err)
	}

	// The plugin now sends a pre-rendered string.
	// The host will just pass this string through.
	// The `resp.Json` field (bytes) directly contains the rendered UI string.
	return string(resp.Json), nil
}

// CollectStructured gets structured data from the plugin for programmatic access (e.g., versionCheck)
func (p *ProviderProxy) CollectStructured(hostname string) (interface{}, error) {
	req := &pb.CollectRequest{Hostname: hostname}
	resp, err := p.client.CollectStructured(context.Background(), req)
	if err != nil {
		log.Error().Str("name", p.name).Str("hostname", hostname).Err(err).Msg("Plugin RPC call to CollectStructured failed")
		return nil, fmt.Errorf("plugin RPC call to CollectStructured failed for %s: %w", p.name, err)
	}

	// Return the raw JSON data as bytes for the caller to unmarshal
	return resp.Json, nil
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
	log.Debug().Msg(fmt.Sprintf("Loading plugins from directory: %s", dir))

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Msg(fmt.Sprintf("Plugin directory does not exist, skipping: %s", dir))
			return nil // no plugins dir, that's fine
		}
		log.Error().Str("dir", dir).Err(err).Msg("Failed to read plugin directory")
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
			log.Error().Str("path", path).Err(err).Msg("Error starting plugin")
			cli.Kill() // Clean up the plugin even if it errored out, as it may still be running
			continue
		}
		raw, err := rpcClient.Dispense("provider")
		if err != nil {
			log.Error().Str("path", path).Err(err).Msg("Error dispensing provider")
			cli.Kill() // Clean up the plugin even if it errored out, as it may still be running
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

		log.Debug().Str("name", pName).Msg("Successfully loaded plugin")
		loadedCount++
	}

	if loadedCount > 0 {
		log.Debug().Int("loadedCount", loadedCount).Int("entries", len(entries)).Str("dir", dir).Msg("Plugin loading completed")
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

	log.Debug().Int("count", len(pluginClients)).Msg("Shutting down plugins")

	for _, client := range pluginClients {
		client.Kill()
	}

	pluginClients = nil
	log.Debug().Msg("All plugins shut down")
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
