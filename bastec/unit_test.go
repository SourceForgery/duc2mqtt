package bastec

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestConnect_Validation(t *testing.T) {
	testCases := []struct {
		name      string
		rawURL    string
		errorPart string
	}{
		{name: "non-empty path", rawURL: "http://user:pass@example.com/abc", errorPart: "invalid url path"},
		{name: "missing user", rawURL: "http://example.com", errorPart: "missing user"},
		{name: "missing password", rawURL: "http://user@example.com", errorPart: "missing password"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}
			_, err = Connect(*parsed)
			if err == nil || !strings.Contains(err.Error(), tc.errorPart) {
				t.Fatalf("expected error containing %q, got: %v", tc.errorPart, err)
			}
		})
	}
}

func TestConnect_Success(t *testing.T) {
	var sawUserQuery string
	var sawHash string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/if/login.js" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		sawUserQuery = r.URL.Query().Get("username")
		hash := r.URL.Query().Get("hash")
		if hash == "" {
			_, _ = w.Write([]byte(`{"salt_a":"AQI=","salt_b":"AwQ="}`))
			return
		}

		sawHash = hash
		http.SetCookie(w, &http.Cookie{Name: "SESSION_ID", Value: "session-42"})
		_, _ = w.Write([]byte(`{"name":"n","userid":"u","company":"c","city":"z"}`))
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	parsed.User = url.UserPassword("alice", "secret")

	client, err := Connect(*parsed)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if sawUserQuery != "ALICE" {
		t.Fatalf("expected upper-cased username query, got %q", sawUserQuery)
	}
	if sawHash == "" {
		t.Fatal("expected non-empty hash query in login request")
	}
	if client.sessionId != "session-42" {
		t.Fatalf("unexpected session id: %q", client.sessionId)
	}
	if client.RequestURL.Path != "if/json_rpc.js" {
		t.Fatalf("unexpected request URL path: %q", client.RequestURL.Path)
	}
}

func TestJsonRpc_SetsHeadersAndIncrementsId(t *testing.T) {
	receivedIDs := []int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: %q", got)
		}
		if got := r.Header.Get("Cookie"); got != "SESSION_ID=session-42" {
			t.Fatalf("unexpected cookie header: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var req JsonRpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		receivedIDs = append(receivedIDs, req.Id)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	requestURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse request URL: %v", err)
	}

	client := &BastecClient{sessionId: "session-42", RequestURL: *requestURL}
	if _, err := client.jsonRpc(JsonRpcRequest{JsonRpcVersion: "2.0", Method: "x"}); err != nil {
		t.Fatalf("first jsonRpc call failed: %v", err)
	}
	if _, err := client.jsonRpc(JsonRpcRequest{JsonRpcVersion: "2.0", Method: "x"}); err != nil {
		t.Fatalf("second jsonRpc call failed: %v", err)
	}

	if len(receivedIDs) != 2 || receivedIDs[0] != 1 || receivedIDs[1] != 2 {
		t.Fatalf("unexpected id sequence: %v", receivedIDs)
	}
}

func TestJsonRpc_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	requestURL, _ := url.Parse(server.URL)
	client := &BastecClient{sessionId: "session-42", RequestURL: *requestURL}

	_, err := client.jsonRpc(JsonRpcRequest{JsonRpcVersion: "2.0", Method: "x"})
	if err == nil || !strings.Contains(err.Error(), "http error code 403") {
		t.Fatalf("expected http status error, got: %v", err)
	}
}

func TestGetVersion_UsesPdbVersionMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req JsonRpcRequest
		_ = json.Unmarshal(body, &req)
		if req.Method != "pdb.version" {
			t.Fatalf("unexpected method: %s", req.Method)
		}
		_, _ = w.Write([]byte(`{"json-rpc":"2.0","result":"ok","id":1}`))
	}))
	defer server.Close()

	requestURL, _ := url.Parse(server.URL)
	client := &BastecClient{sessionId: "session-42", RequestURL: *requestURL}

	body, err := client.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if !strings.Contains(string(body), `"result":"ok"`) {
		t.Fatalf("unexpected response body: %s", string(body))
	}
}

func TestBrowse_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"json-rpc":"2.0","result":{"devid":"dev1","points":[{"pid":"p1","desc":"D1","acc":"rw","type":"number","attr":"W"}]},"id":1}`))
	}))
	defer server.Close()

	requestURL, _ := url.Parse(server.URL)
	client := &BastecClient{sessionId: "session-42", RequestURL: *requestURL}

	resp, err := client.Browse()
	if err != nil {
		t.Fatalf("Browse failed: %v", err)
	}
	if resp.Result.DevId != "dev1" {
		t.Fatalf("unexpected devid: %s", resp.Result.DevId)
	}
	if len(resp.Result.Points) != 1 || resp.Result.Points[0].Pid != "p1" {
		t.Fatalf("unexpected points: %+v", resp.Result.Points)
	}
}

func TestGetValues_SuccessAndError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var req JsonRpcRequest
			_ = json.Unmarshal(body, &req)
			if req.Method != "pdb.getvalue" {
				t.Fatalf("unexpected method: %s", req.Method)
			}
			if len(req.Params) != 1 || len(req.Params[0]) != 2 {
				t.Fatalf("unexpected params: %+v", req.Params)
			}
			_, _ = w.Write([]byte(`{"json-rpc":"2.0","result":{"timet":1,"times":"x","points":[{"pid":"p1","value":12.3,"decimals":1,"decimals_shown":1}]},"id":1}`))
		}))
		defer server.Close()

		requestURL, _ := url.Parse(server.URL)
		client := &BastecClient{sessionId: "session-42", RequestURL: *requestURL}
		resp, err := client.GetValues([]string{"p1", "p2"})
		if err != nil {
			t.Fatalf("GetValues failed: %v", err)
		}
		if len(resp.Result.Points) != 1 || resp.Result.Points[0].Pid != "p1" {
			t.Fatalf("unexpected points: %+v", resp.Result.Points)
		}
	})

	t.Run("error field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"json-rpc":"2.0","error":"boom","id":1}`))
		}))
		defer server.Close()

		requestURL, _ := url.Parse(server.URL)
		client := &BastecClient{sessionId: "session-42", RequestURL: *requestURL}
		_, err := client.GetValues([]string{"p1"})
		if err == nil || !strings.Contains(err.Error(), "getValues error: boom") {
			t.Fatalf("expected getValues error, got: %v", err)
		}
	})
}
