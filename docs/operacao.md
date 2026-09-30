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
| `max-instances` | **1**, enquanto o fan-out do stream da sessão ao vivo for em memória (ver [Stream da sessão ao vivo](#stream-da-sessão-ao-vivo)) |
| Timeout de requisição | Pelo menos 35 minutos (`--timeout=2100`, o máximo é 60 minutos): o stream vive até 30. O padrão do Cloud Run, 5 minutos, cortaria o stream antes |

O CockroachDB fica no Google Cloud, na mesma região, no plano atual do Samuel (decidido em 29/09/2026): o plano legado Unlimited, contratado antes da mudança de licenças de 2024. Trocar de plano perde o Unlimited. Os backups ficam em São Paulo e são guardados por no máximo 30 dias, para cumprir o prazo de exclusão (ver [Privacidade](privacidade.md)). Falta conferir essa configuração no console antes do primeiro deploy.

## Custos estimados (São Paulo)

Estimativas, não uma fatura. Servem para decidir arquitetura, como o stream só ficar aberto durante a sessão.

| Cenário | Custo estimado | Observação |
| --- | --- | --- |
| Base (uso normal da mesa) | ~US$ 0,15/mês | Cloud Run escala a zero fora das sessões. |
| Pesado (mais uso, mais mesas) | ~US$ 1,60–6,40/mês | Ainda assim, ordem de grandeza de poucos dólares. |
| Acidente: stream/WebSocket aberto o mês todo | ~US$ 55–88/mês | Por isso o stream do Connect só abre durante a sessão (ver [Arquitetura](arquitetura.md)). |
| Egress (saída de dados de São Paulo) | US$ 0,19/GiB | Sem free tier. Por isso as imagens de mapa vão para o Cloud Storage com URL, em vez de base64 na resposta (ver [Modelo de dados](dados.md)). |

## Stream da sessão ao vivo

O stream (`PlayService.WatchGameSession`, ADR-0005) só fica aberto enquanto alguém está com a página da sessão aberta e visível. Enquanto um stream está aberto, a instância do Cloud Run está atendendo uma requisição e é cobrada; por isso o servidor e o app limitam o tempo de cada um (ver [Arquitetura](arquitetura.md#sessão-ao-vivo)).

| Regra | Valor | Onde |
| --- | --- | --- |
| Heartbeat do servidor | A cada 25 segundos | `play.DefaultHeartbeat` |
| Stream morto, para o app | Nada chegou em uns 60 segundos: o app reconecta | App |
| Nova checagem da sessão de login e da participação | A cada 60 segundos, no banco | `play.DefaultRecheck` |
| Vida máxima de um stream | 30 minutos; depois o servidor encerra sem erro, e o app abre outro | `play.DefaultMaxLifetime` |
| Reconexão | Espera crescente: 1 s, 2 s, 4 s, até 30 s, com variação aleatória; só com a aba visível | App |
| Aba escondida | Depois de 2 minutos escondida, o app fecha o stream; ao voltar, reconecta e lê a foto de novo | App |
| Aviso de sessão | Consulta leve (`ListOpenGameSessions`) a cada 30 segundos com a aba visível, sem stream | App |
| Desligamento do servidor | Os streams terminam na hora, quando o graceful shutdown começa (`httpserver.Server.OnShutdown`), e os apps reconectam | `cmd/api` |

**`max-instances = 1` enquanto o fan-out for em memória.** A mudança feita pelo mestre é entregue aos streams pelo hub do `play/live`, na memória do servidor. Com duas instâncias, o mestre numa e o jogador na outra, a mudança não chegaria ao jogador. Uma instância só (1 vCPU, 512 MiB) atende a mesa com folga. Quando não bastar, o hub dá lugar a um canal compartilhado (changefeed do CockroachDB ou Pub/Sub), e o `max-instances` pode subir.

## Segredos

Credenciais (client secret do Google OAuth, connection string do banco, e outras) ficam no Secret Manager do Google Cloud, nunca em variável de ambiente solta no repositório ou no deploy. A lista exata de segredos por ambiente está **a definir**.

### Segredos e configuração do login

O login do mestre (módulo `identity`) precisa de um segredo novo, o client secret do provedor OIDC, e de alguns valores que não são segredo:

| Variável | Segredo? | Em produção |
| --- | --- | --- |
| `OIDC_CLIENT_SECRET` | Sim | Secret Manager, entregue ao Cloud Run como variável de ambiente |
| `DATABASE_URL` | Sim (tem a senha do banco) | Secret Manager |
| `OIDC_ISSUER` | Não | `https://accounts.google.com` |
| `OIDC_CLIENT_ID` | Não | Variável de ambiente do serviço |
| `OIDC_REDIRECT_URL` | Não | `https://<domínio>/auth/callback`, cadastrada igual no client OAuth do Google. Trocar de domínio pede cadastrar a URL nova antes do deploy |
| `OIDC_MAX_AGE` | Não | Não definir com o Google, que não documenta `max_age`. A reautenticação a cada 30 dias (NIST SP 800-63B-4) vem da sessão de 30 dias no servidor. No ambiente local, com o devidp, é `1h` |

O backend nunca escreve o client secret no log: o tipo `config.Secret` sai como `[REDACTED]`.

### O devidp nunca é deployado

O devidp (`backend/cmd/devidp`) é o provedor OIDC de desenvolvimento do `make up` e do CI. Ele loga qualquer pessoa como qualquer usuário de teste, sem senha, então não pode existir em nenhum ambiente de verdade:

- A imagem que vai para o Cloud Run é a do `backend/Dockerfile`, que só tem `api` e `migrate`. O devidp tem imagem própria (`deploy/local/devidp.Dockerfile`), que ninguém publica. O workflow `e2e` confere a cada PR que a imagem de produção não tem o binário.
- Se alguém tentar, ele não sobe: recusa issuer fora de loopback (`localhost`, `*.localhost`, `127.0.0.1`, `::1`) e recusa rodar no Cloud Run (`K_SERVICE` definido).

Em produção, o login é só pelo Google (`OIDC_ISSUER=https://accounts.google.com`).

### Limite de tentativas no login e o IP do cliente

`GET /auth/login` tem um limite por IP e um geral, em memória (ver [Arquitetura](arquitetura.md#limite-de-tentativas-no-login)). No Cloud Run, o IP do cliente é o último item do `X-Forwarded-For`, que o front end do Google acrescenta. Isso vale para o Cloud Run **sem load balancer na frente**, que é o plano atual.

- **No primeiro deploy, confira** que o último item do `X-Forwarded-For` é mesmo o IP de quem acessa: o Google documenta o formato para os load balancers, mas não diz quantos itens o front end acrescenta sem load balancer. Se o último item for de uma máquina do Google, todos os clientes dividem um limite só (sobra só o geral); ninguém consegue burlar o limite, mas o `cloudRunTrustedHops` de `ratelimit.ClientKey` precisa mudar.
- **Se um dia houver um load balancer na frente**, ele acrescenta `<ip-do-cliente>,<ip-do-load-balancer>`, e o IP do cliente passa a ser o penúltimo item: `cloudRunTrustedHops` vira 2.
- Com várias instâncias, cada uma tem os próprios contadores.

## Alertas de orçamento

Um budget alert no Google Cloud avisa se o custo passar do esperado. Os limiares exatos estão **a definir**.

## A definir

- Nome do domínio (sai do GitHub Student Developer Pack; só é necessário no primeiro deploy).
- Conferir no console do CockroachDB Cloud a região e a retenção dos backups (São Paulo, no máximo 30 dias).
- Avaliar se vale migrar do CockroachDB para o Cloud SQL ou outro produto, já que o plano legado não pode mudar sem perder o Unlimited. Pesa na conta: o código usa o TTL por linha do CockroachDB (sessões, estados de login e convites) e repete transações no erro `40001`; num PostgreSQL, a limpeza viraria um job agendado.
- Lista completa de segredos por ambiente e quem tem acesso.
- Limiares dos alertas de orçamento.

## Ver também

- [Arquitetura](arquitetura.md)
- [Modelo de dados](dados.md)
- `docs/adr/` (repositório privado): decisões de infraestrutura difíceis de desfazer.
