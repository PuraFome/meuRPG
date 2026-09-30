package system

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	systemv1 "github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1"
	"github.com/PuraFome/meuRPG/backend/gen/meurpg/system/v1/systemv1connect"
)

// TestGetServerInfo calls the service the way real clients will: through the
// generated Connect client, over HTTP, with each protocol Connect supports.
func TestGetServerInfo(t *testing.T) {
	t.Parallel()

	var h2cOnly http.Protocols
	h2cOnly.SetUnencryptedHTTP2(true)

	tests := []struct {
		name       string
		opts       []connect.ClientOption
		http2      bool   // gRPC needs HTTP/2; here, unencrypted (h2c)
		wantMethod string // HTTP method the server should see
	}{
		{name: "connect with binary protobuf", wantMethod: http.MethodPost},
		{name: "connect with JSON", opts: []connect.ClientOption{connect.WithProtoJSON()}, wantMethod: http.MethodPost},
		{
			// Allowed because the RPC is marked NO_SIDE_EFFECTS in the .proto.
			name:       "connect with HTTP GET",
			opts:       []connect.ClientOption{connect.WithHTTPGet()},
			wantMethod: http.MethodGet,
		},
		{name: "gRPC over h2c", opts: []connect.ClientOption{connect.WithGRPC()}, http2: true, wantMethod: http.MethodPost},
		{name: "gRPC-Web", opts: []connect.ClientOption{connect.WithGRPCWeb()}, wantMethod: http.MethodPost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// A channel (not a plain variable) hands the method from the
			// server goroutine to the test goroutine without a data race.
			methods := make(chan string, 1)
			mux := http.NewServeMux()
			path, handler := systemv1connect.NewSystemServiceHandler(NewService("v1.2.3", "abc123"))
			mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods <- r.Method
				handler.ServeHTTP(w, r)
			}))

			server := httptest.NewUnstartedServer(mux)
			server.Config.Protocols = new(http.Protocols)
			server.Config.Protocols.SetHTTP1(true)
			server.Config.Protocols.SetUnencryptedHTTP2(true)
			server.Start()
			t.Cleanup(server.Close)

			httpClient := server.Client()
			if tt.http2 {
				httpClient = &http.Client{Transport: &http.Transport{Protocols: &h2cOnly}}
			}
			client := systemv1connect.NewSystemServiceClient(httpClient, server.URL, tt.opts...)

			resp, err := client.GetServerInfo(t.Context(), connect.NewRequest(&systemv1.GetServerInfoRequest{}))
			if err != nil {
				t.Fatalf("GetServerInfo() error = %v", err)
			}

			if got := resp.Msg.GetVersion(); got != "v1.2.3" {
				t.Errorf("version = %q, want %q", got, "v1.2.3")
			}
			if got := resp.Msg.GetCommit(); got != "abc123" {
				t.Errorf("commit = %q, want %q", got, "abc123")
			}
			if gotMethod := <-methods; gotMethod != tt.wantMethod {
				t.Errorf("server saw HTTP %s, want %s", gotMethod, tt.wantMethod)
			}
		})
	}
}
