package alertmanagerroutes

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

var (
	// Port range used to assign ports for the HTTP receiver
	minPort = 30000
	maxPort = 32000

	requestsLogFileFormat = "integration_test_requests_%s.log"
)

func init() {
	flag.IntVar(&minPort, "http-receiver-min-port", minPort, "Minimum port number for HTTP receiver")
	flag.IntVar(&maxPort, "http-receiver-max-port", maxPort, "Maximum port number for HTTP receiver")
}

// httpRequest is a http request record with its full body data
type httpRequest struct {
	*http.Request

	BodyData []byte
}

// httpReceiver is a HTTP server that records incoming requests
// This server stores every incoming requests internally for them to be later
// retrieved using GetHTTPRequests method.
// Any HTTP Proxy connection (CONNECT) made to this server is tunneled to the internal HTTPS server.
type httpReceiver struct {
	records     []httpRequest
	recordsFile *os.File
	mutex       sync.Mutex

	address     string
	httpServer  *httptest.Server
	httpsServer *httptest.Server
}

// NewHTTPReceiver creates a new HTTP receiver
func NewHTTPReceiver(t *testing.T) (*httpReceiver, error) {
	h := &httpReceiver{
		records: []httpRequest{},
	}

	f, err := os.OpenFile(fmt.Sprintf(requestsLogFileFormat, t.Name()), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create requests log file: %v", err)
	}
	h.recordsFile = f

	ln, httpAddress, err := newListener(minPort, maxPort)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP listener: %v", err)
	}
	// We only store the HTTP address, as this is the entry point for Alertmanager to send notifications
	// HTTPS address is only used internally when tunneling via CONNECT method
	h.address = httpAddress

	lns, httpsAddress, err := newListener(minPort, maxPort)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTPS listener: %v", err)
	}

	// Setup HTTP server to receive requests
	// This is the main handler which is handling all the incoming HTTP requests. It is defined here as a closure to have access to both t (for logging) and h (for request storage) variables.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Special handling for CONNECT method (tunneling)
		// We pass the HTTPS address to the handler to be used as the destination for the tunnel.
		// This is in essence simulating a proxy behavior where the destination host is the HTTPS server
		// defined here rather than the original host in the CONNECT request.
		if r.Method == http.MethodConnect {
			handleTunnel(t, httpsAddress, w, r)
			return
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		// Add request to records
		record := httpRequest{
			Request:  r,
			BodyData: body,
		}
		h.mutex.Lock()
		defer h.mutex.Unlock()
		h.records = append(h.records, record)

		// Send 200 OK response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	// Create HTTP server
	h.httpServer = httptest.NewUnstartedServer(handler)
	h.httpServer.Listener = ln

	// Create HTTPS server
	h.httpsServer = httptest.NewUnstartedServer(handler)
	h.httpsServer.Listener = lns

	return h, nil
}

// ports tracks used ports to avoid trying to reuse them
var ports = map[int]struct{}{}

// newListener creates a new TCP listener in the given port range
func newListener(minPort, maxPort int) (net.Listener, string, error) {
	for port := minPort; port <= maxPort; port++ {
		// Stupid optimization to avoid trying already used ports
		if _, used := ports[port]; used {
			continue
		}

		address := fmt.Sprintf(":%d", port)

		l, err := net.Listen("tcp", address)
		if err == nil {
			ports[port] = struct{}{}
			return l, address, nil
		}
	}

	return nil, "", fmt.Errorf("no available ports in range %d-%d", minPort, maxPort)
}

// Start start both HTTP and HTTPS servers
func (h *httpReceiver) Start() {
	h.httpServer.Start()
	h.httpsServer.StartTLS()
}

// Stop stops both HTTP and HTTPS servers
func (h *httpReceiver) Stop() {
	// Stop HTTP servers
	h.httpServer.Close()
	h.httpsServer.Close()
}

// GetAddress returns the HTTP server address
func (h *httpReceiver) GetAddress() string {
	return h.address
}

// GetHTTPRequests returns the recorded HTTP requests received
func (h *httpReceiver) GetHTTPRequests() []httpRequest {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for _, r := range h.records {
		if r.URL.Host == "" {
			r.URL.Host = r.Host
		}
		if r.URL.Scheme == "" {
			if r.TLS == nil {
				r.URL.Scheme = "http"
			} else {
				r.URL.Scheme = "https"
			}
		}

		dump := fmt.Sprintf("%s %s %s\n", r.Method, r.URL.String(), r.Proto)

		for key := range r.Header {
			dump += fmt.Sprintf("%s: %s\n", key, r.Header.Get(key))
		}

		dump += fmt.Sprintf("\n%s\n", r.BodyData)
		fmt.Fprintf(h.recordsFile, "%s", dump)
	}

	return h.records
}

// handleTunnel handles HTTP CONNECT method for tunneling HTTP connection
// It forwards the connection to the given address
func handleTunnel(t *testing.T, address string, w http.ResponseWriter, r *http.Request) {
	// Connect to the destination host
	destConn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer destConn.Close()
	w.WriteHeader(http.StatusOK)

	// Retrieve the underlying current connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		panic("webserver doesn't support hijacking")
	}
	srcConn, _, err := hj.Hijack()
	if err != nil {
		t.Errorf("failed to hijack connection: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer srcConn.Close()

	srcConnStr := fmt.Sprintf("%s->%s", srcConn.LocalAddr().String(), srcConn.RemoteAddr().String())
	dstConnStr := fmt.Sprintf("%s->%s", destConn.LocalAddr().String(), destConn.RemoteAddr().String())

	// Handle connection forwarding data between source and destination
	var wg sync.WaitGroup
	wg.Go(func() { transfer(t, destConn, srcConn, dstConnStr, srcConnStr) })
	wg.Go(func() { transfer(t, srcConn, destConn, srcConnStr, dstConnStr) })
	wg.Wait()
}

// transfer does the low-level data transfer between source and destination connections
func transfer(t *testing.T, destination io.Writer, source io.Reader, destName, srcName string) {
	_, err := io.Copy(destination, source)
	if err != nil {
		t.Logf("Error while proxying request from %s to %s: %v\n", srcName, destName, err)
	}
}
