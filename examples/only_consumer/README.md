# only_consumer

Exemplo mínimo de **consumer** usando `github.com/iSherlott/bullMQ`.

## Como rodar

1) Crie um arquivo `.env` na raiz do repositório (porque o exemplo lê `LoadConfigFromEnv(".env")`).

Exemplo (Azure Redis):

```env
REDIS_HOST=your-redis.redis.cache.windows.net
REDIS_PORT=6380
REDIS_SSL=1
REDIS_PASSWORD=your_access_key
REDIS_DB=0

BULLMQ_KEY_PREFIX=bull
BULLMQ_QUEUE_NAME=notification_queue
BULLMQ_RETRY_LIMIT=3
BULLMQ_DEAD_LETTER_QUEUE=notification_queue_dead_letter
```

2) Em um terminal na raiz do repositório, execute:

```bash
go run ./examples/only_consumer
```

## O que este exemplo faz

- Conecta no Redis.
- Fica aguardando jobs na fila `BULLMQ_QUEUE_NAME`.
- Faz decode do payload JSON e imprime logs.

Para publicar um job de teste, rode o exemplo [../only_publisher](../only_publisher).
