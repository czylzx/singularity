package agent

import (
	"encoding/json"
	"fmt"
	log "github.com/spf13/jwalterweatherman"
	"net/http"
	"org.openappstack/singularity/pluginmanager"
	"path/filepath"
	"strconv"
)

type APIService struct {
	Config *Configuration
}

type ControllerStartReq struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	CIL     string `json:"cil"` // The Controller Instance Location (a path in case of
	// non-containerized and a container-id in case of containerzed)
	Deploy string `json:"deploy"`
}

type ControllerStartResp struct {
	CId string `json:"cid"` // The unique Controller Identifier
}

type ControllerStopReq struct {
	CId string `json:"cid"` // The unique Controller Identifier
}

type Controller struct {
	Name      string
	Version   string
	CIL       string
	Deploy    string
	Pid_cid   string // Process id or container id
	CId       string
	InitParam []byte // in a grey area -- currently not being used
}

type Response struct {
	// Response is used for sending Json Response to the Client i.e. { "Success": "true", Message}
	Success string `json: "suuccess"`
	Message string `json: "message"`
}

// APIServer for the http server
var apiServer *HTTPServer

// Use this map for controller
var runningControllerInstances map[string]Controller
var cidControllerMap map[string]Controller

// Controller unique id
var uniqueId int

// Start the CommandApi Service
func (service *APIService) Start() error {
	configuration := service.Config

	var serverErr error

	//TODO:  Load the old running controller info from kvstore
	/*** XXX: Temp ***/ runningControllerInstances = make(map[string]Controller)
	//TODO:  Load the old controller conf info from kvstore
	/*** XXX: Temp ***/ cidControllerMap = make(map[string]Controller)
	//TODO:  Load the old uniqueId from kvstore
	/*** XXX: Temp ***/ uniqueId = 0

	certFile := filepath.Join(startPath, configuration.Cert)
	keyFile := filepath.Join(startPath, configuration.Key)

	// start command server
	commandConfig := &HttpConfiguration{
		Mode:      configuration.Mode,
		Address:   configuration.Host,
		Port:      configuration.Port,
		Registrar: service,
		Cert:      certFile,
		Key:       keyFile,
	}
	apiServer, serverErr = NewHTTPServer(commandConfig)
	if serverErr != nil {
		log.FATAL.Fatalf("Aborting, Error while Creating the Command Api Server: %s", serverErr)
		return serverErr
	}
	apiServer.Start()

	log.INFO.Printf("APIServer started at : %s", apiServer.addr)

	return nil
}

// Stop the Command Api service
func (service *APIService) Stop() error {
	//Store data to KVStore
	apiServer.Shutdown()
	log.INFO.Printf("APIServer stopped")
	return nil
}

// All api functionality is registered here...
func (api *APIService) Register(s *HTTPServer) {

	// Lifecycle service api
	s.mux.HandleFunc("/v1/api/lifecycle/start", start)
	s.mux.HandleFunc("/v1/api/lifecycle/stop", stop)
}

