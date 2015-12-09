/* PluginReg is the Plugin Registry That keeps track of the Plugin
 * Which are Discovered, Loaded, and Activated
 */

package pluginmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/spf13/jwalterweatherman"
	"io/ioutil"
	PluginConn "org.openappstack/singularity/pluginmanager/pluginconn"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	// An error to indicate the plugin not discovered
	ConfigLoadFailed = errors.New("Configuration load failed")

	// An error to indicate the plugin not discovered
	PluginNotDiscovered = errors.New("Plugin is not Discovered")

	// An error to indicate the connection to plugin could not be made
	PluginConnFailed = errors.New("Plugin to connection failed")

	// An error to indicate the plugin is already loaded
	PluginLoaded = errors.New("Plugin is already loaded")

	UntarError    = errors.New("Failed to unload the Tar file")
	SaveConfError = errors.New("Failed to save the plugin conf")

	// Conf Extension
	DefaultConfFile       = "plugin.conf"
	DefaultPluginConfFile = "runtime.conf"
	DefaultTarExt         = ".tar"
	PluginBinary          = "pluginmain"
	PluginSockFile        = "pluginconn.sock"
	PluginUrl             = "unix://plugin"
	// Default Interval for Discovery search in MS
	DefaultInterval = 500 * time.Millisecond
	// Default Connection retry Count
	ConnRetryCount = 20
	//plugin registry
	pluginReg *PluginReg = nil
)

type Plugin struct {
	// The URL to reach the Plugin
	PluginUrl string
	// The plugin socket file
	PluginSock string
	// The plugin Connection
	pluginConn *PluginConn.PluginClient
	// The plugin supported function
	methods []string
	// The plugin registered callback
	callbacks map[string]bool
	// Plugin disconnected state (currently Being set but not being used)
	connected bool
	// The Plugin instance PId
	pid int
	// The version info
	Version VersionInfo
	// Type
	Type string
	// App Name
	Controller string
}

/* PluginRegConf provides the configuration to create a plugin registry
 */
type PluginRegConf struct {
	// The location to search for Plugin. Default is .
	PluginLocation string
}

type VersionInfo struct {
	start string
	end   string
}

/* Discover plugin information */
type ControllerInfo struct {
	Name     string
	version  VersionInfo
	plugtype string
}

/**** Struct for defining Json Configuration ****/
type Controller struct {
	Name         string `json:"name"`
	FromVersion  string `json:"from-version,omitempty"`
	ToVersion    string `json:"to-version,omitempty"`
	EqualVersion string `json:"equals-version,omitempty"`
}

type PluginType struct {
	Type        string       `json:"plugin-type"`
	Controllers []Controller `json:"controllers"`
}

type PluginConf struct {
	PluginTypes []PluginType `json:"plugin-types"`
}

/*****/

// Struct to define the runtime configuration of the plugin
type RuntimeConf struct {
	Url  string `json:"url"`
	Sock string `json:"sockpath"`
}

/* PluginReg should be created per types of Plugin
 * each PluginReg monitor a specific location */
type PluginReg struct {
	// The discoveredPlugin list -- map the tar location for a appid
	DiscoveredPlugin map[string]struct{}
	LifeCyclePlugins map[ControllerInfo]string
	// The waitgroup to wait for till PluginRegistry doesn't stop
	Wg *sync.WaitGroup
	// The Plugin search location
	PluginLocation string
	// The mutex to sync the Plugin reg access
	RegAccess *sync.Mutex
	// The flag to stop PluginRegistry Service
	StopFlag bool
}

/* Function is called to inititate the PluginRegistry as per the Plugin registry Configuration.
   It initiate and return a plugin registry pointer that could be used to manage plugins.
   If Discovery is enabled the DiscoverService Starts */
