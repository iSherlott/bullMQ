# bullMQ

Pacote Go reutilizável para publicar e consumir jobs em filas BullMQ (Redis), com fluxo de `ack`, `retry` e `dead_letter`.

## Instalação (estilo npm)

```bash
go get github.com/iSherlott/bullMQ@latest
```

Versão específica:

```bash
go get github.com/iSherlott/bullMQ@v0.2.0
```

## Uso em outro projeto

```go
import (
    "context"

    bullmq "github.com/iSherlott/bullMQ"
)

func main() {
    config, err := bullmq.LoadConfigFromEnv(".env")
    if err != nil {
        panic(err)
    }

    publisher, consumer := bullmq.New(config)

    _, err = publisher.Publish(context.Background(), "notification_queue", "send_notification", []byte(`{"message":"hello"}`))
    if err != nil {
        panic(err)
    }

    err = consumer.Consume(context.Background(), "notification_queue", func(ctx context.Context, job bullmq.Job) error {
        return nil
    }, bullmq.ConsumerOptions{
        RetryLimit:      3,
        DeadLetterQueue: "notification_queue_dead_letter",
    })
    if err != nil {
        panic(err)
    }
}
```

## Variáveis de ambiente

Em produção, configure **apenas variáveis de ambiente**. O arquivo `.env` é recomendado somente para desenvolvimento/teste local.

```env
REDIS_HOST=<REDIS_NAME>.redis.cache.windows.net
REDIS_PORT=6380
REDIS_SSL=1
REDIS_PASSWORD=your_redis_access_key
REDIS_DB=1

BULLMQ_KEY_PREFIX=bull
BULLMQ_QUEUE_NAME=SendEmail
BULLMQ_RETRY_LIMIT=1
BULLMQ_DEAD_LETTER_QUEUE=SendEmail_dead_letter
```

Compatibilidade mantida:
- Se `REDIS_ADDR` estiver definido, ele também funciona.
- Se `REDIS_HOST` já vier com porta (`host:port`), ele é respeitado.
- Evite espaço após `=` no `.env` (ex.: use `REDIS_PASSWORD=valor`, não `REDIS_PASSWORD= valor`).

Observação: `LoadConfigFromEnv(".env")` não falha se o arquivo não existir (útil em produção), mas falha se o arquivo existir e estiver inválido.

Nos exemplos `only_publisher` e `only_consumer`, `BULLMQ_QUEUE_NAME`, `BULLMQ_RETRY_LIMIT` e `BULLMQ_DEAD_LETTER_QUEUE` são lidas automaticamente do `.env`.

## Versionamento (SemVer)

- Este projeto segue Semantic Versioning.
- `MAJOR`: quebra de compatibilidade
- `MINOR`: funcionalidade nova compatível
- `PATCH`: correção sem quebrar API

Crie releases com tags Git:

```bash
git tag v0.2.0
git push origin v0.2.0
```

Ao subir uma tag `v*`, o workflow [release.yml](.github/workflows/release.yml) roda testes e cria a GitHub Release automaticamente.

## Estrutura do pacote

- API pública no root (`bullmq`).
- Persistência/Redis em `redis/store.go` (package `redis`).
- Status/state em `state/` (arquivos: `completed`, `failed`, `waiting`, `processing`, `pause`).
- Process sandbox (child process) em `sandbox/`.
- Sem camadas profundas como `pkg/bullmq_pkg`, facilitando consumo em outros projetos.

## API principal

- `New(config)` cria `publisher` e `consumer`.
- `publisher.Publish(ctx, queueName, jobName, payload)` publica jobs.
- `consumer.Consume(ctx, queueName, handler, options)` consome jobs com `ack/retry/dead_letter`.
- `consumer.ConsumeSandboxed(ctx, queueName, process, options)` consome jobs executando o handler em um processo separado (via `sandbox/`).
- O consumer mantém o job no Redis e apenas atualiza `status` (`waiting` -> `processing` -> `completed`/`failed`).

## Sandboxed (via processo)

Use quando você precisa isolar execução em outro binário/processo.

- O filho fala um protocolo JSON via stdin/stdout (veja `sandbox.RunChild`).
- `stderr` do filho é reservado para logs.

## Exemplos separados

Somente publisher (módulos pequenos):

- [examples/only_publisher/main.go](examples/only_publisher/main.go)
- [examples/only_publisher/app/config.go](examples/only_publisher/app/config.go)
- [examples/only_publisher/app/message.go](examples/only_publisher/app/message.go)
- [examples/only_publisher/app/publisher.go](examples/only_publisher/app/publisher.go)

Executar:

```bash
go run ./examples/only_publisher
```

Instruções: [examples/only_publisher/README.md](examples/only_publisher/README.md)

Somente consumer (módulos pequenos):

- [examples/only_consumer/main.go](examples/only_consumer/main.go)
- [examples/only_consumer/app/config.go](examples/only_consumer/app/config.go)
- [examples/only_consumer/app/handler.go](examples/only_consumer/app/handler.go)
- [examples/only_consumer/app/consumer.go](examples/only_consumer/app/consumer.go)

Executar:

```bash
go run ./examples/only_consumer
```

Instruções: [examples/only_consumer/README.md](examples/only_consumer/README.md)
