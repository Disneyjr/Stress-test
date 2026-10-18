// Package loadtest contém a lógica de execução do teste de carga.
package loadtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Config define os parâmetros de execução do teste.
type Config struct {
	URL         string
	Requests    int
	Concurrency int
	Timeout     time.Duration
	Method      string
}

// Validate verifica se a configuração é válida.
func (c Config) Validate() error {
	if c.URL == "" {
		return errors.New("--url é obrigatório")
	}
	u, err := url.ParseRequestURI(c.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("--url inválida: %q (use http:// ou https://)", c.URL)
	}
	if c.Requests <= 0 {
		return errors.New("--requests deve ser maior que zero")
	}
	if c.Concurrency <= 0 {
		return errors.New("--concurrency deve ser maior que zero")
	}
	if c.Timeout <= 0 {
		return errors.New("--timeout deve ser maior que zero")
	}
	return nil
}

// Result representa o resultado de uma única requisição.
type Result struct {
	StatusCode int // 0 quando houve erro (sem resposta HTTP)
	Err        error
	Duration   time.Duration
}

// Run executa o teste de carga e retorna o relatório consolidado.
// Exatamente cfg.Requests requisições são disparadas, salvo cancelamento do ctx.
func Run(ctx context.Context, cfg Config) *Report {
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	workers := cfg.Concurrency
	if workers > cfg.Requests {
		workers = cfg.Requests
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        workers,
			MaxIdleConnsPerHost: workers,
			MaxConnsPerHost:     0,
			IdleConnTimeout:     90 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}
	defer client.CloseIdleConnections()

	jobs := make(chan struct{}, cfg.Requests)
	for i := 0; i < cfg.Requests; i++ {
		jobs <- struct{}{}
	}
	close(jobs)

	results := make(chan Result, workers*2)
	report := newReport(cfg)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				if ctx.Err() != nil {
					return
				}
				results <- doRequest(ctx, client, cfg.Method, cfg.URL)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		report.add(r)
	}
	report.TotalDuration = time.Since(start)
	report.Interrupted = ctx.Err() != nil
	return report
}

func doRequest(ctx context.Context, client *http.Client, method, target string) Result {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return Result{Err: err, Duration: time.Since(start)}
	}
	req.Header.Set("User-Agent", "stress-test-cli/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Result{Err: err, Duration: time.Since(start)}
	}
	// Consome o corpo para permitir reaproveitamento da conexão (keep-alive).
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return Result{StatusCode: resp.StatusCode, Duration: time.Since(start)}
}