func PluginRegInit(regConf PluginRegConf) (*PluginReg, error) {

	var wg sync.WaitGroup

	pluginLocation := regConf.PluginLocation

	pluginReg = &PluginReg{}

	// Map to hold discovered Plugins
	pluginReg.LifeCyclePlugins = make(map[ControllerInfo]string)
	pluginReg.DiscoveredPlugin = make(map[string]struct{})

	pluginReg.PluginLocation = pluginLocation
	pluginReg.Wg = &wg
	pluginReg.RegAccess = &sync.Mutex{}
	pluginReg.StopFlag = false
	wg.Add(1)
	go discoverPlugin(&wg, pluginReg)
	return pluginReg, nil
}

/* Function to wait for PluginReg Discovery service to be stopped. If its not started then it return immediately */
func (pluginReg *PluginReg) WaitForStop() {
	pluginReg.Wg.Wait()
}

/* Function to stop the Plugin Registry service. It stops the discovery service */
func (pluginReg *PluginReg) Stop() {
	pluginReg.StopFlag = true
}

/* Function for the routine to discover services */
func discoverPlugin(wg *sync.WaitGroup, pluginReg *PluginReg) {
	defer wg.Done()
	/* loop to Check for the Plugin Update */
	for true {
		pluginLocation := pluginReg.PluginLocation
		// Check the plugin location for a new plugin
		files, dirReadError := ioutil.ReadDir(pluginLocation)
		if dirReadError != nil {
			break
		}
		// Check for range of files in the location
		for _, f := range files {
			var fileName string
			if f.IsDir() {
				// Skip if it is a directory */
				continue
			}
			fileName = f.Name()
			ext := filepath.Ext(fileName)
			// Check if it is a tar File
			if ext == DefaultTarExt {
				// Get the plugin name
				tarName := fileName[0 : len(fileName)-len(ext)]
				// Check if the plugin is already discovered
				_, tarDiscovered := pluginReg.DiscoveredPlugin[tarName]
				if !tarDiscovered {
					// Untar the tar file to get the pconf
					tarFile := filepath.Join(pluginLocation, fileName)
					// Untar the file in proper location
					untarErr := untarIt(tarFile, pluginLocation)
					if untarErr != nil {
						log.ERROR.Println("Failed to untar the file: ", tarFile, ", Error: ", untarErr)
						//return nil, UntarError
						continue
					}
					// Read the plugin.conf
					// Get the configuration file
					tarFold := filepath.Join(pluginLocation, tarName)
					confFile := filepath.Join(tarFold, DefaultConfFile)
					// Load new plugin Conf
					pluginConf, confLoadErr := loadPluginConfigs(confFile)
					if confLoadErr != nil {
						log.ERROR.Println("Configuration load failed for file: ", confFile, ", Error: ", confLoadErr)
						//return nil, confLoadError
						continue
					}
					// Check for the available plugin type
					for _, pluginType := range pluginConf.PluginTypes {
						// Check for all the application
						for _, controller := range pluginType.Controllers {
							controllerInfo := &ControllerInfo{}
							controllerInfo.Name = controller.Name
							if controller.EqualVersion != "" {

								controllerInfo.version = VersionInfo{controller.EqualVersion, ""}
							} else {
								controllerInfo.version = VersionInfo{controller.FromVersion, controller.ToVersion}
							}
							fmt.Printf("Plugin type: %s\n", pluginType.Type)
							switch pluginType.Type {
							case "Lifecycle", "LIFECYCLE", "lifecycle":
								pluginReg.LifeCyclePlugins[*controllerInfo] = tarFold
								break
							default:
								log.ERROR.Println("Invalid pligin type. Ignoring: ", pluginType.Type, " for plugin : ", fileName)
							}

						}
					}
					pluginReg.DiscoveredPlugin[tarName] = struct{}{}

				}
			}
		}
		// Check if stop file has been raised
		if pluginReg.StopFlag {
			break
		}
		// Wait for 1 sec
		time.Sleep(time.Duration(DefaultInterval))
	}
}

/* Check if a plugin is discovered by the plugin registry discovery service automatically or is discover implicitly */
func (pluginReg *PluginReg) IsDiscovered(pluginname string) bool {

	return pluginReg.isDiscovered(pluginname)
}

