# Operação

O objetivo é custo perto de zero: sem sessão ativa, nada fica rodando nem conectado.

## Hospedagem

O backend roda no Cloud Run, em `southamerica-east1` (São Paulo).

| Item | Valor |
| --- | --- |
| Região | `southamerica-east1` |
| CPU | 1 vCPU |
| Memória | 512 MiB |
| `min-instances` | 0 (escala a zero sem uso) |
| `max-instances` | Baixo, valor exato **a definir** |

O CockroachDB fica no Google Cloud, na mesma região. O plano e o tamanho do cluster estão **a definir**.

## Custos estimados (São Paulo)

Estimativas, não uma fatura. Servem para decidir arquitetura, como o stream só ficar aberto durante a sessão.

| Cenário | Custo estimado | Observação |
| --- | --- | --- |
| Base (uso normal da mesa) | ~US$ 0,15/mês | Cloud Run escala a zero fora das sessões. |
| Pesado (mais uso, mais mesas) | ~US$ 1,60–6,40/mês | Ainda assim, ordem de grandeza de poucos dólares. |
| Acidente: stream/WebSocket aberto o mês todo | ~US$ 55–88/mês | Por isso o stream do Connect só abre durante a sessão (ver [Arquitetura](arquitetura.md)). |
| Egress (saída de dados de São Paulo) | US$ 0,19/GiB | Sem free tier. Por isso as imagens de mapa vão para o Cloud Storage com URL, em vez de base64 na resposta (ver [Modelo de dados](dados.md)). |

## Segredos

Credenciais (client secret do Google OAuth, connection string do banco, e outras) ficam no Secret Manager do Google Cloud, nunca em variável de ambiente solta no repositório ou no deploy. A lista exata de segredos por ambiente está **a definir**.

## Alertas de orçamento

Um budget alert no Google Cloud avisa se o custo passar do esperado. Os limiares exatos estão **a definir**.

## A definir

- Nome do domínio (sai do GitHub Student Developer Pack; só é necessário no primeiro deploy).
- `max-instances` exato do Cloud Run.
- Plano e tamanho do cluster do CockroachDB.
- Lista completa de segredos por ambiente e quem tem acesso.
- Limiares dos alertas de orçamento.
- Política de ociosidade do stream em tempo real (ADR-0005, proposta).

## Ver também

- [Arquitetura](arquitetura.md)
- [Modelo de dados](dados.md)
- `docs/adr/` (repositório privado): decisões de infraestrutura difíceis de desfazer.
