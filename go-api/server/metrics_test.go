package server

import (
	"bufio"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

const metricsToken = "test-metrics-token"

// scrape fetches /metrics with the right token and returns each series as "name{labels}" -> value.
func scrape(t *testing.T, a *app) map[string]float64 {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, a.srv.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+metricsToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/metrics: got %d", resp.StatusCode)
	}
	series := map[string]float64{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.LastIndex(line, " ")
		if v, err := strconv.ParseFloat(line[i+1:], 64); err == nil {
			series[line[:i]] = v
		}
	}
	return series
}

func TestMetricsEndpointIsOffUnlessATokenIsConfigured(t *testing.T) {
	t.Setenv("METRICS_TOKEN", "")
	a := newApp(t)
	if r := a.newClient().get("/metrics"); r.Status != http.StatusNotFound {
		t.Errorf("without METRICS_TOKEN the endpoint must not exist, got %d", r.Status)
	}
}

func TestMetricsRequireTheBearerToken(t *testing.T) {
	t.Setenv("METRICS_TOKEN", metricsToken)
	a := newApp(t)
	cl := a.newClient()

	for name, header := range map[string][]string{
		"no credentials":                  nil,
		"wrong token":                     {"Authorization", "Bearer nope"},
		"token without the Bearer prefix": {"Authorization", metricsToken},
		"empty bearer":                    {"Authorization", "Bearer "},
	} {
		if r := cl.req(http.MethodGet, "/metrics", nil, header...); r.Status != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, r.Status)
		}
	}
	if r := cl.req(http.MethodGet, "/metrics", nil, "Authorization", "Bearer "+metricsToken); r.Status != http.StatusOK {
		t.Errorf("correct token: got %d", r.Status)
	}
}

func TestMetricsCountRequestsAndEvents(t *testing.T) {
	t.Setenv("METRICS_TOKEN", metricsToken)
	a := newApp(t)
	// newApp clears the quota variables, and the limit is read per request, so set it afterwards.
	t.Setenv("QUOTA_SUMMARIES_PER_DAY", "1")
	before := scrape(t, a)

	anon := a.newClient()
	anon.get("/healthz")
	anon.get("/definitely/not/a/route/zq81x") // unknown URLs must not create a series each
	anon.post("/login", map[string]string{"email": "nobody@example.com", "password": "Whatever-1234"})

	cl, _ := a.newUser()
	cl.post("/summarize-text", map[string]string{"text": "first request is within the quota"})
	cl.post("/summarize-text", map[string]string{"text": "second request is over it"})

	after := scrape(t, a)
	delta := func(series string) float64 { return after[series] - before[series] }

	for series, want := range map[string]float64{
		`http_requests_total{method="GET",route="/healthz",status="200"}`:         1,
		`http_requests_total{method="GET",route="unmatched",status="404"}`:        1,
		`http_requests_total{method="POST",route="/login",status="401"}`:          1,
		`account_events_total{event="login_failed"}`:                              1,
		`account_events_total{event="signup"}`:                                    1,
		`ai_service_requests_total{endpoint="/summarize-text",outcome="ok"}`:      1,
		`quota_rejections_total{kind="summaries"}`:                                1,
		`http_requests_total{method="POST",route="/summarize-text",status="429"}`: 1,
	} {
		if got := delta(series); got != want {
			t.Errorf("%s changed by %v, want %v", series, got, want)
		}
	}

	for series := range after {
		if strings.Contains(series, "zq81x") {
			t.Errorf("a raw URL leaked into a label: %s", series)
		}
	}
}