/* Internal: Check if a plugin is already discovered */
func (pluginReg *PluginReg) isDiscovered(appPlugin string) bool {
	_, pluginDiscovered := pluginReg.DiscoveredPlugin[appPlugin]
	if !pluginDiscovered {
		return false
	}
	return true
}

/* Unload a Plugin from the plugin Registry. It invokes a stop request to the plugin.
   (It doesn't remove the Plugin from Discovered Plugin List) */
func (plugin *Plugin) UnloadPlugin() error {

	// Initiate Locking
	//pluginReg.RegAccess.Lock()
	//defer pluginReg.RegAccess.Unlock()

	// Send the Stop request
	stopErr := plugin.stop()
	if stopErr != nil {
		log.ERROR.Println("Failed to isend stop to the plugin: ", stopErr)
	}

	// Close the connection
	plugin.pluginConn.Close()

	// Kill the plugin process
	stoppErr := stopProcess(plugin.pid)
	if stoppErr != nil {
		log.ERROR.Println("Failed to stop the plugin process: ", stoppErr)
	}

	return nil
}

/* Function to reload a plugin */
func (plugin *Plugin) ReloadPlugin() error {

	plugin.UnloadPlugin()

	// Get the plugin reload info
	name := plugin.Controller
	plugType := plugin.Type
	version := plugin.Version.start

	newPlugin, err := pluginReg.LoadPluginInstance(plugType, name, version)
	if err != nil {
		return fmt.Errorf("Failed to reload plugin: %v", err)
	}

	// The plugin Connection
	plugin.pluginConn = newPlugin.pluginConn
	// Plugin disconnected state (currently Being set but not being used)
	plugin.connected = newPlugin.connected
	// The Plugin instance PId
	plugin.pid = newPlugin.pid

	return nil
}

func stopProcess(pid int) error {

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("Failed to get Process of Id: %d", pid)
	}

	// Kill the procecss
	killErr := process.Signal(syscall.SIGUSR1)
	if killErr != nil {
		return fmt.Errorf("Failed to deliver SIGUSR1 to process %d: %v", pid, killErr)
	}

	return nil
}

// Get LifeCycle Plugin Loc
func (pluginReg *PluginReg) getLifeCyclePluginLoc(controller string, version string) (string, *VersionInfo) {

	// Check every monitor plugin
	for controllerInfo, location := range pluginReg.LifeCyclePlugins {
		if controllerInfo.Name == controller && isVersionEqual(controllerInfo.version.start, controllerInfo.version.end, version) {
			return location, &VersionInfo{controllerInfo.version.start, controllerInfo.version.end}

		}
	}
	return "", nil
}

