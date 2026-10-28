package typesafe

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codenerd/internal/config"
)

func nanValue() float64 { return math.NaN() }

func TestFromConfig_AbsentSectionIsDisabled(t *testing.T) {
	c, err := FromConfig(nil)
	if c != nil || err != nil {
		t.Errorf("FromConfig(nil) = %v, %v", c, err)
	}
}

func TestFromConfig_BuildsFromTheSection(t *testing.T) {
	t.Setenv("TS_FROM_CFG_KEY", "env-key")
	var auth, long string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, long = r.Header.Get("Authorization"), r.Header.Get("TypeSafe-Danger-Zone-Long-Context")
		io.WriteString(w, answersJSON)
	}))
	defer srv.Close()
	c, err := FromConfig(&config.TypeSafeConfig{Model: "jev-1.14.0", BaseURL: srv.URL, APIKeyEnv: "TS_FROM_CFG_KEY", RequestTimeout: "9s", LongContext: true})
	if err != nil || c == nil {
		t.Fatalf("FromConfig = %v, %v", c, err)
	}
	if c.Model() != "jev-1.14.0" || c.timeout != 9*time.Second {
		t.Errorf("model=%q timeout=%v", c.Model(), c.timeout)
	}
	if _, err := c.Ask(context.Background(), "x", testQuestions()); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer env-key" || long != "true" {
		t.Errorf("auth=%q long=%q", auth, long)
	}
}

func TestFromConfig_PresentButBrokenIsAnError(t *testing.T) {
	for name, sec := range map[string]*config.TypeSafeConfig{
		"no model":    {BaseURL: "https://x.test", APIKey: "k"},
		"no base url": {Model: "m", APIKey: "k"},
		"unset env":   {Model: "m", BaseURL: "https://x.test", APIKeyEnv: "TS_UNSET_FROM_CFG"},
	} {
		if c, err := FromConfig(sec); c != nil || err == nil {
			t.Errorf("%s: %v, %v", name, c, err)
		}
	}
}
