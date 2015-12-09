package pluginmanager

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// The lifecycle plugin Impl
type LifecycleAppInstance interface {
	Start(data []byte) error
	Stop(data []byte) error
}

// The singularity plugin Impl
type SingularityPluginImpl struct {
	pluginReg                    *PluginImpl
	controllerInstanceRegisterer func([]byte) (interface{}, error)
	controllerInstanceMap        map[string]interface{}
}

const (
	runtimeConf = "runtime.conf"
)

var (
	singularityPlugin *SingularityPluginImpl
)

// Function to register a plugin
func RegisterPlugin(registrar func([]byte) (interface{}, error)) (*SingularityPluginImpl, error) {

	pluginConf := PluginImplConf{PluginLoc: runtimeConf, Activator: pluginStarter, Stopper: pluginStopper}
	// Implement the Plugin
	regPlugin, pluginInitError := PluginInit(pluginConf)
	if pluginInitError != nil {
		return nil, fmt.Errorf("Failed to initialize the plugin: %v", pluginInitError)
	}

	singularityPluginImpl := &SingularityPluginImpl{}
	singularityPluginImpl.pluginReg = regPlugin
	singularityPluginImpl.controllerInstanceRegisterer = registrar
	singularityPluginImpl.controllerInstanceMap = make(map[string]interface{})

	singularityPlugin = singularityPluginImpl

	return singularityPluginImpl, nil
}

func pluginStarter(data []byte) []byte {
	return nil
}

func pluginStopper(data []byte) []byte {
	return nil
}

func manageInit(reqdata []byte) []byte {

	controllerid, data, err := decapsuleControllerId(reqdata)
	if err != nil {
		return []byte(fmt.Sprintf("Failed to decapsule controllerid: %s", err))
	}

	lifecycleinstance, initerr := singularityPlugin.controllerInstanceRegisterer(data)
	if initerr != nil {
		return []byte(fmt.Sprintf("failed to initialize controller instance: %s", initerr))
	}

	singularityPlugin.controllerInstanceMap[controllerid] = lifecycleinstance

	return nil
}

func manageStart(reqdata []byte) []byte {

	controllerid, data, decodeerr := decapsuleControllerId(reqdata)
	if decodeerr != nil {
		return []byte(fmt.Sprintf("Failed to decalsule controllerid %s", decodeerr))
	}

	// Get the lifecycleinstance from the map
	controllerInstance, found := singularityPlugin.controllerInstanceMap[controllerid]
	if !found {
		return []byte(fmt.Sprintf("Appinstance not initialized"))
	}

	lifecycleApp := controllerInstance.(LifecycleAppInstance)

	err := lifecycleApp.Start(data)
	retData := []byte(fmt.Sprintf("%v", err))
	return retData
}

func manageStop(reqdata []byte) []byte {

	controllerid, data, decodeerr := decapsuleControllerId(reqdata)
	if decodeerr != nil {
		return []byte(fmt.Sprintf("Failed to decalsule controllerid %s", decodeerr))
	}

	// Get the lifecycleinstance from the map
	controllerInstance, found := singularityPlugin.controllerInstanceMap[controllerid]
	if !found {
		return []byte(fmt.Sprintf("Appinstance not initialized"))
	}

	lifecycleApp := controllerInstance.(LifecycleAppInstance)

	err := lifecycleApp.Stop(data)
	retData := []byte(fmt.Sprintf("%v", err))
	return retData
}

// Function to start a plugin
func (plugin *SingularityPluginImpl) StartPlugin() error {

	plugReg := plugin.pluginReg

	// Start the plugin
	(*plugReg).Start()
	return nil
}

// Function to wait for a plugin to stop. The wait finish when the plugin gets a SIGUSR1 from agent
func (plugin *SingularityPluginImpl) WaitForPluginStop() error {
	pluginExitChannel := makeExitChannel()
	// We block on this channel
	<-pluginExitChannel
	plugReg := plugin.pluginReg
	// Stop the plugin server
	(*plugReg).Stop()
	return nil
}

func makeExitChannel() chan os.Signal {
	//channel for catching signals of interest
	signalCatchingChannel := make(chan os.Signal)

	//catch Ctrl-C and Kill -30 <pid> signals
	signal.Notify(signalCatchingChannel, syscall.SIGUSR1, syscall.SIGTERM)

	return signalCatchingChannel
}
