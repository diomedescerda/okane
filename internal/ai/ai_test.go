package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kenma/okane/internal/ai"
)

// countBody records request payloads and lets each handler choose responses.
type fakeServer struct {
	t        *testing.T
	statuses []int    // response status per call (last one repeats)
	contents []string // message content per call (last one repeats)
	bodies   []chatCapture
}

type chatCapture struct {
	model          string
	temperature    float64
	responseFormat string
	system         string
	user           string
	auth           string
}

func (f *fakeServer) handler(w http.ResponseWriter, r *http.Request) {
	capture := chatCapture{auth: r.Header.Get("Authorization")}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	var req struct {
		Model          string  `json:"model"`
		Temperature    float64 `json:"temperature"`
		ResponseFormat struct {
			Type string `json:"type"`
		} `json:"response_format"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		f.t.Fatal(err)
	}
	capture.model = req.Model
	capture.temperature = req.Temperature
	capture.responseFormat = req.ResponseFormat.Type
	if len(req.Messages) > 0 {
		capture.system = req.Messages[0].Content
	}
	if len(req.Messages) > 1 {
		capture.user = req.Messages[1].Content
	}
	f.bodies = append(f.bodies, capture)

	// Call N (1-based) gets statuses[N-1] and contents[N-1]; the last entry
	// repeats for any extra calls.
	idx := len(f.bodies) - 1
	if idx >= len(f.statuses) {
		idx = len(f.statuses) - 1
	}
	status := 200
	if len(f.statuses) > 0 {
		status = f.statuses[idx]
	}
	ci := idx
	if ci >= len(f.contents) {
		ci = len(f.contents) - 1
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if ci < 0 {
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": f.contents[ci]}},
		},
	})
}

func minIndex(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func newFake(t *testing.T, statuses []int, contents []string) (*fakeServer, *httptest.Server) {
	t.Helper()
	fake := &fakeServer{t: t, statuses: statuses, contents: contents}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(srv.Close)
	return fake, srv
}

func clientFor(srv *httptest.Server) *ai.Client {
	return ai.New(ai.Config{BaseURL: srv.URL, APIKey: "test-key", Model: "test-model", Timeout: 5 * time.Second})
}

func TestClientComplete(t *testing.T) {
	fake, srv := newFake(t, []int{200}, []string{`{"answer":"hi"}`})
	client := clientFor(srv)

	content, err := client.Complete(context.Background(), "be terse", "say hi")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	want := `{"answer":"hi"}`
	if content != want {
		t.Fatalf("content = %q, want %q", content, want)
	}

	if len(fake.bodies) != 1 {
		t.Fatalf("calls = %d, want 1", len(fake.bodies))
	}
	call := fake.bodies[0]
	if call.model != "test-model" {
		t.Errorf("model = %q, want test-model", call.model)
	}
	if call.temperature != 0 {
		t.Errorf("temperature = %v, want 0", call.temperature)
	}
	if call.responseFormat != "json_object" {
		t.Errorf("response_format = %q, want json_object", call.responseFormat)
	}
	if call.system != "be terse" || call.user != "say hi" {
		t.Errorf("messages: system = %q, user = %q", call.system, call.user)
	}
	if call.auth != "Bearer test-key" {
		t.Errorf("authorization = %q, want bearer token", call.auth)
	}
}

func TestClientCompleteNoAPIKey(t *testing.T) {
	fake, srv := newFake(t, []int{200}, []string{`{}`})
	client := ai.New(ai.Config{BaseURL: srv.URL})

	if _, err := client.Complete(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if fake.bodies[0].auth != "" {
		t.Errorf("authorization = %q, want empty without API key", fake.bodies[0].auth)
	}
	if fake.bodies[0].model != "glm-5.1" {
		t.Errorf("model = %q, want glm-5.1 default", fake.bodies[0].model)
	}
}

func TestClientRetriesOnceOn500(t *testing.T) {
	fake, srv := newFake(t, []int{500, 200}, []string{`{"kept":true}`})
	client := clientFor(srv)
	client.SetRetryDelay(time.Millisecond)

	content, err := client.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if content != `{"kept":true}` {
		t.Fatalf("content = %q", content)
	}
	if len(fake.bodies) != 2 {
		t.Fatalf("calls = %d, want 2", len(fake.bodies))
	}
}

func TestClientNoRetryOn4xx(t *testing.T) {
	fake, srv := newFake(t, []int{401}, []string{`{"error":{"message":"bad key"}}`})
	client := clientFor(srv)
	client.SetRetryDelay(time.Millisecond)

	_, err := client.Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if len(fake.bodies) != 1 {
		t.Fatalf("calls = %d, want 1 (no retry on 4xx)", len(fake.bodies))
	}
}

func TestClientCategorizeParsesAssignments(t *testing.T) {
	content := "```json\n{\"assignments\":[" +
		"{\"transaction_id\":\"22222222-2222-2222-2222-222222222222\",\"category_id\":\"11111111-1111-1111-1111-111111111111\",\"slug\":\"expense-groceries\"}," +
		"{\"transaction_id\":\"\",\"category_id\":\"33333333-3333-3333-3333-333333333333\"}]}\n```"
	_, srv := newFake(t, []int{200}, []string{content})
	client := clientFor(srv)

	got, err := client.Categorize(context.Background(), "categorize these")
	if err != nil {
		t.Fatalf("Categorize: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("assignments = %+v, want one entry", got)
	}
	if got[0].TransactionID != "22222222-2222-2222-2222-222222222222" ||
		got[0].CategoryID != "11111111-1111-1111-1111-111111111111" ||
		got[0].Slug != "expense-groceries" {
		t.Errorf("assignment = %+v", got[0])
	}
}

func TestClientCategorizeInvalidJSON(t *testing.T) {
	_, srv := newFake(t, []int{200}, []string{"not json at all"})
	client := clientFor(srv)

	if _, err := client.Categorize(context.Background(), "p"); err == nil {
		t.Fatal("expected parse error")
	}
}
