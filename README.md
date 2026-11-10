# Execution Service

Microsserviço responsável pela fila e execução da OS. Utiliza MongoDB próprio para o estado operacional e histórico de execução e publica eventos em RabbitMQ.

## Papel na arquitetura

O serviço consome a confirmação de pagamento, inicia a execução, registra o estado operacional no MongoDB e comunica a conclusão ao OS Service. Não acessa PostgreSQL nem bancos de outros serviços.

## Contratos

- `POST /executions/{orderID}/start`
- `POST /executions/{orderID}/finish`
- `GET /executions/{orderID}`
- `GET /health`

Contrato OpenAPI: [docs/openapi.yaml](docs/openapi.yaml).

Exemplos:

```bash
curl -X POST http://localhost:8083/executions/ORDER_ID/start
curl http://localhost:8083/executions/ORDER_ID
curl -X POST http://localhost:8083/executions/ORDER_ID/finish
```

Configuração: `PORT`, `MONGODB_URL`, `MONGODB_DATABASE` e `RABBITMQ_URL`.

## Mensageria

- Exchange: `tech-challenge.events`.
- Consome `billing.PaymentConfirmed` na fila `execution-service-payments`.
- Publica `execution.ExecutionStarted`.
- Publica `execution.ServiceOrderExecutionCompleted`.

## Execução local

```bash
make tests
make build
docker compose -f ../tech-challenge-project/docker-compose.phase4.yml up --build execution-service
```

## CI/CD e observabilidade

- `ci.yml`: formatação, `go vet`, testes, cobertura e build.
- `sonar.yml`: qualidade do código.
- `deploy.yml`: publicação da imagem e rollout no Kubernetes.
- Healthchecks e labels Datadog estão previstos nos manifests de infraestrutura.

## Cobertura e pendências

Os testes ficam em [tests](tests) e cobrem 100% do pacote `internal/application`, acima do mínimo de 80% nesse escopo. Ainda faltam testes de integração com MongoDB/RabbitMQ, histórico completo de diagnóstico, fila persistente, retry/DLQ e armazenamento durável com PVC/StatefulSet no Kubernetes.
