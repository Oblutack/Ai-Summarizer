package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeAIStream starts a fake AI service that replies to /summarize-text with the given raw
// lines (each followed by a blank line, as in server-sent events). It records the query string.
func fakeAIStream(t *testing.T, lines []string, query *string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if query != nil {
			*query = r.URL.RawQuery
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, l := range lines {
			_, _ = w.Write([]byte(l + "\n\n"))
			w.(http.Flusher).Flush()
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AI_SERVICE_URL", srv.URL)
}

func streamRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/public/summarize-text", PublicSummarizeText)
	return r
}

type sseEvent map[string]any

func parseEvents(t *testing.T, body string) []sseEvent {
	t.Helper()
	var out []sseEvent
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		payload, ok := strings.CutPrefix(block, "data: ")
		if !ok {
			t.Fatalf("not an SSE data line: %q", block)
		}
		var e sseEvent
		if err := json.Unmarshal([]byte(payload), &e); err != nil {
			t.Fatalf("bad event JSON %q: %v", payload, err)
		}
		out = append(out, e)
	}
	return out
}

func TestStreamForwardsEventsAndStripsSourceText(t *testing.T) {
	var query string
	fakeAIStream(t, []string{
		`data: {"type":"status","stage":"preparing"}`,
		`data: {"type":"status","stage":"summarizing","done":1,"total":2}`,
		`data: {"type":"delta","text":"Hello "}`,
		`data: {"type":"delta","text":"world"}`,
		`data: {"type":"done","filename":"a.pdf","text":"THE FULL SOURCE TEXT"}`,
	}, &query)

	w := postText(streamRouter(), "?stream=true&style=bullets", `{"text":"hello"}`)
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("got %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Content-Type") != "text/event-stream; charset=utf-8" {
		t.Fatalf("content type must declare UTF-8 so clients don't guess: %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("X-Accel-Buffering") != "no" || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("missing anti-buffering headers: %v", w.Header())
	}
	if strings.Contains(w.Body.String(), "THE FULL SOURCE TEXT") {
		t.Fatal("source text must never reach the client")
	}
	if !strings.Contains(query, "stream=true") || !strings.Contains(query, "style=bullets") {
		t.Fatalf("AI service should be asked to stream with the options: %q", query)
	}

	events := parseEvents(t, w.Body.String())
	if len(events) != 5 {
		t.Fatalf("got %d events: %v", len(events), events)
	}
	if events[1]["done"] != float64(1) || events[1]["total"] != float64(2) {
		t.Fatalf("progress event was altered: %v", events[1])
	}
	last := events[len(events)-1]
	if last["type"] != "done" || last["filename"] != "a.pdf" || last["text"] != nil {
		t.Fatalf("done event wrong: %v", last)
	}
}

func TestStreamErrorEventIsForwardedAndEndsTheStream(t *testing.T) {
	fakeAIStream(t, []string{
		`data: {"type":"delta","text":"partial"}`,
		`data: {"type":"error","status":502,"message":"The language model failed to produce a summary. Please try again."}`,
		`data: {"type":"delta","text":"must not be forwarded"}`,
	}, nil)

	w := postText(streamRouter(), "?stream=true", `{"text":"hello"}`)
	events := parseEvents(t, w.Body.String())
	if len(events) != 2 || events[1]["type"] != "error" {
		t.Fatalf("got %v", events)
	}
	if strings.Contains(w.Body.String(), "must not be forwarded") {
		t.Fatal("events after an error must not be forwarded")
	}
}

func TestStreamThatEndsWithoutDoneBecomesAnError(t *testing.T) {
	fakeAIStream(t, []string{`data: {"type":"delta","text":"half a summary"}`}, nil)

	w := postText(streamRouter(), "?stream=true", `{"text":"hello"}`)
	events := parseEvents(t, w.Body.String())
	last := events[len(events)-1]
	if last["type"] != "error" || !strings.Contains(last["message"].(string), "ended unexpectedly") {
		t.Fatalf("got %v", events)
	}
}

func TestStreamUpstreamRejectionBeforeStreamingIsAPlainJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":"No text found to summarize."}`))
	}))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)

	w := postText(streamRouter(), "?stream=true", `{"text":"hello"}`)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "No text found") {
		t.Fatalf("got %d %s", w.Code, w.Body.String())
	}
	if strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal("a rejected request must not be an event stream")
	}
}

func TestStreamRequestIsValidatedBeforeCallingTheAIService(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	t.Setenv("AI_SERVICE_URL", srv.URL)

	for _, body := range []string{`{"text":"  "}`, `not json`} {
		if w := postText(streamRouter(), "?stream=true", body); w.Code != http.StatusBadRequest {
			t.Errorf("body %q: got %d, want 400", body, w.Code)
		}
	}
	if w := postText(streamRouter(), "?stream=true&style=poem", `{"text":"hi"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad style: got %d, want 400", w.Code)
	}
	if called {
		t.Fatal("invalid requests must not reach the AI service")
	}
}

func TestStreamHandlesLargeDoneEvents(t *testing.T) {
	big := strings.Repeat("x", 300_000) // more than bufio.Scanner's default 64 KB token limit
	fakeAIStream(t, []string{
		`data: {"type":"delta","text":"ok"}`,
		`data: {"type":"done","filename":"big.pdf","text":"` + big + `"}`,
	}, nil)

	w := postText(streamRouter(), "?stream=true", `{"text":"hello"}`)
	events := parseEvents(t, w.Body.String())
	if events[len(events)-1]["type"] != "done" {
		t.Fatalf("large done event lost: %v", events)
	}
	if strings.Contains(w.Body.String(), big[:100]) {
		t.Fatal("source text leaked")
	}
}
