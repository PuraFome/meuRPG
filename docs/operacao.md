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
| Egress (saída de dados de São Paulo) | US$ 0,19/GiB | Sem free tier. Por isso as imagens de mapa vão para o Cloud Storage e saem por uma URL própria, em vez de base64 dentro das respostas. O navegador do mestre guarda cada imagem por um ano (`private, immutable`) e a baixa uma vez; o do jogador pergunta de novo a cada uso (`no-cache`, para parar de ver o que o mestre escondeu), mas a resposta é um `304` sem corpo enquanto a imagem não muda, então cada aparelho ainda baixa cada imagem uma vez (ver [Arquitetura](arquitetura.md#servir-as-imagens)). |
| Imagens da galeria no Cloud Storage (Standard, São Paulo) | US$ 0,035 por GiB por mês ([preços do Cloud Storage](https://cloud.google.com/storage/pricing), consultados em 30/09/2026) | Uma campanha com a galeria cheia (500 MiB, o limite proposto) fica em ~US$ 0,02/mês. O soft delete de 7 dias cobra o mesmo preço pelo que foi apagado, durante esses dias. A API lê o bucket na mesma região, sem egress; cada leitura é uma operação classe B, cobrada por milhar (mesma página). |

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

## Peças da imagem da névoa

O servidor monta, para cada jogador, as peças da imagem de um mapa com névoa (MR-036, RN-10; [Arquitetura](arquitetura.md#as-peças-da-imagem-por-jogador-etapa-9-fatia-95), ADR-0016). É trabalho de CPU e de memória num servidor de 1 vCPU e 512 MiB, então tem orçamento:

| Regra | Valor | Onde |
| --- | --- | --- |
| Cópia de trabalho de um mapa | Até 2.048 px no lado maior (8,4 MB de RGBA para 4.096 × 2.048); decodificada uma vez, encolhida quadrado por quadrado | `maps/tiles.go`, `workingMaxSide` |
| Mapas com cópia na memória | 2, do servidor todo; o menos usado sai primeiro | `workingCopies` |
| Renderizações ao mesmo tempo | 1; os outros pedidos esperam até 8 s | `renderWait` |
| Faltas de cache por usuário | 60 de uma vez e 10 por segundo; passou, `429` com `Retry-After` | `newTileRenderer` |
| Cache das peças prontas | 32 MiB de PNG, contados em bytes, o menos usado sai primeiro | `tileCacheBytes` |
| Imagem recusada | A decodificação passaria de 192 MiB (PNG de 16 bits de mais de 24 megapixels, de antes de o envio guardar 8 bits) | `decodeLimit` |
| Pior caso da memória | Uns 280 MB mais uns 50 MB do resto, contra o `GOMEMLIMIT` de 400 MiB (a conta está no CONTRIBUTING) | CONTRIBUTING |

- **O `503` com `Retry-After: 2` e `reason: BUSY`** quer dizer que mais de uma renderização esperou mais de 8 s: acontece quando muitos jogadores abrem, ao mesmo tempo, um mapa que ninguém abriu desde que o servidor subiu (cada um precisa da primeira peça), ou logo depois de uma troca de grade ou de imagem. O app tenta de novo sozinho; as peças que já estão no cache não esperam nada. Um `429` com `reason: RATE_LIMITED` é um usuário pedindo peças demais sem cache; o app também tenta de novo.
- **O limite de capacidade: dois mapas com névoa em jogo ao mesmo tempo.** As cópias de trabalho são do servidor todo. Com três ou mais mapas com névoa sendo jogados juntos (várias campanhas na mesma instância), as cópias se revezam, e cada volta decodifica a imagem de novo (de 150 ms a 600 ms, segurando a vez de renderização e a vaga do envio): as primeiras peças demoram mais e o `503` fica mais provável. O remédio é subir `workingCopies` (cada cópia custa até 17 MB) ou ter mais instâncias.
- **Nada disso é guardado no disco nem no banco:** reiniciar o servidor esvazia a cópia de trabalho, as peças e as peças de cada jogador, e elas se refazem sozinhas. A memória do que cada jogador viu (o que decide as peças) é a de `map_vision_memory`.
- **A imagem de um mapa com névoa continua no blob store, uma vez só;** as peças não ocupam cota da galeria.

## O cache do conteúdo da mesa

O conteúdo que o mestre cadastra (MR-025, RN-23; [Arquitetura](arquitetura.md#o-conteúdo-da-mesa-ao-vivo-etapa-10-fatia-101c), ADR-0018) é montado na memória do servidor, por campanha e por revisão, e guardado num cache pequeno. É memória e CPU num servidor de 1 vCPU e 512 MiB, então tem orçamento:

| Regra | Valor | Onde |
| --- | --- | --- |
| Conteúdos no cache | 8, do servidor todo, por (campanha, revisão); o menos usado sai primeiro | `characters/tablesource.go`, `maxLiveContents` |
| Catálogos do `ListContent` | Até 8 conteúdos, cada um com o catálogo do mestre e o dos jogadores | `characters/contentsource.go`, `maxCatalogs` |
| Entradas por campanha | 300; 64 KiB de dados por entrada | `rules.MaxOverlayEntries`, `characters.MaxTableEntryBytes` |
| Um conteúdo montado | Uns **1,4 MB** retidos além do SRD (o SRD é um só, compartilhado), para uma mesa de 300 entradas (10 classes, 30 subclasses, 20 raças, 40 sub-raças, 40 antecedentes e 160 magias, com os dados de 135 KB) | `TestLiveContentMemory` |
| Os catálogos de um conteúdo | Uns **0,14 MB** (o do mestre e o dos jogadores; 75 KB no fio) | `TestLiveContentMemory` |
| Os 8 juntos | **Uns 12 MB** (11 MB de conteúdos e 1 MB de catálogos), medidos, no caso em que os dois caches guardam os mesmos 8 conteúdos | `TestLiveContentMemory` |
| **O pior caso** | **Uns 23 MB** (conta, não medida: 16 conteúdos × 1,37 MB = 22 MB, mais 1,1 MB de catálogos). O cache de catálogos (`maxCatalogs`) usa o conteúdo como chave e o mantém vivo, e tem a própria ordem de saída; quando as campanhas em jogo giram mais depressa que 8 revisões, os dois caches guardam conteúdos diferentes, até 8 + 8. Cabe nos 24 MB que o plano reservou e nos 400 MiB do `GOMEMLIMIT` (some aos 280 MB da conta das peças da névoa, no CONTRIBUTING) | derivado |
| Montar depois de uma escrita | De **6 a 9 ms** por campanha, uma vez por revisão (ler os dados, montar a sobreposição, o `With`); o pior caso que os orçamentos do motor deixam passar leva uns 31 ms | `TestLiveContentMemory`, `BenchmarkWith` |

- **Nada disso é cache do banco de verdade:** reiniciar o servidor o esvazia e ele se refaz sozinho, na primeira leitura de cada campanha. A revisão da campanha (`campaign_content_state`) é lida em **toda** leitura de conteúdo, dentro da transação de quem lê: é uma consulta pela chave primária, e é ela que faz uma edição do mestre valer na hora.
- **Com mais de 8 campanhas com conteúdo da mesa em jogo ao mesmo tempo** o cache dá voltas: cada leitura de uma campanha fora dele monta de novo (6 a 9 ms). Com uma instância só e uma mesa por vez, isso não acontece; quando acontecer, o número é `maxLiveContents`, e cada conteúdo a mais custa uns 1,5 MB.
- **Nenhum dado pessoal:** o conteúdo da mesa são nomes e textos que o mestre escreve para o jogo (ver [Privacidade](privacidade.md)).
- **Rodar de novo a medida:** `MEURPG_MEASURE=1 go test ./internal/characters -run TestLiveContentMemory -v` (em `backend/`, sem banco).

## Segredos

Credenciais (client secret do Google OAuth, connection string do banco, e outras) ficam no Secret Manager do Google Cloud, nunca em variável de ambiente solta no repositório ou no deploy. A lista exata de segredos por ambiente está **a definir**.

### Segredos e configuração do login

O login do mestre (módulo `identity`) precisa de um segredo novo, o client secret do provedor OIDC, e de alguns valores que não são segredo:

| Variável | Segredo? | Em produção |
| --- | --- | --- |
| `OIDC_CLIENT_SECRET` | Sim | Secret Manager, entregue ao Cloud Run como variável de ambiente |
| `DATABASE_URL` | Sim (tem a senha do banco) | Secret Manager. Aceita `pool_max_conns=N` (e `connect_timeout`); sem `pool_max_conns`, o pool abre no máximo 10 conexões (ver [O pool de conexões](#o-pool-de-conexões)) |
| `OIDC_ISSUER` | Não | `https://accounts.google.com` |
| `OIDC_CLIENT_ID` | Não | Variável de ambiente do serviço |
| `OIDC_REDIRECT_URL` | Não | `https://<domínio>/auth/callback`, cadastrada igual no client OAuth do Google. Trocar de domínio pede cadastrar a URL nova antes do deploy |
| `OIDC_MAX_AGE` | Não | Não definir com o Google, que não documenta `max_age`. A reautenticação a cada 30 dias (NIST SP 800-63B-4) vem da sessão de 30 dias no servidor. No ambiente local, com o devidp, é `1h` |

O backend nunca escreve o client secret no log: o tipo `config.Secret` sai como `[REDACTED]`.

### O pool de conexões

O backend fala com o CockroachDB por um pool do pgx. Sem `pool_max_conns` na `DATABASE_URL`, o pool abre no máximo **10** conexões (o padrão do pgx, `max(4, vCPUs)`, daria 4 no Cloud Run de 1 vCPU). Para mudar o tamanho, ponha `?pool_max_conns=N` na connection string do Secret Manager: uma versão nova do segredo só vale numa revisão nova do Cloud Run ou quando uma instância sobe de novo, então é preciso fazer o deploy de uma revisão (ou deixar a instância reiniciar) depois de criar a versão.

- **Por que 10.** A instância é uma só (`max-instances` 1), e a mesa é um mestre e até seis jogadores; cada requisição é uma transação curta. 10 cobre as leituras periódicas dos streams e uma rajada de ações, e fica muito abaixo do que o CockroachDB recomenda (cerca de quatro conexões por vCPU do cluster, somadas entre as instâncias). Se um dia `max-instances` subir, o total (instâncias × `pool_max_conns`) é o que conta.
- **Esperar uma conexão é normal; esperar para sempre não é.** Com o pool cheio, uma requisição espera a vez. Uma requisição **nunca** pede uma segunda conexão enquanto a transação dela segura a primeira (ver [Arquitetura](arquitetura.md#transações-e-o-pool-de-conexões)): foi esse erro que travou 5 requisições por 176 s no CI. Se aparecerem `context canceled` ou `context deadline exceeded` em leituras simples ("look up session", "get membership") junto com uma requisição lenta, é esse o sintoma: procure uma leitura pelo pool dentro de um `db.InTx`.
- **Memória e custo.** Cada conexão ocupa memória no nó do CockroachDB; por isso o pool é pequeno e não cresce sozinho.

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

### Imagens

As imagens da galeria (MR-019) ficam num blob store (ver [Arquitetura](arquitetura.md#onde-as-imagens-ficam)).

| Variável | Segredo? | Onde |
| --- | --- | --- |
| `BLOB_DIR` | Não | A pasta das imagens em disco. No ambiente local, `/var/lib/meurpg/images`, num volume do Docker Compose. Sem ela, as imagens ficam desligadas (`503`). Em produção, não se usa disco: vem o Cloud Storage (abaixo) |

**No primeiro deploy (a implementação do Cloud Storage ainda não existe, porque não há deploy):** um bucket só para as imagens, em `southamerica-east1`, classe Standard, com acesso uniforme no nível do bucket e prevenção de acesso público; soft delete de 7 dias (o prazo da [Privacidade](privacidade.md)); e só a conta de serviço da API com acesso (`roles/storage.objectUser` no bucket), sem nenhuma URL pública nem URL assinada: quem entrega a imagem é sempre a API, depois de conferir quem pede. A implementação entra no pacote `blob`, atrás da mesma interface.

**Memória.** O envio processa uma imagem por vez em cada instância, e recusa a imagem cuja decodificação passaria de 256 MiB (estimativa do pacote `maps/images`). Com 512 MiB por instância, sobra espaço para o resto, desde que o coletor de lixo do Go saiba o limite: definir `GOMEMLIMIT` (por exemplo, `400MiB`) no primeiro deploy. O pior caso das peças da névoa, somado, é de uns 330 MB (ver [Peças da imagem da névoa](#peças-da-imagem-da-névoa)); os envios de PNG passam a ser guardados com 8 bits por canal, para que decodificá-los depois custe 4 bytes por pixel.

## Alertas de orçamento

Um budget alert no Google Cloud avisa se o custo passar do esperado. Os limiares exatos estão **a definir**.

## A definir

- Nome do domínio (sai do GitHub Student Developer Pack; só é necessário no primeiro deploy).
- Conferir no console do CockroachDB Cloud a região e a retenção dos backups (São Paulo, no máximo 30 dias).
- Avaliar se vale migrar do CockroachDB para o Cloud SQL ou outro produto, já que o plano legado não pode mudar sem perder o Unlimited. Pesa na conta: o código usa o TTL por linha do CockroachDB (sessões, estados de login e convites) e repete transações no erro `40001`; num PostgreSQL, a limpeza viraria um job agendado.
- Lista completa de segredos por ambiente e quem tem acesso.
- Limiares dos alertas de orçamento.
- O bucket das imagens e a implementação do Cloud Storage no pacote `blob` (bucket privado em São Paulo, soft delete de 7 dias, só a conta de serviço da API), e `GOMEMLIMIT` no Cloud Run (ver [Imagens](#imagens)).

## Ver também

- [Arquitetura](arquitetura.md)
- [Modelo de dados](dados.md)
- `docs/adr/` (repositório privado): decisões de infraestrutura difíceis de desfazer.
