package pluginmanager

import (
	"errors"
	"fmt"
	log "github.com/spf13/jwalterweatherman"
	store "org.openappstack/singularity/store"
)

type AppPlugin struct {
	appName string
}

// Manage Plugin Interface
type ManagePlugin interface {
	Init(controllerId string, data []byte) error
	Start(controllerId string, data []byte) error
	Stop(controllerId string, data []byte) error
}

type PluginStore struct {
	pluginReg *PluginReg
	// Map of all plugin mapped againest a instance Id
	allManagePlugins map[*ControllerInfo]*Plugin
	// The kvstore
	kvstore *store.KVStore
}

type MonitorPluginInstance struct {
	plugin *Plugin
}

type ManagePluginInstance struct {
	plugin *Plugin
}

var (
	NotInitialized error = errors.New("Controller Not initialized")
)

var pluginStore *PluginStore

/* Function to initialize the singularity Plugin store */
func PluginStoreInit(kvstore *store.KVStore) error {

	// init Monitor plugin store
	conf := PluginRegConf{PluginLocation: "plugin"}
	pluginReg, regInitErr := PluginRegInit(conf)
	if regInitErr != nil {
		log.ERROR.Printf("Pluginregistry init failed: %v", regInitErr)
		return fmt.Errorf("Plugin Store init failed: %v", regInitErr)
	}
	pluginStore = &PluginStore{}

	pluginStore.pluginReg = pluginReg
	pluginStore.kvstore = kvstore
	pluginStore.allManagePlugins = make(map[*ControllerInfo]*Plugin)

	// Initialize plugin store from kvstore
	err := loadPluginstoreFromKvstore()
	if err != nil {
		log.ERROR.Printf("Pluginstore load from kvstore failed: %v", err)
		return fmt.Errorf("KVStore load failed: %v", err)
	}

	return nil
}

/* Load plugin store from the kvstore */
func loadPluginstoreFromKvstore() error {
	err := pluginStore.kvstore.GetAll(store.Plugin_instances_bucket, saveToStore)
	if err != nil {
		return fmt.Errorf("Failed to save to store : %v", err)
	}
	return nil
}

/* save data to the store */
func saveToStore(k, v []byte) error {
	controllerInfo := ControllerInfo{}
	LoadInterface(k, &controllerInfo)
	plugin := &Plugin{}
	LoadInterface(v, plugin)
	// Connect to the plugin
	connErr := plugin.ReConnect()
	if connErr != nil {
		err := plugin.ReloadPlugin()
		if err != nil {
			log.ERROR.Printf("Failed to connect : %v", connErr)
		}
	}
	switch controllerInfo.plugtype {
	case "manage":
		pluginStore.allManagePlugins[&controllerInfo] = plugin
		break
	}
	return nil
}

/* Function to Get a specific Manage Plugin To execute a request */
func GetManagePlugin(controller string, version string) (ManagePlugin, error) {

	pluginReg := pluginStore.pluginReg

	// Check if already any plugin is running
	plugin := getLoadedPlugin("lifecycle", controller, version)
	if plugin != nil {
		appPlugin := &ManagePluginInstance{plugin}
		return ManagePlugin(appPlugin), nil
	}

	// get the plugin from the plugin reg
	plugin, loadErr := pluginReg.LoadPluginInstance("lifecycle", controller, version)
	if loadErr != nil {
		return nil, fmt.Errorf("Plugin could not be loaded: %v", loadErr)
	}

	controllerInfo := &ControllerInfo{controller, plugin.Version, "manage"}

	// Store in the all plugin list
	pluginStore.allManagePlugins[controllerInfo] = plugin

	// Set the plugin in the kvstore
	setErr := pluginStore.kvstore.Set(store.Plugin_instances_bucket, getBytes(&plugin.Version), getBytes(plugin))
	if setErr != nil {
		log.ERROR.Printf("Failed to save plugin in kvstore: %v", setErr)
	}

	appPlugin := &ManagePluginInstance{plugin}

	return ManagePlugin(appPlugin), nil
}

/* get a plugin which is already loaded */
func getLoadedPlugin(plugType, controller, version string) *Plugin {
	// check plugin type
	var pluginMap map[*ControllerInfo]*Plugin
	switch plugType {
	case "lifecycle":
		pluginMap = pluginStore.allManagePlugins
		break
	}
	for controllerInfo, plugin := range pluginMap {
		if controllerInfo.Name == controller && isVersionEqual(controllerInfo.version.start, controllerInfo.version.end, version) {
			return plugin
		}
	}
	return nil
}

/* Function to perform init on a Manage Plugin Instance*/
func (appPlugin *ManagePluginInstance) Init(controllerId string, data []byte) error {

	// Get the Plugin
	plugin := appPlugin.plugin

	reqdata, err := encapsuleControllerId(controllerId, data)
	if err != nil {
		return fmt.Errorf("Failed to encapsule controllerId")
	}

	// Execute the Init request
	exeErr, returnByte := plugin.Execute("pluginmanager.manageInit", reqdata)
	if exeErr != nil {
		return fmt.Errorf("Request to plugin could not be made: %v", exeErr)
	}
	// Check if return byte is nil
	if returnByte != nil {
		retString := string(returnByte)
		if retString != "" || retString != "<nil>" {
			return fmt.Errorf(retString)
		}
	}
	return nil
}

/* Function to perform Start on a manage Plugin instance */
func (appPlugin *ManagePluginInstance) Start(controllerId string, data []byte) error {

	// Get the Plugin
	plugin := appPlugin.plugin

	reqdata, err := encapsuleControllerId(controllerId, data)
	if err != nil {
		return fmt.Errorf("Failed to encapsule controllerId")
	}

	// Execute the Init request
	exeErr, returnByte := plugin.Execute("pluginmanager.manageStart", reqdata)
	if exeErr != nil {
		return fmt.Errorf("Request to plugin could not be made: %v", exeErr)
	}
	// Check if return byte is nil
	if returnByte != nil {
		retString := string(returnByte)
		if retString != "" {
			return fmt.Errorf(retString)
		}
	}
	return nil
}

/* Function to perform Stop on a manage Plugin instance */
func (appPlugin *ManagePluginInstance) Stop(controllerId string, data []byte) error {

	// Get the Plugin
	plugin := appPlugin.plugin

	reqdata, err := encapsuleControllerId(controllerId, data)
	if err != nil {
		return fmt.Errorf("Failed to encapsule controllerId")
	}

	// Execute the Init request
	exeErr, returnByte := plugin.Execute("pluginmanager.manageStop", reqdata)
	if exeErr != nil {
		return fmt.Errorf("Request to plugin could not be made: %v", exeErr)
	}
	// Check if return byte is nil
	if returnByte != nil {
		retString := string(returnByte)
		if retString != "" {
			return fmt.Errorf(retString)
		}
	}
	return nil
}

/* Function to stop the singularity Plugin store */
func PlugStoreStop() error {

	// Unload all the manage plugins
	for _, plugin := range pluginStore.allManagePlugins {
		err := plugin.UnloadPlugin()
		if err != nil {
			log.ERROR.Println("Failed to unload plugin ", plugin, " : ", err)
		}
	}

	pluginStore.pluginReg.Stop()

	return nil
}
