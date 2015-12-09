package agent

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"
)

// interface for http handler registrations
type HttpHandlerRegistrar interface {
	Register(s *HTTPServer)
}

// HTTPServer is used to wrap an Agent and expose various API's
// in a RESTful manner
type HTTPServer struct {
	mux      *http.ServeMux
	listener net.Listener
	addr     string
}

// configuration for the http server
type HttpConfiguration struct {
	Mode      string
	Address   string
	Port      int
	Registrar HttpHandlerRegistrar
	Cert      string
	Key       string
}

// unixSocketAddr tests if a given address describes a domain socket,
// and returns the relevant path part of the string if it is.
func unixSocketAddr(addr string) (string, bool) {
	if !strings.HasPrefix(addr, "unix://") {
		return "", false
	}
	return strings.TrimPrefix(addr, "unix://"), true
}

// Get the listener address
func getListenerAddr(addr string, port int) (net.Addr, error) {

	if path, ok := unixSocketAddr(addr); ok {
		return &net.UnixAddr{Name: path, Net: "unix"}, nil
	}

	ip := net.ParseIP(addr)
	if ip == nil {
		return nil, fmt.Errorf("Failed to parse IP: %v", addr)
	}

	return &net.TCPAddr{IP: ip, Port: port}, nil
}

// FilePermissions is an interface which allows a struct to set
// ownership and permissions easily on a file it describes.
type FilePermissions interface {
	// User returns a user ID or user name
	User() string

	// Group returns a group ID. Group names are not supported.
	Group() string

	// Mode returns a string of file mode bits e.g. "0644"
	Mode() string
}

// setFilePermissions handles configuring ownership and permissions settings
// on a given file. It takes a path and any struct implementing the
// FilePermissions interface. All permission/ownership settings are optional.
// If no user or group is specified, the current user/group will be used. Mode
// is optional, and has no default (the operation is not performed if absent).
// User may be specified by name or ID, but group may only be specified by ID.
func setFilePermissions(path string, p FilePermissions) error {
	var err error
	uid, gid := os.Getuid(), os.Getgid()

	if p.User() != "" {
		if uid, err = strconv.Atoi(p.User()); err == nil {
			goto GROUP
		}

		// Try looking up the user by name
		if u, err := user.Lookup(p.User()); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
			goto GROUP
		}

		return fmt.Errorf("invalid user specified: %v", p.User())
	}

GROUP:
	if p.Group() != "" {
		if gid, err = strconv.Atoi(p.Group()); err != nil {
			return fmt.Errorf("invalid group specified: %v", p.Group())
		}
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("failed setting ownership to %d:%d on %q: %s",
			uid, gid, path, err)
	}

	if p.Mode() != "" {
		mode, err := strconv.ParseUint(p.Mode(), 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode specified: %v", p.Mode())
		}
		if err := os.Chmod(path, os.FileMode(mode)); err != nil {
			return fmt.Errorf("failed setting permissions to %d on %q: %s",
				mode, path, err)
		}
	}

	return nil
}

// Create a listener as per the http configuration
func getListener(httpAddr net.Addr, config *HttpConfiguration) (net.Listener, error) {

	if config.Mode == "http" {
		socketPath, isSocket := unixSocketAddr(config.Address)
		if isSocket {
			if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
				fmt.Printf("[WARN] agent: Replacing socket %q", socketPath)
			}
			if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error removing socket file: %s", err)
			}
		}

		ln, err := net.Listen(httpAddr.Network(), httpAddr.String())
		if err != nil {
			return nil, fmt.Errorf("Failed to get Listen on %s: %v", httpAddr.String(), err)
		}
		var listener net.Listener
		if isSocket {
			listener = ln
		} else {
			listener = tcpKeepAliveListener{ln.(*net.TCPListener)}
		}
		return listener, nil
	} else {
		cert, err := tls.LoadX509KeyPair(config.Cert, config.Key)
		if err != nil {
			return nil, fmt.Errorf("Failed to load Certificate : %v", err)
		}
		tlsConfig := tls.Config{Certificates: []tls.Certificate{cert}}
		tlsConfig.Rand = rand.Reader
		service := httpAddr.String()
		listener, err := tls.Listen("tcp", service, &tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("Failed to get Listen on %s: %v", httpAddr.String(), err)
		}
		return listener, nil
	}
}

// Create a new HTTP Server
func NewHTTPServer(config *HttpConfiguration) (*HTTPServer, error) {

	var server *HTTPServer

	if config.Port > 0 {

		httpAddr, err := getListenerAddr(config.Address, config.Port)
		if err != nil {
			return nil, fmt.Errorf("Failed to get listener address:port: %v", err)
		}

		// Get listener for the Http server
		listener, err := getListener(httpAddr, config)
		if err != nil {
			return nil, fmt.Errorf("Failed to set Listner: %s", err)
		}

		// Create the mux
		mux := http.NewServeMux()

		// Create the server
		server = &HTTPServer{
			mux:      mux,
			listener: listener,
			addr:     httpAddr.String(),
		}

		// register the http handlers
		config.Registrar.Register(server)
	}

	return server, nil
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by NewHttpServer so
// dead TCP connections eventually go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(30 * time.Second)
	return tc, nil
}

// Start the http Server
func (s *HTTPServer) Start() {

	go http.Serve(s.listener, s.mux)

}

// Shutdown is used to shutdown the HTTP server
func (s *HTTPServer) Shutdown() error {
	if s != nil {
		fmt.Printf("[DEBUG] http: Shutting down http server (%v)\n", s.addr)
		err := s.listener.Close()
		if err != nil {
			fmt.Printf("[ERROR] Failed to close http listener: %v\n", err)
			return err
		}
	}
	return nil
}

// Write a json string with given header code
func WriteJsonResponse(v interface{}, code int, w http.ResponseWriter) error {
	js, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
	return nil
}
