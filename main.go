package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Disneyjr/Stress-test/internal/loadtest"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("stress-test", flag.ContinueOnError)
	cfg := loadtest.Config{}
	fs.StringVar(&cfg.URL, "url", "", "URL do serviço a ser testado (obrigatório)")
	fs.IntVar(&cfg.Requests, "requests", 0, "Número total de requisições (obrigatório)")
	fs.IntVar(&cfg.Concurrency, "concurrency", 1, "Número de chamadas simultâneas")
	fs.DurationVar(&cfg.Timeout, "timeout", 30*time.Second, "Timeout de cada requisição (ex: 10s, 500ms)")
	fs.StringVar(&cfg.Method, "method", "GET", "Método HTTP")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Uso: stress-test --url=<url> --requests=<n> --concurrency=<n>")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "Erro:", err)
		fs.Usage()
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Iniciando teste de carga: %s | requests=%d | concurrency=%d\n\n",
		cfg.URL, cfg.Requests, cfg.Concurrency)

	report := loadtest.Run(ctx, cfg)
	report.Print(os.Stdout)

	if report.Interrupted {
		return 130
	}
	return 0
}
