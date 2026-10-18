package loadtest

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Report consolida as métricas da execução.
type Report struct {
	URL           string
	Concurrency   int
	Requested     int
	TotalDuration time.Duration
	Total         int
	StatusCounts  map[int]int
	ErrorCounts   map[string]int
	Errors        int
	Interrupted   bool
	latencies     []time.Duration
}

func newReport(cfg Config) *Report {
	return &Report{
		URL:          cfg.URL,
		Concurrency:  cfg.Concurrency,
		Requested:    cfg.Requests,
		StatusCounts: make(map[int]int),
		ErrorCounts:  make(map[string]int),
		latencies:    make([]time.Duration, 0, cfg.Requests),
	}
}

func (r *Report) add(res Result) {
	r.Total++
	r.latencies = append(r.latencies, res.Duration)
	if res.Err != nil {
		r.Errors++
		r.ErrorCounts[classifyError(res.Err)]++
		return
	}
	r.StatusCounts[res.StatusCode]++
}

// OK retorna a quantidade de respostas HTTP 200.
func (r *Report) OK() int { return r.StatusCounts[200] }

func classifyError(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Sprintf("%s: %v", opErr.Op, opErr.Err)
	}
	return err.Error()
}

func (r *Report) percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted))*p+0.5) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// Print escreve o relatório formatado no writer informado.
func (r *Report) Print(w io.Writer) {
	line := strings.Repeat("=", 60)
	fmt.Fprintln(w, line)
	fmt.Fprintln(w, "                 RELATÓRIO DO TESTE DE CARGA")
	fmt.Fprintln(w, line)
	fmt.Fprintf(w, "URL alvo:                     %s\n", r.URL)
	fmt.Fprintf(w, "Concorrência:                 %d\n", r.Concurrency)
	fmt.Fprintf(w, "Tempo total de execução:      %s\n", r.TotalDuration.Round(time.Millisecond))
	fmt.Fprintf(w, "Total de requests realizados: %d (solicitados: %d)\n", r.Total, r.Requested)
	fmt.Fprintf(w, "Requests com status HTTP 200: %d\n", r.OK())
	if r.TotalDuration > 0 {
		fmt.Fprintf(w, "Requests por segundo:         %.2f\n", float64(r.Total)/r.TotalDuration.Seconds())
	}
	if r.Interrupted {
		fmt.Fprintln(w, "ATENÇÃO: execução interrompida antes de concluir todas as requisições.")
	}

	fmt.Fprintln(w, strings.Repeat("-", 60))
	fmt.Fprintln(w, "Distribuição de códigos de status HTTP:")
	codes := make([]int, 0, len(r.StatusCounts))
	for c := range r.StatusCounts {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	if len(codes) == 0 {
		fmt.Fprintln(w, "  (nenhuma resposta HTTP recebida)")
	}
	for _, c := range codes {
		fmt.Fprintf(w, "  [%d] %-8d %6.2f%%\n", c, r.StatusCounts[c], pct(r.StatusCounts[c], r.Total))
	}

	if r.Errors > 0 {
		fmt.Fprintln(w, strings.Repeat("-", 60))
		fmt.Fprintf(w, "Erros (sem resposta HTTP): %d (%.2f%%)\n", r.Errors, pct(r.Errors, r.Total))
		msgs := make([]string, 0, len(r.ErrorCounts))
		for m := range r.ErrorCounts {
			msgs = append(msgs, m)
		}
		sort.Slice(msgs, func(i, j int) bool { return r.ErrorCounts[msgs[i]] > r.ErrorCounts[msgs[j]] })
		for _, m := range msgs {
			fmt.Fprintf(w, "  %-8d %s\n", r.ErrorCounts[m], m)
		}
	}

	if len(r.latencies) > 0 {
		sorted := append([]time.Duration(nil), r.latencies...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		var sum time.Duration
		for _, d := range sorted {
			sum += d
		}
		fmt.Fprintln(w, strings.Repeat("-", 60))
		fmt.Fprintln(w, "Latência:")
		fmt.Fprintf(w, "  min: %s | média: %s | max: %s\n",
			sorted[0].Round(time.Microsecond),
			(sum / time.Duration(len(sorted))).Round(time.Microsecond),
			sorted[len(sorted)-1].Round(time.Microsecond))
		fmt.Fprintf(w, "  p50: %s | p90: %s | p95: %s | p99: %s\n",
			r.percentile(sorted, 0.50).Round(time.Microsecond),
			r.percentile(sorted, 0.90).Round(time.Microsecond),
			r.percentile(sorted, 0.95).Round(time.Microsecond),
			r.percentile(sorted, 0.99).Round(time.Microsecond))
	}
	fmt.Fprintln(w, line)
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) * 100 / float64(total)
}
