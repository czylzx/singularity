package agent

import (
	"encoding/json"
	"fmt"
	log "github.com/spf13/jwalterweatherman"
	"org.openappstack/singularity/pluginmanager"
	store "org.openappstack/singularity/store"
	"os"
	"path/filepath"
)

// configuration related error
type ConfigError string

// configuration as loaded from conf file
type Configuration struct {
	Host         string
	Port         int
	LogFile      string
	LogThreshold string
	KVStoreName  string
	Mode         string
	Cert         string
	Key          string
}

var (
	mainStore  *store.KVStore = nil
	startPath  string
	confPath   string
	apiService *APIService
)

func (err ConfigError) Error() string {
	return fmt.Sprintf("Encountered error: %s", string(err))
}

// Start the agent
func Start() {

	// load configurations
	var configuration Configuration
	var configerr error

	startPath, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	confPath = filepath.Join(startPath, "conf")

	configuration, configerr = loadConfigs()
	if configerr != nil {
		fmt.Printf(configerr.Error())
		os.Exit(1)
	}
	fmt.Printf("Configuration loaded...")

	// initialize logging
	initLogging(&configuration)
	log.INFO.Printf("Logging subsystem initialized...")

	// Initialize the KV store
	storeErr := initKVStore(&configuration)
	if storeErr != nil {
		log.FATAL.Fatalf("Aborting, KV store initialization failed due to err: %s", storeErr)
		return
	}
	log.INFO.Printf("KVStore initialized...")

	serviceErr := startApiService(&configuration)
	if serviceErr != nil {
		return
	}
	log.INFO.Printf("APIService Started")

	// Start the Plugin Registry service
	err := pluginmanager.PluginStoreInit(mainStore)
	if err != nil {
		log.INFO.Printf("pluginStoreInit Failed")
		os.Exit(1)
	}

	log.INFO.Printf("Agent started successfully\n")

}

// Start API service
func startApiService(configuration *Configuration) error {
	var err error

	// start command server
	apiService = &APIService{
		Config: configuration,
	}
	err = apiService.Start()
	if err != nil {
		return err
	}
	return nil
}

// load the config data from the file
func loadConfigs() (Configuration, error) {

	// open the config file
	fname := filepath.Join(confPath, "singularity.conf")
	file, _ := os.Open(fname)

	// load the config from file
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	loaderr := decoder.Decode(&configuration)
	if loaderr != nil {
		return configuration, ConfigError("Could not read configuration file")
	}

	return configuration, nil
}

// Init KV store
func initKVStore(configuration *Configuration) error {
	var err error

	fname := filepath.Join(startPath, configuration.KVStoreName)
	log.INFO.Printf("KVStore file: %s\n", fname)
	mainStore, err = store.NewKVStore(fname)
	if err != nil {
		return err
	}

	return nil
}

// initialize logging...
func initLogging(configuration *Configuration) {

	log.SetLogFile(configuration.LogFile)

	threshold := configuration.LogThreshold
	if threshold == "TRACE" {
		log.SetLogThreshold(log.LevelTrace)
		log.SetStdoutThreshold(log.LevelTrace)
	} else if threshold == "DEBUG" {
		log.SetLogThreshold(log.LevelDebug)
		log.SetStdoutThreshold(log.LevelDebug)
	} else if threshold == "INFO" {
		log.SetLogThreshold(log.LevelInfo)
		log.SetStdoutThreshold(log.LevelInfo)
	} else if threshold == "WARN" {
		log.SetLogThreshold(log.LevelWarn)
		log.SetStdoutThreshold(log.LevelWarn)
	} else if threshold == "ERROR" {
		log.SetLogThreshold(log.LevelError)
		log.SetStdoutThreshold(log.LevelError)
	} else if threshold == "CRITICAL" {
		log.SetLogThreshold(log.LevelCritical)
		log.SetStdoutThreshold(log.LevelCritical)
	} else if threshold == "FATAL" {
		log.SetLogThreshold(log.LevelFatal)
		log.SetStdoutThreshold(log.LevelFatal)
	}
}

// stop the agent
func Stop() {
	apiService.Stop()
	pluginmanager.PlugStoreStop()
	log.INFO.Printf("Agent stopped\n")
}
