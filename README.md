# Stress-test

Sistema de CLI (Command Line Interface) em Go para realizar testes de carga em serviços web.

Você informa a URL, o número total de requisições e quantas chamadas simultâneas devem ser feitas. No final, a ferramenta mostra um relatório com:

- tempo total de execução;
- total de requests realizados;
- quantidade de requests com status HTTP 200;
- distribuição dos outros códigos de status (404, 500 etc.);
- erros de conexão e latências (extras).

---

## Como rodar o projeto

### Pré-requisitos

- [Git](https://git-scm.com/downloads)
- [Docker](https://docs.docker.com/get-docker/), com o serviço/Docker Desktop **em execução**
- *(Opcional, só para rodar sem Docker)* [Go 1.25+](https://go.dev/dl/)

### Passo 1 — Clonar o repositório

```bash
git clone https://github.com/Disneyjr/Stress-test.git
cd Stress-test
```

### Passo 2 — Buildar a imagem Docker

```bash
docker build -t stress-test .
```

Esse comando gera uma imagem local chamada `stress-test`. Para confirmar que ela foi criada:

```bash
docker images stress-test
```

### Passo 3 — Executar o teste de carga

```bash
docker run --rm stress-test --url=http://google.com --requests=1000 --concurrency=10
```

O que cada parte faz:

- `docker run --rm stress-test`: executa a imagem e remove o container quando o teste termina.
- `--url=http://google.com`: serviço que será testado.
- `--requests=1000`: faz exatamente 1000 requisições.
- `--concurrency=10`: mantém no máximo 10 requisições simultâneas.

Quando o teste termina, o relatório é exibido no terminal (veja o [exemplo de saída](#exemplo-de-saída)).

### Passo 4 — Ver as opções disponíveis

```bash
docker run --rm stress-test --help
```

---

## Parâmetros

| Flag            | Obrigatório | Padrão | Descrição                                     |
|-----------------|-------------|--------|-----------------------------------------------|
| `--url`         | sim         | —      | URL do serviço a ser testado (`http`/`https`) |
| `--requests`    | sim         | —      | Número total de requisições                   |
| `--concurrency` | não         | `1`    | Número de chamadas simultâneas                |
| `--timeout`     | não         | `30s`  | Timeout por requisição (ex.: `5s`, `500ms`)   |
| `--method`      | não         | `GET`  | Método HTTP                                   |

## Mais exemplos

```bash
# HTTPS com 500 requisições e 50 simultâneas
docker run --rm stress-test --url=https://example.com --requests=500 --concurrency=50

# Timeout menor por requisição
docker run --rm stress-test --url=https://example.com --requests=200 --concurrency=20 --timeout=5s
```

### Testando um serviço que roda na sua máquina

Dentro do container, `localhost` aponta para o próprio container, não para a sua máquina. Use uma destas opções:

```bash
# Windows / macOS (Docker Desktop)
docker run --rm stress-test --url=http://host.docker.internal:8080 --requests=500 --concurrency=50

# Linux
docker run --rm --network host stress-test --url=http://localhost:8080 --requests=500 --concurrency=50
```

---

## Rodando sem Docker (opcional)

Requer Go 1.25+ instalado.

```bash
# Executar direto
go run . --url=http://google.com --requests=100 --concurrency=10

# Ou gerar o binário e executar
go build -o stress-test .
./stress-test --url=http://google.com --requests=100 --concurrency=10
```

> No Windows, o binário é gerado como `stress-test.exe`: `go build -o stress-test.exe .` e depois `.\stress-test.exe --url=...`

## Rodando os testes automatizados

```bash
go test ./...
```

---

## Exemplo de saída

```
Iniciando teste de carga: http://google.com | requests=50 | concurrency=10

============================================================
                 RELATÓRIO DO TESTE DE CARGA
============================================================
URL alvo:                     http://google.com
Concorrência:                 10
Tempo total de execução:      1.212s
Total de requests realizados: 50 (solicitados: 50)
Requests com status HTTP 200: 50
Requests por segundo:         41.26
------------------------------------------------------------
Distribuição de códigos de status HTTP:
  [200] 50       100.00%
------------------------------------------------------------
Latência:
  min: 192.115ms | média: 223.758ms | max: 301.773ms
  p50: 205.533ms | p90: 286.752ms | p95: 287.254ms | p99: 301.773ms
============================================================
```

Requisições que falham sem resposta HTTP (timeout, conexão recusada, DNS etc.) também entram no total e aparecem em uma seção **Erros**, agrupadas por motivo.

## Problemas comuns

| Problema | Solução |
|----------|---------|
| `error during connect ... dockerDesktopLinuxEngine` | O Docker não está rodando. Abra o Docker Desktop e tente de novo. |
| `--url inválida` | Inclua o esquema na URL: `http://` ou `https://`. |
| Muitos erros `connect: connection refused` | O serviço não está acessível a partir do container. Veja [Testando um serviço que roda na sua máquina](#testando-um-serviço-que-roda-na-sua-máquina). |
| Muitos `timeout` | Diminua `--concurrency` ou aumente `--timeout`. |

---

## Como funciona

- Uma fila com exatamente `--requests` itens é consumida por `min(--concurrency, --requests)` workers (goroutines). Isso garante o número exato de requisições e respeita o limite de concorrência.
- O cliente HTTP reaproveita conexões (keep-alive). O corpo de cada resposta é lido por completo para que a conexão possa ser reutilizada.
- Redirecionamentos são seguidos; o status contabilizado é o da resposta final.
- `Ctrl+C` interrompe o teste e ainda exibe o relatório parcial (no Docker, use `docker run -it ...` para que o `Ctrl+C` chegue ao processo).

## Estrutura

```
.
├── main.go                     # parsing das flags e ponto de entrada da CLI
├── internal/loadtest/
│   ├── runner.go               # execução concorrente das requisições
│   ├── report.go               # agregação e impressão do relatório
│   └── runner_test.go          # testes com servidor HTTP local
├── Dockerfile                  # build multi-stage (imagem final baseada em scratch)
└── go.mod
```
