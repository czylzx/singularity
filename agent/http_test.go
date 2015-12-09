package agent

import (
	"fmt"
	"testing"
)

var httpServ *HTTPServer
var httpsServ *HTTPServer

const (
	address    = "127.0.0.1"
	http_port  = 8181
	https_port = 8182
	certFile   = "../conf/cert.pem"
	keyFile    = "../conf/key.pem"
)

type APIService struct {
}

func (service APIService) Register(s *HTTPServer) {
	fmt.Printf("Registering the http server")
}

func TestHttpServerCreation(t *testing.T) {
	var service APIService
	var serverErr error

	service = APIService{}

	httpServConf := &HttpConfiguration{
		Mode:      "http",
		Address:   address,
		Port:      http_port,
		Registrar: service,
		Cert:      certFile,
		Key:       keyFile,
	}
	httpServ, serverErr := NewHTTPServer(httpServConf)
	if httpServ == nil {
		t.Fatalf("Server is nil. Err: %s", serverErr)
	}
	if serverErr != nil {
		t.Fatalf("Err: %s", serverErr)
	}
	httpServ.Start()
}

func TestHttpsServerCreation(t *testing.T) {
	var service APIService
	var serverErr error

	httpServConf := &HttpConfiguration{
		Mode:      "https",
		Address:   address,
		Port:      https_port,
		Registrar: service,
		Cert:      certFile,
		Key:       keyFile,
	}
	httpsServ, serverErr := NewHTTPServer(httpServConf)
	if httpsServ == nil {
		t.Fatalf("Server is nil. Err: %s", serverErr)
	}
	if serverErr != nil {
		t.Fatalf("Err: %s", serverErr)
	}
	httpsServ.Start()
}

func TestHttpServerStop(t *testing.T) {
	var serverErr error
	serverErr = httpServ.Shutdown()
	if serverErr != nil {
		t.Fatalf("Server stop failed. Err: %s", serverErr)
	}
}

func TestHttpsServerStop(t *testing.T) {
	var serverErr error
	serverErr = httpsServ.Shutdown()
	if serverErr != nil {
		t.Fatalf("Server stop failed. Err: %s", serverErr)
	}
}
