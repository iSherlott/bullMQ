# only_publisher

Exemplo mínimo de **publisher** usando `github.com/iSherlott/bullMQ`.

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
```

2) Em um terminal na raiz do repositório, execute:

```bash
go run ./examples/only_publisher
```

## O que este exemplo faz

- Conecta no Redis.
- Publica um job em `BULLMQ_QUEUE_NAME` com payload JSON.
- Imprime o `job_id` retornado.

Para consumir o job, rode o exemplo [../only_consumer](../only_consumer).