/* Load the plugin to the plugin Registry explicitly when lazy load is active.
(if The discovery Process is not running, It search for the plugin and then load it to the registry)
*/
func (pluginReg *PluginReg) LoadPluginInstance(plugType string, controller string, version string) (*Plugin, error) {

	var pluginLoc string = ""
	var versionInfo *VersionInfo = nil
	// Get the plugin location
	switch plugType {
	case "Lifecycle", "LIFECYCLE", "lifecycle":
		pluginLoc, versionInfo = pluginReg.getLifeCyclePluginLoc(controller, version)
		break
	default:
		return nil, fmt.Errorf("Invalid pligin type. Ignoring: %s", plugType)

	}
	if pluginLoc == "" {
		return nil, fmt.Errorf("Plugin tar not discovered")
	}

	// Get the plugin tar location
	tarFold := pluginLoc

	// Runtime Conf file
	confFile := filepath.Join(tarFold, DefaultPluginConfFile)

	// Create RuntimeConf
	pluginConf := RuntimeConf{}
	StartPath := "./" + PluginBinary
	pluginConf.Url = PluginUrl
	pluginConf.Sock = PluginSockFile

	// Save new plugin Conf
	confSaveError := saveRuntimeConfigs(confFile, pluginConf)
	if confSaveError != nil {
		log.ERROR.Println("Configuration load failed for file: ", confFile, ", Error: ", confSaveError)
		return nil, SaveConfError
	}

	// get the start path
	startPath := filepath.Join(tarFold, StartPath)

	// Start the Plugin
	fmt.Printf("Starting plugin: %s\n", startPath)
	pid, startErr := pluginReg.startPlugin(startPath)
	if startErr != nil {
		log.ERROR.Println("Failed to start the plugin: ", startErr)
	}

	// get the unix socket file path
	sockFile := filepath.Join(tarFold, pluginConf.Sock)

	retryCount := 0
	var pluginConn *PluginConn.PluginClient = nil
	time.Sleep(DefaultInterval * 4)
	for retryCount < ConnRetryCount {
		var connErr error
		// Initiate Connection to a Plugin
		fmt.Printf("Trying to connect: %s\n", sockFile)
		pluginConn, connErr = PluginConn.NewPluginClient(sockFile)
		if connErr == nil {
			break
		}
		retryCount++
		// Sleep for a delay
		time.Sleep(DefaultInterval)
	}
	if pluginConn == nil {
		return nil, PluginConnFailed
	}

	plugin := &Plugin{}
	plugin.PluginSock = sockFile
	plugin.PluginUrl = pluginConf.Url
	plugin.pluginConn = pluginConn
	plugin.connected = true
	plugin.callbacks = make(map[string]bool)
	// set the plugin instance process id
	plugin.pid = pid
	plugin.Version = *versionInfo
	plugin.Type = plugType
	plugin.Controller = controller

	// Activate the plugin
	activateErr := plugin.activate()
	if activateErr != nil {
		return plugin, activateErr
	}

	return plugin, nil
}

func (pluginReg *PluginReg) startPlugin(startFile string) (int, error) {

	// Change the file permission
	err := os.Chmod(startFile, 0777)
	if err != nil {
		fmt.Printf("Failed to change mode: %v", err)
		return 0, err
	}

	dir := filepath.Dir(startFile)
	startPath, _ := filepath.Abs(dir)
	file := path.Base(startFile)

	_, lookErr := exec.LookPath(startFile)
	if lookErr != nil {
		fmt.Printf("Lookerror")
		return 0, lookErr
	}
	env := os.Environ()
	attr := &syscall.ProcAttr{Dir: startPath, Env: env}
	pid, execErr := syscall.ForkExec(file, nil, attr)
	if execErr != nil {
		fmt.Printf("Exeerror")
		return 0, execErr
	}
	fmt.Printf("Started process: %d\n", pid)
	return pid, nil
}

// function to check plugin status
func (plugin *Plugin) checkConnection() bool {
	if plugin.Ping() != nil {
		return false
	}
	return true
}

func (plugin *Plugin) ReConnect() error {

	// Connect to the plugin
	pluginConn, connErr := PluginConn.NewPluginClient(plugin.PluginSock)
	if connErr != nil {
		plugin.connected = false
		return fmt.Errorf("Failed to reconnect: %v", connErr)
	}
	// Set connection object
	plugin.pluginConn = pluginConn
	plugin.connected = true

	return nil
}

// Activate a plugin
func (plugin *Plugin) activate() error {
	pluginUrl := plugin.PluginUrl
	pluginConn := plugin.pluginConn

	requestUrl := pluginUrl + "/Activate"
	request := &PluginConn.PluginRequest{Url: requestUrl, Body: nil}

	resp, reqerr := pluginConn.Request(request)
	if reqerr != nil {
		plugin.connected = false
		return reqerr
	}
	if resp.Status != "200 OK" {
		return fmt.Errorf("request failed. Status: %s", resp.Status)
	}

	// Get the response
	unmarshalError := json.Unmarshal(resp.Body, &plugin.methods)
	if unmarshalError != nil {
		return fmt.Errorf("Json Unmarshal failed: %s", unmarshalError)
	}

	return nil
}

// Deactivate a plugin
func (plugin *Plugin) stop() error {
	pluginUrl := plugin.PluginUrl
	pluginConn := plugin.pluginConn

	requestUrl := pluginUrl + "/Stop"
	request := &PluginConn.PluginRequest{Url: requestUrl, Body: nil}

	resp, err := pluginConn.Request(request)
	if err != nil {
		return err
	}
	if resp.Status != "200 OK" {
		return fmt.Errorf("request failed")
	}

	return nil
}

