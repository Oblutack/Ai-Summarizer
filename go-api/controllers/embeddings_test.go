package controllers

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func floats(values ...float32) []byte {
	raw := make([]byte, 0, len(values)*4)
	for _, v := range values {
		raw = binary.LittleEndian.AppendUint32(raw, math.Float32bits(v))
	}
	return raw
}

func TestDotIsTheCosineOfUnitVectors(t *testing.T) {
	a, b, c := floats(1, 0), floats(1, 0), floats(0, 1)
	if got := dot(a, b); math.Abs(got-1) > 1e-9 {
		t.Errorf("equal vectors: %v", got)
	}
	if got := dot(a, c); math.Abs(got) > 1e-9 {
		t.Errorf("perpendicular vectors: %v", got)
	}
	diagonal := floats(float32(math.Sqrt2/2), float32(math.Sqrt2/2))
	if got := dot(a, diagonal); math.Abs(got-math.Sqrt2/2) > 1e-6 {
		t.Errorf("45 degrees: %v", got)
	}
}

func TestVectorsOfDifferentModelsNeverMatch(t *testing.T) {
	if got := dot(floats(1, 0), floats(1, 0, 0)); !math.IsInf(got, -1) {
		t.Errorf("different lengths must not compare, got %v", got)
	}
	if got := dot([]byte{1, 2, 3}, []byte{1, 2, 3}); !math.IsInf(got, -1) {
		t.Errorf("a broken vector must not compare, got %v", got)
	}
}

func TestRankFusionLetsMeaningLeadAndKeywordSettleTies(t *testing.T) {
	// 1 and 2 are equally good by meaning order, 3 is only a keyword hit.
	got := fuseRankings([]uint{1, 2}, []uint{3, 2, 1})
	if !slices.Equal(got, []uint{1, 2, 3}) {
		t.Errorf("meaning leads: %v", got)
	}
	// A passage high in both rankings beats one that is only first by meaning.
	got = fuseRankings([]uint{5, 6, 7}, []uint{7, 6})
	if got[0] != 6 {
		t.Errorf("a passage high in both rankings should win: %v", got)
	}
	// But a keyword hit that meaning does not rate must not overtake the best by meaning.
	got = fuseRankings([]uint{5, 6}, []uint{9, 8, 7})
	if got[0] != 5 {
		t.Errorf("keyword alone must not overtake the best by meaning: %v", got)
	}
	// A keyword hit that meaning does not know is kept, after everything meaning found.
	got = fuseRankings([]uint{1}, []uint{9})
	if !slices.Equal(got, []uint{1, 9}) {
		t.Errorf("exact terms are rescued: %v", got)
	}
}

func TestRankFusionOfNothingIsNothing(t *testing.T) {
	if got := fuseRankings(nil, nil); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestRankFusionIsStableForEqualScores(t *testing.T) {
	a := fuseRankings([]uint{4, 3}, nil)
	b := fuseRankings([]uint{4, 3}, nil)
	if !slices.Equal(a, b) {
		t.Errorf("not deterministic: %v vs %v", a, b)
	}
}

// ---- asking the AI service ------------------------------------------------------------------------------

func aiServing(t *testing.T, handler http.HandlerFunc) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AI_SERVICE_URL", srv.URL)
	t.Cleanup(ResetEmbeddingBackoff)
	ResetEmbeddingBackoff()
	return &calls
}

func vectorsJSON(model string, dim int, vectors ...[]byte) string {
	encoded := make([]string, len(vectors))
	for i, v := range vectors {
		encoded[i] = `"` + base64.StdEncoding.EncodeToString(v) + `"`
	}
	return fmt.Sprintf(`{"model":%q,"dim":%d,"minScore":0.5,"vectors":[%s]}`, model, dim, strings.Join(encoded, ","))
}

func TestEmbedTextsReadsTheVectorsAndWhatTheServiceSaysAboutThem(t *testing.T) {
	aiServing(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(vectorsJSON("m1", 2, floats(1, 0), floats(0, 1))))
	})
	got, err := embedTexts(context.Background(), "req", []string{"a", "b"}, kindPassage)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "m1" || got.MinScore != 0.5 || len(got.Vectors) != 2 || dot(got.Vectors[0], floats(1, 0)) < 0.99 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestMalformedEmbeddingRepliesAreRejected(t *testing.T) {
	replies := map[string]string{
		"not json":          `oops`,
		"no model":          `{"dim":2,"vectors":["AAAAAAAAAAA="]}`,
		"no dimension":      `{"model":"m","dim":0,"vectors":["AAAAAAAAAAA="]}`,
		"too few vectors":   vectorsJSON("m", 2, floats(1, 0)),
		"wrong vector size": vectorsJSON("m", 3, floats(1, 0), floats(0, 1)),
		"not base64":        `{"model":"m","dim":2,"vectors":["%%%","%%%"]}`,
		"other service":     `{"summary":"fake summary"}`,
	}
	for name, body := range replies {
		t.Run(name, func(t *testing.T) {
			aiServing(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
			if _, err := embedTexts(context.Background(), "req", []string{"a", "b"}, kindPassage); err == nil {
				t.Error("a reply like this must not be trusted")
			}
		})
	}
}

func TestAFailureMakesTheServiceRestForAWhile(t *testing.T) {
	calls := aiServing(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"detail":"Embeddings are turned off"}`))
	})
	for range 3 {
		if _, err := embedTexts(context.Background(), "req", []string{"a"}, kindQuery); err == nil {
			t.Fatal("expected an error")
		}
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("the service should be asked once and then left alone, was asked %d times", n)
	}
	if _, err := embedTexts(context.Background(), "req", []string{"a"}, kindQuery); err != errEmbeddingsPaused {
		t.Errorf("a paused service says so: %v", err)
	}
}

func TestACancelledRequestDoesNotPauseEmbeddingsForEveryone(t *testing.T) {
	aiServing(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(vectorsJSON("m1", 2, floats(1, 0))))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := embedTexts(ctx, "req", []string{"a"}, kindQuery); err == nil {
		t.Fatal("a cancelled request cannot succeed")
	}
	if _, err := embedTexts(context.Background(), "req", []string{"a"}, kindQuery); err != nil {
		t.Errorf("one person leaving must not switch search by meaning off: %v", err)
	}
}
