package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sampleDataDir copies the bundled sample library into a temp dir so tests
// can write vocabulary without touching the repository.
func sampleDataDir(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "..", "bgtutor", "data", "episodes", "001-cooking-basics")
	dst := filepath.Join(t.TempDir(), "episodes", "001-cooking-basics")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"meta.json", "paragraphs.json"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Dir(filepath.Dir(dst))
}

// connect starts the real HTTP handler and returns an MCP client session.
func connect(t *testing.T, token, clientToken string) (*mcp.ClientSession, error) {
	t.Helper()
	ts := httptest.NewServer(Handler(New(sampleDataDir(t)), HTTPOptions{Token: token}))
	t.Cleanup(ts.Close)
	httpClient := &http.Client{Transport: headerTransport{token: clientToken}}
	client := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	cs, err := client.Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: httpClient, MaxRetries: -1}, nil)
	if err == nil {
		t.Cleanup(func() { _ = cs.Close() })
	}
	return cs, err
}

type headerTransport struct{ token string }

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if h.token != "" {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+h.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		return map[string]any{"error": res.Content[0].(*mcp.TextContent).Text}, true
	}
	var out map[string]any
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out, false
}

func TestSessionFlowOverHTTP(t *testing.T) {
	cs, err := connect(t, "secret", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cs.InitializeResult().Instructions, "get_paragraph") {
		t.Error("server instructions missing")
	}
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 4 {
		t.Fatalf("tools: %v %v", tools, err)
	}

	eps, _ := call(t, cs, "list_episodes", nil)
	if eps["count"].(float64) != 1 {
		t.Fatalf("list_episodes: %v", eps)
	}
	p, _ := call(t, cs, "get_paragraph", map[string]any{"episode_id": "001-cooking-basics", "index": 1})
	if p["position"] != "1 of 9" || p["is_last"] != false || p["speaker"] != "Host" {
		t.Errorf("paragraph 1: %v", p)
	}
	last, _ := call(t, cs, "get_paragraph", map[string]any{"episode_id": "001-cooking-basics", "index": 9})
	if last["is_last"] != true || last["next_index"] != nil {
		t.Errorf("paragraph 9: %v", last)
	}
	bad, isErr := call(t, cs, "get_paragraph", map[string]any{"episode_id": "001-cooking-basics", "index": 10})
	if !isErr || !strings.Contains(bad["error"].(string), "1 to 9") {
		t.Errorf("out of range: %v", bad)
	}

	saved, _ := call(t, cs, "save_vocabulary", map[string]any{
		"term": "тава", "translation": "baking tray", "episode_id": "001-cooking-basics", "paragraph_index": 6})
	if saved["status"] != "saved" {
		t.Errorf("save: %v", saved)
	}
	list, _ := call(t, cs, "list_vocabulary", map[string]any{"query": "tray"})
	if list["total_matching"].(float64) != 1 {
		t.Errorf("list_vocabulary: %v", list)
	}
}

func TestTokenRequired(t *testing.T) {
	if _, err := connect(t, "secret", ""); err == nil {
		t.Error("connect without token should fail")
	}
	if _, err := connect(t, "secret", "wrong"); err == nil {
		t.Error("connect with wrong token should fail")
	}
}

func TestHealthzIsOpen(t *testing.T) {
	ts := httptest.NewServer(Handler(New(t.TempDir()), HTTPOptions{Token: "secret"}))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %v %v", resp, err)
	}
}

// TestWorksBehindTunnel simulates a tunnel: the request reaches the localhost
// listener with a public Host header.
func TestWorksBehindTunnel(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  int
	}{{"secret", http.StatusOK}, {"", http.StatusForbidden}} {
		ts := httptest.NewServer(Handler(New(sampleDataDir(t)), HTTPOptions{Token: tc.token}))
		body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_episodes","arguments":{}}}`
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
		req.Host = "tutor.example.com"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer secret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		ts.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("token=%q: status %d, want %d", tc.token, resp.StatusCode, tc.want)
		}
	}
}