/* Get the list of available (registered) methods for a specific plugin */
func (plugin *Plugin) GetMethods() []string {

	var methods []string
	methods = plugin.methods
	return methods
}

/* Register a callback that will be called on notification from the plugin */
func (plugin *Plugin) RegisterCallback(function func([]byte)) error {

	if !plugin.connected {
		return fmt.Errorf("Plugin is not connected")
	}

	funcName := getFuncName(function)
	if funcName == "" {
		return fmt.Errorf("Failed to get the method name")
	}
	// Check if the callback is already registered
	_, ok := plugin.callbacks[funcName]
	if ok {
		return fmt.Errorf("The callback is already Registerd")
	}
	// Put the callback function in the callbacks map
	plugin.callbacks[funcName] = false

	// Start the execution thread
	go plugin.executeCallback(funcName, function)

	return nil
}

// Internal:  thread body to execute a callback request
func (plugin *Plugin) executeCallback(funcName string, function func([]byte)) {
	// wrap the method name in bytes
	data, marshalErr := json.Marshal(funcName)
	if marshalErr != nil {
		log.ERROR.Printf("Json Marshal Failed to encode method name")
		return
	}

	pluginUrl := plugin.PluginUrl
	pluginConn := plugin.pluginConn

	requestUrl := pluginUrl + "/" + "RegisterCallback"
	request := &PluginConn.PluginRequest{Url: requestUrl, Body: data}

	//	for plugin.callbacks[funcName] == false {
	for true {
		resp, err := pluginConn.Request(request)
		if err != nil {
			plugin.connected = false
			log.FATAL.Fatalf("Failed to sent CallBack Execution Request: %v", err)
			return
		}
		if resp.Status != "200 OK" {
			log.FATAL.Fatalf("Failed to sent callback request")
			return
		}
		// get the data from resp
		callBackInput := resp.Body
		// call the callback
		function(callBackInput)
	}
}

/* Executes a specific plugin method by the method name. Each method takes a byte array as input
   and returns a byte array as output */
func (plugin *Plugin) Execute(funcName string, body []byte) (error, []byte) {

	found := false

	if !plugin.connected {
		return fmt.Errorf("Plugin is not connected"), nil
	}

	// check if method is registered
	for _, method := range plugin.methods {
		if method == funcName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Method of name : %s is not registered", funcName), nil
	}

	pluginUrl := plugin.PluginUrl
	pluginConn := plugin.pluginConn

	requestUrl := pluginUrl + "/" + funcName
	request := &PluginConn.PluginRequest{Url: requestUrl, Body: body}

	resp, err := pluginConn.Request(request)
	if err != nil {
		plugin.connected = false
		// try to reconnect the plugin
		err := plugin.ReConnect()
		if err != nil {
			err = plugin.ReloadPlugin()
		}
		if err != nil {
			return fmt.Errorf("Failed to communicate with plugin"), nil
		}
	}
	if resp.Status != "200 OK" {
		return fmt.Errorf("request failed"), nil
	}

	ret := resp.Body

	if string(resp.Body) == "<nil>" {
		ret = nil
	}

	return nil, ret
}

/* Ping a specific plugin to check the plugin status */
func (plugin *Plugin) Ping() error {

	pluginUrl := plugin.PluginUrl
	pluginConn := plugin.pluginConn

	testData := "Test Data"
	sendData := []byte(testData)

	requestUrl := pluginUrl + "/" + "Ping"
	request := &PluginConn.PluginRequest{Url: requestUrl, Body: sendData}

	resp, err := pluginConn.Request(request)
	if err != nil {
		plugin.connected = false
		return err
	}
	if resp.Status != "200 OK" {
		return fmt.Errorf("request failed")
	}

	receivedData := string(resp.Body)

	if receivedData != testData {
		return fmt.Errorf("Received data is different than sent one")
	}

	return nil
}
