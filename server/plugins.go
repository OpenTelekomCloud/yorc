package server

import (
	"os"
	"path/filepath"

	gplugin "github.com/hashicorp/go-plugin"
	"github.com/pkg/errors"

	"novaforge.bull.com/starlings-janus/janus/config"
	"novaforge.bull.com/starlings-janus/janus/log"
	"novaforge.bull.com/starlings-janus/janus/plugin"
	"novaforge.bull.com/starlings-janus/janus/prov/registry"

	// Registering openstack delegate executor in the registry
	_ "novaforge.bull.com/starlings-janus/janus/prov/terraform/openstack"
)

type pluginManager struct {
	pluginClients []*gplugin.Client
	shutdownChan  chan struct{}
}

func newPluginManager(shutdownChan chan struct{}) *pluginManager {
	pm := &pluginManager{
		pluginClients: make([]*gplugin.Client, 0),
		shutdownChan:  shutdownChan,
	}
	go pm.cleanup()
	return pm
}

func (pm *pluginManager) cleanup() {
	<-pm.shutdownChan
	for _, client := range pm.pluginClients {
		client.Kill()
	}
	pm.pluginClients = nil
}

func (pm *pluginManager) loadPlugins(cfg config.Configuration) error {
	pluginsPath := cfg.PluginsDirectory
	if pluginsPath == "" {
		pluginsPath = config.DefaultPluginDir
	}
	pluginPath, err := filepath.Abs(pluginsPath)
	if err != nil {
		return errors.Wrap(err, "Failed to explore plugins directory")
	}
	pluginsFiles, err := filepath.Glob(filepath.Join(pluginPath, "*"))
	if err != nil {
		return errors.Wrap(err, "Failed to explore plugins directory")
	}
	plugins := make([]string, 0)
	for _, pFile := range pluginsFiles {
		fInfo, err := os.Stat(pFile)
		if err != nil {
			return errors.Wrap(err, "Failed to explore plugins directory")
		}
		if !fInfo.IsDir() && fInfo.Mode().Perm()&0111 != 0 {
			plugins = append(plugins, pFile)
		}
	}
	reg := registry.GetRegistry()
	for _, pFile := range plugins {
		log.Debugf("Loading plugin %q...", pFile)
		client := plugin.NewClient(pFile)
		pm.pluginClients = append(pm.pluginClients, client)
		// Connect via RPC
		rpcClient, err := client.Client()
		if err != nil {
			return errors.Wrapf(err, "Failed to load plugin %q", pFile)
		}

		// Request the delegate plugin
		raw, err := rpcClient.Dispense(plugin.DelegatePluginName)
		if err != nil {
			return errors.Wrapf(err, "Failed to load plugin %q", pFile)
		}

		delegateExecutor := raw.(plugin.DelegateExecutor)
		supportedTypes, err := delegateExecutor.GetSupportedTypes()
		if err != nil {
			return errors.Wrap(err, "Failed to retrieve supported type for delegate")
		}

		reg.RegisterDelegates(supportedTypes, delegateExecutor, filepath.Base(pFile))

	}

	return nil
}