// Starts controller deployed at a given location
func start(w http.ResponseWriter, r *http.Request) {
	log.DEBUG.Printf("Executing API - start")

	decoder := json.NewDecoder(r.Body)

	req := &ControllerStartReq{}

	decodeErr := decoder.Decode(req)
	if decodeErr != nil {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Invalid request: Failed to Decode: %v", decodeErr)}, 400, w)
		log.DEBUG.Printf("Failed to decode request: %v", decodeErr)
		return
	}

	// Check if the controller is already started -- using the CIL
	controller, ok := runningControllerInstances[req.CIL]
	if ok {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Controller is already started at: %s", req.CIL)}, 400, w)
		log.DEBUG.Printf("Controller is already started at: %s", req.CIL)
		return
	}

	// Create a controller instance and mappit to a unique controller id
	controller = Controller{Name: req.Name, Version: req.Version, CIL: req.CIL, Deploy: req.Deploy, Pid_cid: "", InitParam: nil}

	// Get the plugin for the controller
	lifecyclePlugin, pluginerr := pluginmanager.GetManagePlugin(controller.Name, controller.Version)
	if pluginerr != nil {
		WriteJsonResponse(Response{"false", fmt.Sprintf("failed to load proper plugin for controller: %s of Version: %s : Error: %v", controller.Name, controller.Version, pluginerr)}, 400, w)
		log.DEBUG.Printf("Fialed to load plugin for controller: %s of Version: %s: Error: %v", controller.Name, controller.Version, pluginerr)
		return
	}

	// Send request to the plugin
	controller.CId = GetUniqueControllerID()
	initError := lifecyclePlugin.Init(controller.CId, []byte(controller.CIL))
	if initError != nil {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Failed to initialize lifecycle plugin for controller: %s : Error: %v", controller.Name, initError)}, 400, w)
		log.DEBUG.Printf("Failed to init controller : %s : Error: %v", controller.Name, initError)
		return
	}
	startError := lifecyclePlugin.Start(controller.CId, nil)
	if startError != nil {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Failed to start lifecycle plugin for controller: %s : Error: %v", controller.Name, startError)}, 400, w)
		log.DEBUG.Printf("Failed to start controller : %s : Error: %v", controller.Name, startError)
		return
	}

	// Map the controller instance to the CId map -- using the CId
	cidControllerMap[controller.CId] = controller
	/*** TODO : Need to be stored in KV STore ***/

	// map the controller instance to the start map -- using the CIL
	runningControllerInstances[controller.CIL] = controller
	/*** TODO : Need to be stored in KV STore ***/

	WriteJsonResponse(Response{"true", controller.CId}, 200, w)
}

// Stop Controller deployed at a given location
func stop(w http.ResponseWriter, r *http.Request) {
	log.DEBUG.Printf("Executing API - stop")

	decoder := json.NewDecoder(r.Body)

	req := &ControllerStopReq{}

	decodeErr := decoder.Decode(req)
	if decodeErr != nil {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Invalid request: Failed to Decode: %v", decodeErr)}, 400, w)
		log.DEBUG.Printf("Failed to decode request: %v", decodeErr)
		return
	}

	// Get the Controller details
	controller, ok := cidControllerMap[req.CId]
	if !ok {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Invalid controller id: %s", req.CId)}, 400, w)
		log.DEBUG.Printf("Invalid controller id: %s", req.CId)
		return
	}

	// Check if the controller is already started -- using the CIL
	controller, ok = runningControllerInstances[controller.CIL]
	if !ok {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Controller has mot started at: %s", controller.CIL)}, 400, w)
		log.DEBUG.Printf("Controller has not started at: %s", controller.CIL)
		return
	}

	// Get the plugin for the controller
	lifecyclePlugin, pluginerr := pluginmanager.GetManagePlugin(controller.Name, controller.Version)
	if pluginerr != nil {
		WriteJsonResponse(Response{"false", fmt.Sprintf("failed to load proper plugin for controller: %s of Version: %s : Error: %v", controller.Name, controller.Version, pluginerr)}, 400, w)
		log.DEBUG.Printf("Fialed to load plugin for controller: %s of Version: %s: Error: %v", controller.Name, controller.Version, pluginerr)
		return
	}

	// send request to the plugin
	stopError := lifecyclePlugin.Stop(controller.CId, nil)
	if stopError != nil {
		WriteJsonResponse(Response{"false", fmt.Sprintf("Failed to stop lifecycle plugin for controller: %s : Error: %v", controller.Name, stopError)}, 400, w)
		log.DEBUG.Printf("Failed to start controller : %s : Error: %v", controller.Name, stopError)
		return
	}

	// Delete the controller from the running controller map
	delete(runningControllerInstances, controller.CIL)

	WriteJsonResponse(Response{"true", ""}, 200, w)
}

// Get the unique controller id
func GetUniqueControllerID() string {
	return strconv.Itoa(uniqueId)
}
