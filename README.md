# Execution Service

Microsserviço responsável pela fila e execução da OS. Utiliza MongoDB próprio para o estado operacional e histórico de execução e publica eventos em RabbitMQ.

## Papel na arquitetura

O serviço consome a confirmação de pagamento, inicia a execução, registra o estado operacional no MongoDB e comunica a conclusão ao OS Service. Não acessa PostgreSQL nem bancos de outros serviços.

## Contratos

- `POST /executions/{orderID}/start`
- `POST /executions/{orderID}/finish`
- `POST /executions/{orderID}/fail` — registra falha e publica `ExecutionFailed`.
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
- Publica `execution.ExecutionFailed`; o OS Service cancela a OS e solicita estorno ao Billing Service.

### Tratamento de falha

```bash
curl -X POST http://localhost:8083/executions/ORDER_ID/fail
```

Esse endpoint publica `ExecutionFailed`. O OS Service marca a OS como `CANCELED` e publica `PaymentRefundRequested`, iniciando a compensação do pagamento no Billing Service.

Esse evento representa a compensação da etapa de execução. O serviço também mantém o estado local como `FAILED`, sem acessar o banco do OS Service.

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

## Testes e cobertura

Os testes unitários ficam em [tests](tests).

```bash
make tests
```

Cobertura atual:

```text
internal/application: 100%
```

O CI exige no mínimo 80% do pacote de regras de negócio e cobre transições da execução, eventos publicados, validação da OS e o evento de falha.

## Aderência à Fase 4

| Requisito | Implementação |
|---|---|
| Microsserviço independente | Módulo Go, Dockerfile, CI, Sonar e deploy próprios. |
| Banco próprio | MongoDB exclusivo do Execution Service. |
| Execução e produção | Início, consulta, finalização e falha da execução. |
| Mensageria | Consome `PaymentConfirmed` e publica eventos de execução. |
| Saga | Executa a etapa operacional da Saga orquestrada pelo OS Service. |
| Compensação | Publica `ExecutionFailed` para cancelar a OS e solicitar estorno. |
| Qualidade | Testes unitários, cobertura e SonarQube. |
| Kubernetes | Deployment e Service estão centralizados no [tech-challenge-infra-k8s](../tech-challenge-infra-k8s/k8s/base/microservices.yaml). |

## Manifestos Kubernetes

Os manifestos são aplicados pelo [tech-challenge-infra-k8s](../tech-challenge-infra-k8s/k8s/base/microservices.yaml). O Execution Service mantém o código e o ciclo de build/deploy independentes, enquanto a infraestrutura compartilhada do cluster permanece centralizada para evitar configurações divergentes entre ambientes.

O workflow deste repositório publica a imagem `tech-challenge-execution-service` e atualiza o Deployment no ambiente escolhido.
