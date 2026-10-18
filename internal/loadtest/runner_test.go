package loadtest

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunExactRequestCountAndConcurrency(t *testing.T) {
	var hits, inFlight, maxInFlight int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt64(&inFlight, 1)
		for {
			m := atomic.LoadInt64(&maxInFlight)
			if cur <= m || atomic.CompareAndSwapInt64(&maxInFlight, m, cur) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		n := atomic.AddInt64(&hits, 1)
		atomic.AddInt64(&inFlight, -1)
		switch {
		case n%10 == 0:
			w.WriteHeader(http.StatusInternalServerError)
		case n%5 == 0:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	cfg := Config{URL: srv.URL, Requests: 100, Concurrency: 7, Timeout: 5 * time.Second}
	rep := Run(context.Background(), cfg)

	if hits != 100 || rep.Total != 100 {
		t.Fatalf("esperado 100 requests, servidor=%d relatório=%d", hits, rep.Total)
	}
	if rep.OK() != 80 || rep.StatusCounts[404] != 10 || rep.StatusCounts[500] != 10 {
		t.Fatalf("distribuição inesperada: %v", rep.StatusCounts)
	}
	if maxInFlight > 7 {
		t.Fatalf("concorrência excedida: %d", maxInFlight)
	}

	var buf bytes.Buffer
	rep.Print(&buf)
	for _, s := range []string{"Tempo total", "Total de requests realizados: 100", "status HTTP 200: 80", "[404]", "[500]"} {
		if !strings.Contains(buf.String(), s) {
			t.Errorf("relatório não contém %q:\n%s", s, buf.String())
		}
	}
}

func TestRunCountsConnectionErrors(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close() // servidor fechado => erros de conexão

	rep := Run(context.Background(), Config{URL: addr, Requests: 5, Concurrency: 10, Timeout: time.Second})
	if rep.Total != 5 || rep.Errors != 5 {
		t.Fatalf("esperado 5 erros, total=%d erros=%d", rep.Total, rep.Errors)
	}
}

func TestValidate(t *testing.T) {
	cases := []Config{
		{URL: "", Requests: 1, Concurrency: 1, Timeout: time.Second},
		{URL: "google.com", Requests: 1, Concurrency: 1, Timeout: time.Second},
		{URL: "http://x", Requests: 0, Concurrency: 1, Timeout: time.Second},
		{URL: "http://x", Requests: 1, Concurrency: 0, Timeout: time.Second},
	}
	for _, c := range cases {
		if c.Validate() == nil {
			t.Errorf("esperado erro para %+v", c)
		}
	}
	if err := (Config{URL: "http://x", Requests: 1, Concurrency: 1, Timeout: time.Second}).Validate(); err != nil {
		t.Errorf("erro inesperado: %v", err)
	}
}
