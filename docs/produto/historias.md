# Histórias e critérios de aceite

O MVP tem 16 histórias, mais 2 pré-requisitos — 18 no total até o MVP: as 10 marcadas como MVP no plano, as duas que o Samuel acrescentou em 28/09/2026 (MR-015 e MR-016), as três que o Samuel pôs no MVP em 29/09/2026 (MR-018 e MR-019, documento de campanha e galeria de imagens, e MR-024, aprovar o personagem do convite), a que o Vinicius pôs no MVP em 30/09/2026 (MR-028, mostrar uma imagem aos jogadores), e duas marcadas "MVP (pré-requisito)" desde 29/09/2026: o convite (MR-002), que leva à MR-003, e os NPCs (MR-005), que são os inimigos do combate. "Já existe em parte" não é mais uma prioridade: o app antigo é descontinuado, então nenhuma história "já existe" no sistema novo — MR-002 e MR-005 entram como qualquer outra história do MVP, com os próprios testes.

Uma história está pronta quando todos os critérios dela passam. Cada critério vira um teste automático: Playwright para o que aparece na tela, teste em Go para a regra no servidor. Não há testes de caracterização do app antigo — o sistema novo só precisa provar os próprios critérios de aceite.

MR-021 e MR-022 são novas e saíram das respostas do Samuel de 28/09/2026. Ficaram como "Depois", mas o modelo de dados já nasce preparado para as duas (ver [Modelo de dados](../dados.md)). MR-023 e MR-024 são novas, das respostas do Samuel de 29/09/2026 (mais de um mestre, RN-13; convite com aprovação, RN-15). No mesmo dia, ele pôs a MR-024 no MVP, na Etapa 4, e a MR-023 depois do MVP.

## Índice

| ID | Área | Prioridade |
| --- | --- | --- |
| [MR-001](#mr-001-criar-campanha) | Campanha | MVP |
| [MR-003](#mr-003-entrar-pelo-convite) | Personagem | MVP |
| [MR-004](#mr-004-ficha-no-formato-do-pdf) | Personagem | MVP |
| [MR-006](#mr-006-ficha-travada) | Personagem | MVP |
| [MR-008](#mr-008-pontos-de-interesse) | Mapa | MVP |
| [MR-009](#mr-009-mapa-sem-spoiler) | Mapa | MVP |
| [MR-011](#mr-011-iniciar-a-sessão) | Sessão | MVP |
| [MR-012](#mr-012-acompanhar-a-sessão) | Sessão | MVP |
| [MR-013](#mr-013-ordem-dos-turnos) | Combate | MVP |
| [MR-014](#mr-014-sua-vez) | Combate | MVP |
| [MR-015](#mr-015-ações-da-cena-de-rp) | RP | MVP |
| [MR-016](#mr-016-dar-xp) | Progressão | MVP |
| [MR-018](#mr-018-documento-de-campanha) | Apoio | MVP |
| [MR-019](#mr-019-galeria-de-imagens) | Apoio | MVP |
| [MR-024](#mr-024-aprovar-o-personagem-do-convite) | Personagem | MVP |
| [MR-028](#mr-028-mostrar-uma-imagem-aos-jogadores) | Sessão | MVP |
| [MR-002](#mr-002-gerar-convite) | Campanha | MVP (pré-requisito) |
| [MR-005](#mr-005-criar-npcs) | Personagem | MVP (pré-requisito) |
| [MR-007](#mr-007-importar-ficha-em-pdf) | Personagem | Depois |
| [MR-010](#mr-010-desenhar-masmorras) | Masmorra | Depois |
| [MR-017](#mr-017-subir-de-nível) | Progressão | Depois |
| [MR-020](#mr-020-consultar-o-livro-de-regras) | Apoio | Depois |
| [MR-021](#mr-021-copiar-personagem) | Personagem | Depois |
| [MR-022](#mr-022-reutilizar-npcs) | Personagem | Depois |
| [MR-023](#mr-023-passar-ou-dividir-a-campanha) | Campanha | Depois |
| [MR-025](#mr-025-cadastrar-conteúdo-da-mesa) | Regras | A definir |
| [MR-026](#mr-026-propor-uma-raça-ou-classe-nova) | Regras | A definir |
| [MR-027](#mr-027-ler-as-regras-de-um-pdf) | Regras | A definir |

## Prioridade: MVP

### MR-001: Criar campanha

**Como** mestre, **quero** criar uma campanha que reúna as sessões, os personagens e os mapas, **para** organizar cada mesa separadamente.

- Prioridade: MVP
- Regras: RN-05
- Módulos: campaigns

#### Critérios de aceite
- **Dado** que estou logado, **quando** crio a campanha "Mirathel", **então** viro mestre dela **e** só os membros a veem na lista.

#### Implementado
- Backend pronto em 29/09/2026 (módulo `campaigns`, ver [Arquitetura](../arquitetura.md#módulo-campaigns-e-autorização)). Teste: `TestMR001_CreatorBecomesMasterAndOnlyMembersSeeTheCampaign`.
- Tela pronta em 29/09/2026: `/campanhas` (`web/src/app/pages/campaigns/`), lista com o papel de cada campanha (Mestre ou Jogador) e o formulário "Criar campanha"; no visual novo de 29/09/2026 ([Design](../design.md)), cada campanha é uma linha, e a lista vazia explica como criar uma ou entrar por convite. Teste Playwright: `o mestre cria uma campanha pela tela e a vê como mestre na lista` (`@MR-001`, `e2e/tests/campaigns.spec.ts`).

### MR-003: Entrar pelo convite

**Como** jogador, **quero** criar meu personagem pelo link do convite, **para** ele já entrar vinculado à campanha.

- Prioridade: MVP
- Regras: RN-03
- Módulos: characters, campaigns

#### Critérios de aceite
- **Dado** um convite válido para "Mirathel", **quando** o jogador abre o link e faz login com o Google, **então** vira jogador da campanha e cria um personagem do tipo jogador, que o mestre já vê na campanha.
- **Dado** um convite expirado, **quando** alguém abre o link, **então** vê uma mensagem clara **e** nada é criado.

#### Implementado
- Backend da entrada na campanha pronto em 29/09/2026: com login, o convite vira participação como jogador; aceitar de novo não muda nada; dois jogadores disputando o último uso não entram os dois. Testes: `TestMR003_ValidInviteMakesTheUserAPlayerTheMasterSees` (primeiro critério, sem a parte do personagem) e `TestMR003_ExpiredInviteGivesAClearErrorAndCreatesNothing`.
- Backend de "abre o link e faz login" pronto em 29/09/2026: quem não está logado entra e aceita o convite num passo só (`POST /auth/login` com a intenção `campaign_invite`), sem guardar nada no navegador, e cai em `/campanhas/<id>` ou em `/convite/erro?motivo=<código>`. Testes em Go, com provedor falso e CockroachDB: `TestSignInWithAnInviteJoinsTheCampaign`, `TestSignInWithAnUnusableInvite` e `TestSignInWithAnInviteAsAMember`.
- Tela pronta em 29/09/2026: `/convite` (`web/src/app/pages/invite/invite-accept.ts`), que lê o token do fragmento da URL, o apaga da URL na hora (`history.replaceState`) e aceita o convite (logado) ou oferece "Entrar para aceitar o convite" (deslogado). Teste Playwright, com um segundo usuário já logado: `jogador já logado abre o link do convite, entra na campanha e o mestre o vê nos membros` (`@MR-003`, `e2e/tests/invite.spec.ts`). O caminho de quem abre o link deslogado depende do contrato de login com convite (`intent=campaign_invite`), ainda não integrado nesta branch: o teste `visitante sem sessão entra pelo convite, faz login e é adicionado à campanha automaticamente` está escrito como `test.fixme` no mesmo arquivo, para ligar quando o backend chegar. E o caminho sem sessão, pelo login: `visitante sem sessão entra pelo convite, faz login e é adicionado à campanha automaticamente` (`@MR-003`), que também confere que o token não aparece em nenhuma URL pedida pelo navegador.
- Backend do personagem pronto em 29/09/2026 (Etapa 4): o jogador cria o próprio personagem, do tipo jogador, com `CharacterService.CreateCharacter`, e o mestre o vê na lista da campanha, com o nome de exibição do jogador, a classe e a raça (`ListCharacters`). O personagem nasce como rascunho. Teste: `TestMR003_PlayerCreatesTheirCharacterAndTheMasterSeesIt`. A tela de criar o personagem (o editor em passos) veio no PR das telas da Etapa 4, com o teste Playwright "o jogador entra pelo convite, cria o personagem e o mestre o vê na campanha".

#### Relacionadas
- RN-03: respondida em 29/09/2026 — o jogador só cria um personagem novo nesta campanha quando o atual morre; o personagem morto fica no sistema (ver [Regras de negócio](regras.md)).
- RN-17 decide o login do jogador sem Google (handle por mesa): ele entra sem senha e, quando a primeira sessão de 30 dias vence, precisa definir uma senha ou vincular o Google (ADR-0009, opção 3; ver [Perguntas em aberto](perguntas-em-aberto.md)).
- Quando o convite exige aprovação (RN-15), o jogador entra como membro pendente, vai direto criar o personagem, e o personagem nasce pendente até o mestre aprovar ou recusar. Implementado em 29/09/2026; ver [MR-024](#mr-024-aprovar-o-personagem-do-convite).

### MR-004: Ficha no formato do PDF

**Como** jogador, **quero** ver minha ficha num formato parecido com o PDF oficial, **para** achar tudo onde estou acostumado.

- Prioridade: MVP
- Regras: —
- Módulos: characters, rules

#### Critérios de aceite
- **Dado** um personagem completo, **quando** o jogador abre a ficha no celular, **então** vê as seções da ficha oficial (atributos, perícias, combate, magias, equipamento) **e** os valores calculados, como modificadores e CD de magia, vêm prontos do servidor.

#### Implementado
- Backend pronto em 29/09/2026 (Etapa 4): `CharacterService.GetCharacter` devolve a ficha como o jogador a preencheu e, junto, os valores calculados pelo servidor (`DerivedSheet`): atributos e modificadores, testes de resistência, perícias, passivas, CA, PV, deslocamento, sentidos, CD e ataque de magia, espaços, magias, ataques, características e as pendências da ficha. Teste: `TestMR004_SheetComesWithServerCalculatedValues`, com o Pensantus (INT 18, +4; CD 14; ataque de magia +6; CA 13; PV 23). A tela da ficha veio no PR das telas da Etapa 4, com o teste Playwright "o jogador abre a ficha no celular e vê as seções da ficha oficial com os valores calculados pelo servidor".

#### Relacionadas
- As regras como dados, com as fórmulas no Expr, calculam a ficha. O Samuel aceitou esse desenho em 29/09/2026. Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-006: Ficha travada

**Como** mestre, **quero** que a ficha do jogador fique só para visualização a partir da primeira sessão, **para** só eu e o sistema alterarmos.

- Prioridade: MVP
- Regras: RN-01
- Módulos: characters

#### Critérios de aceite
- **Dado** que a primeira sessão da campanha já começou, **quando** o jogador tenta editar os atributos da própria ficha, **então** o servidor recusa **e** o mestre consegue editar a mesma ficha.
- **Dado** que nenhuma sessão começou, **quando** o jogador edita a ficha, **então** a alteração é salva.
- **Dado** um personagem criado depois da primeira sessão, **quando** o jogador edita a ficha antes da próxima sessão, **então** a alteração é salva **e**, quando a próxima sessão começa, a ficha trava.
- **Dado** que a ficha travou, **quando** o jogador tenta editar a história do personagem, **então** o servidor recusa; **depois que** o mestre libera a história desse personagem, o jogador edita e salva, **e** a liberação acaba quando a próxima sessão começa.

Os dois últimos critérios foram respondida pelo Vinicius em 29/09/2026.

#### Implementado
- Backend pronto em 29/09/2026 (Etapa 4). A sessão começa por `PlayService.StartGameSession`, que trava as fichas na mesma transação; o jogador recebe `failed_precondition` com o motivo (`SHEET_LOCKED`, `STORY_LOCKED`), e o mestre libera a história com `SetStoryEditing`. Testes, um por critério:
  - `TestMR006_AfterTheFirstSessionOnlyTheMasterEditsTheSheet`;
  - `TestMR006_BeforeAnySessionThePlayerEditsTheSheet`;
  - `TestMR006_CharacterCreatedAfterTheFirstSessionLocksAtTheNextOne`, e no `play`, com a sessão de verdade, `TestCharacterCreatedLaterLocksAtTheNextSession`;
  - `TestMR006_AfterTheLockTheStoryNeedsTheMastersPermission`, e no `play`, `TestStartingASessionTurnsStoryEditingOff`.
- A tela (ficha só para leitura, botões do mestre) veio no PR das telas da Etapa 4, com os testes Playwright de `e2e/tests/sheet-lock.spec.ts`.

### MR-008: Pontos de interesse

**Como** mestre, **quero** criar pontos de interesse que abrem uma batalha, um submapa ou uma cena de RP.

- Prioridade: MVP
- Regras: —
- Módulos: maps

#### Critérios de aceite
- **Dado** um mapa da campanha, **quando** o mestre cria um ponto do tipo batalha, submapa ou cena de RP, **então** o ponto aparece no mapa **e** abrir o ponto leva ao encontro, ao submapa ou à cena.

#### Implementado
- Na Etapa 5: o `MapService` cria, muda, move e apaga mapas e pontos dos três tipos (ver [Arquitetura](../arquitetura.md#módulo-maps-mapas-pontos-e-tokens)). O mapa nasce de uma imagem da galeria e nasce escondido; o ponto também. O ponto de submapa leva a outro mapa da mesma campanha, nunca ao próprio mapa. Teste: `TestMR008_MasterCreatesPointsOfEachKind`.
- As telas vieram em seguida: "Novo mapa" (nome e uma imagem da galeria), o painel "Mapas" da campanha e o editor do mestre no computador (escolher o tipo e clicar no mapa põe o ponto, escondido; arrastar ou as setas movem; o painel do ponto salva tudo junto em "Salvar ponto"; "Adicionar token"). No celular o mestre só anda e dá zoom e revela pelas listas. Abrir o ponto de submapa é a ficha do ponto com "Abrir <mapa>". Teste: `maps.spec.ts` (`@MR-008`: o mestre cria o mapa e os três pontos pela tela, e o jogador abre o submapa pela ficha do ponto) e `a11y.spec.ts`. Ver [Design](../design.md#mapas-e-imagem-mostrada).
- Abrir o ponto: o de submapa mostra o nome e a descrição, com "Abrir <mapa>" (decisão do desenho de 30/09/2026: primeiro a ficha do ponto, depois o submapa). Os de batalha e de cena, por enquanto, só mostram o nome e a descrição: passam a abrir o encontro com o combate (Etapa 6) e a cena de RP com as cenas (Etapa 7). É a proposta da pergunta 29 do documento de acompanhamento, esperando o Samuel. Até lá, abrir um ponto de batalha ou de cena só mostra a ficha dele.

### MR-009: Mapa sem spoiler

**Como** jogador, **quero** ver no mapa só os pontos que meu grupo já conhece, **para** não receber spoiler.

- Prioridade: MVP
- Regras: RN-10
- Módulos: maps

#### Critérios de aceite
- **Dado** um mapa com um ponto revelado e outro escondido, **quando** o jogador abre o mapa, **então** só o revelado aparece **e** a resposta do servidor não contém o escondido.

#### Implementado
- O servidor, na Etapa 5: o `MapService` decide no servidor o que cada um vê (RN-10). O jogador recebe só os pontos revelados, só os tokens visíveis, e só os mapas revelados ou o mapa atual da sessão; um mapa escondido é `not_found` para ele, igual a um mapa que não existe. Teste: `TestMR009_PlayersNeverReceiveHiddenPoints` lê a resposta do jogador como o JSON que o app recebe e confere que não há o ID, o nome nem a descrição do ponto escondido; e confere que uma mudança só em coisas escondidas chega pelo stream só ao mestre. `TestRN10_PlayersCannotOpenHiddenMaps` cobre os mapas e os submapas.
- A tela do jogador (`/campanhas/<id>/mapas/<mapa>`) desenha só o que chegou, com a trilha do submapa, "Mapas revelados" e "Pontos deste mapa"; um mapa escondido mostra "Mapa não encontrado". Teste: `maps.spec.ts` (`@MR-009`) abre o mapa com o jogador, lê a resposta de `GetMap` que a própria página recebeu e confere que ela não traz o ID nem o nome do ponto escondido.

### MR-011: Iniciar a sessão

**Como** mestre, **quero** iniciar a sessão, que os jogadores recebam uma notificação no app e ter um link da sessão para mandar a eles, **para** todos entrarem juntos.

- Prioridade: MVP
- Regras: RN-06, RN-07
- Módulos: play, campaigns, characters

#### Critérios de aceite
- **Dado** uma campanha com três jogadores, **quando** o mestre inicia a sessão, **então** quem está com o app aberto vê a notificação **e** o mestre pode copiar o link da sessão.
- **Dado** o link da sessão, **quando** alguém que não é membro abre o link, **então** vê "peça um convite ao mestre" **e** não entra.
- **Dado** que é a primeira sessão da campanha, **quando** o mestre inicia a sessão, **então** as fichas dos jogadores travam.

#### Implementado
- O terceiro critério já vale desde a Etapa 4: `PlayService.StartGameSession` abre a sessão e trava, na mesma transação, as fichas dos jogadores que ainda são rascunho. Teste: `TestRN01_StartingTheFirstSessionLocksPlayerSheetsOnly` (no `play`).
- O servidor dos dois primeiros critérios veio na Etapa 5: `PlayService.ListOpenGameSessions` diz ao app, a cada 30 segundos com a aba visível, quais sessões estão abertas nas campanhas da pessoa, para o aviso (`TestListOpenGameSessions`); a página da sessão (`/campanhas/<id>/sessao`) lê `GetLiveSession` e abre o stream `WatchGameSession`, que respondem `not_found` a quem não é membro e ao membro pendente ("Peça um convite ao mestre") e `failed_precondition` com `NO_OPEN_SESSION` sem sessão aberta (`TestWatchGameSessionRefuses`, `TestAuthorizationMatrix`). Copiar o link é só da tela.
- As telas vieram em seguida, na Etapa 5: o aviso "A sessão 4 de Mirathel começou." embaixo da barra do app, com "Entrar na sessão" (para quem joga na campanha, fora da página dela e das páginas de sessão; fechar vale só para a aba), o link "Ao vivo" na barra, a etiqueta "Sessão ao vivo" em "Minhas campanhas", o painel "Sessão" da campanha com "Entrar na sessão" e "Copiar link da sessão", e a página da sessão. Quem não é membro, ou é membro pendente, vê "Peça um convite ao mestre", sem o nome da campanha; um membro sem sessão aberta vê "Nenhuma sessão em andamento", e a página abre a sessão sozinha quando o mestre inicia. Testes: `live-session.spec.ts` (`@MR-011`), com o jogador com o app aberto enquanto o mestre inicia a sessão pela tela.

#### Relacionadas
- RN-06: respondida em 29/09/2026 — a notificação em tela, para quem está com o app aberto, basta no MVP; não há notificação push do navegador.
- RN-07: respondida em 29/09/2026 — padrão de 1 uso e 7 dias, o mestre escolhe de 1 a 20 usos e de 5 minutos a 30 dias, e pode revogar (ver [MR-002](#mr-002-gerar-convite)).

### MR-012: Acompanhar a sessão

**Como** jogador, **quero** acompanhar minha ficha e o mapa atual durante a sessão.

- Prioridade: MVP
- Regras: RN-02, RN-10, RN-11
- Módulos: play, maps

#### Critérios de aceite
- **Dado** uma sessão ativa, **quando** o mestre move um token ou o sistema aplica dano ao personagem, **então** o celular do jogador mostra a mudança sem recarregar a página **e** as notas do mestre nunca aparecem.

#### Implementado
- O servidor da metade da ficha veio na Etapa 5: o mestre corrige PV, PV temporários, espaços de magia e dados de vida durante a sessão (`AdjustCharacterVitals`, RN-02), e a mudança chega na hora, pelo stream `WatchGameSession`, ao mestre e ao dono do personagem, nunca a outro jogador (`TestRN02_MasterAdjustsVitalsDuringSession`, `TestPlayersSeeOnlyTheirOwnVitals`, `TestLiveStreamIsNotBuffered`). Nada da sessão ao vivo carrega as notas do mestre (`TestRN11_LiveSessionNeverCarriesMasterNotes`).
- As telas da metade da ficha também: o jogador vê os PV, os PV temporários, a CA, os dados de vida e os espaços de magia do próprio personagem, e o número muda na tela quando o mestre corrige, sem recarregar. O mestre vê o grupo e corrige em "Ajustar" (uma folha no celular, um diálogo no desktop), que não passa do máximo da ficha e, depois do fim da sessão, diz "A sessão acabou". Sem conexão, a página diz "Reconectando…" com a hora da última atualização, e os números continuam na tela. Testes: `live-session.spec.ts` (`@MR-012`, `@RN-02`) e `a11y.spec.ts`.
- O servidor da metade do mapa veio na Etapa 5, com os mapas: o mestre escolhe o mapa atual da sessão (`PlayService.SetCurrentMap`, que também o revela) e move os tokens (`MapService.PlaceMapToken`); o stream leva `current_map_changed`, `token_moved` e `map_changed`, e o jogador só ouve falar do que ele vê (RN-10). Teste: `TestMR012_TokenMovesReachPlayersLive` (o token visível chega ao jogador; o token escondido de um NPC, só ao mestre) e `TestSetCurrentMap`.
- As telas do mapa na sessão vieram com as telas dos mapas: o mapa atual aparece no lugar do aviso "O mestre ainda não escolheu um mapa.", para o jogador (uma prévia que abre o mapa inteiro) e para o mestre (o seletor "Mapa atual", tokens que se arrastam, "Pontos do mapa" e "Tokens no mapa" com "Revelar aos jogadores" e "Esconder"). A página lê o mapa de novo em `map_changed` (e, se o servidor responde `not_found`, o jogador perdeu a vista do mapa e volta ao aviso), troca o mapa em `current_map_changed` e move o token em `token_moved` sem ler nada. Teste: `maps.spec.ts` (`@MR-012`: o mestre escolhe o mapa e move um token pelo teclado; a página aberta do jogador mostra o mapa e a nova posição sem recarregar).
- Na Etapa 5, o que muda ao vivo na ficha é a correção do mestre; "o sistema aplica dano" vem com o combate (Etapa 6).

#### Relacionadas
- RN-02: respondida em 29/09/2026 — sim, o mestre pode corrigir PV e espaços de magia na mão durante a sessão; o mestre tem a palavra final (ver [Regras de negócio](regras.md)).

### MR-013: Ordem dos turnos

**Como** jogador, **quero** ver a ordem dos turnos, onde cada um está e quanto posso me mover, **para** planejar minha ação.

- Prioridade: MVP
- Regras: —
- Módulos: play, rules

#### Critérios de aceite
- **Dado** um combate com a iniciativa definida, **quando** o jogador abre a tela de combate, **então** vê a ordem dos turnos, onde está cada combatente visível e quanto ainda pode se mover neste turno.

#### Relacionadas
- O deslocamento disponível vem do motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-014: Sua vez

**Como** jogador, na minha vez, **quero** ver minhas ações, ações bônus e ataques possíveis.

- Prioridade: MVP
- Regras: RN-02
- Módulos: play, rules

#### Critérios de aceite
- **Dado** um combate, **quando** chega a vez do Pensantus, **então** o jogador vê ação, ação bônus, reação e movimento disponíveis **e** as magias sem espaço de magia aparecem desabilitadas.
- **Dado** que o Pensantus conjura Mísseis Mágicos (Magic Missile) com um espaço de 1º círculo, **quando** a ação é confirmada, **então** o sistema marca o espaço como usado.

#### Relacionadas
- RN-02: o mestre pode corrigir PV e espaços de magia na mão (ver [Regras de negócio](regras.md)).
- Quais ações, ações bônus, reações e recursos o sistema conhece vem do motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-015: Ações da cena de RP

**Como** jogador, **quero** ver numa lista simples as ações que o mestre escolheu para a cena,
**para** saber o que posso rolar e usar fora de combate.

- Prioridade: MVP
- Regras: —
- Módulos: play, rules

#### Critérios de aceite
- **Dado** uma cena de RP com as ações que o mestre escolheu para ela, **quando** o jogador abre a cena, **então** vê a lista de ações escolhidas pelo mestre, cada rolagem com o bônus do próprio personagem já calculado (ex.: Investigação) **e** as habilidades que só valem em combate não aparecem.
- **Dado** um combate (MR-013, MR-014), **quando** o jogador vê as ações possíveis, **então** quem decide essa lista é o sistema, pelas regras de D&D — nunca o mestre. A cena de RP é o único lugar em que o mestre escolhe a lista.

#### Relacionadas
- Respondida em 29/09/2026: na cena de RP, o mestre escolhe as ações possíveis da cena, e o jogador vê o que pode fazer com o próprio bônus; no combate, quem decide e mostra as ações é o sistema, pelas regras de D&D. Ver [Regras de negócio](regras.md) e [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).
- Quais habilidades aparecem na lista, e o bônus de cada uma, vêm do motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-016: Dar XP

**Como** mestre, **quero** dar XP ao grupo por inimigos derrotados, por ouro ou por marcos, conforme a campanha, ou quando eu quiser.

- Prioridade: MVP
- Regras: RN-09, RN-12
- Módulos: progression

#### Critérios de aceite
- **Dado** uma campanha no modo por inimigos e dois goblins derrotados (50 XP cada), **quando** o encontro termina, **então** os 100 XP são divididos entre os quatro personagens do grupo, 25 para cada.
- **Dado** uma campanha no modo por marcos, **quando** o mestre registra um marco, **então** todos os personagens do grupo ficam marcados para subir de nível **e** nenhum XP é contado.
- **Dado** os modos por inimigos ou por ouro, **quando** o mestre dá XP ao grupo por conta própria, **então** o XP entra nas fichas **e** o histórico da campanha mostra quem deu, quando e por quê.

#### Relacionadas
- RN-09: respondida em 29/09/2026 — no modo por ouro, 1 XP por 1 peça de ouro (PO), como nas edições antigas.
- O que cada personagem ganha ao subir de nível vem do motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-018: Documento de campanha

**Como** mestre, **quero** um documento de campanha com texto, imagens, links para mapas e fichas que abrem num modal.

- Prioridade: MVP
- Regras: —
- Módulos: campaigns (o documento); maps e characters (as imagens, os mapas e as fichas dos links)

Existia no app antigo (descontinuado). Confirmada no MVP pelo Samuel em 29/09/2026, na Etapa 5 do [roadmap](../roadmap.md), ao lado dos mapas.

#### Critérios de aceite (proposta)
Propostos por nós, com a resposta padrão da pergunta 27 do documento de acompanhamento (só o mestre vê o documento), esperando o Samuel:

- **Dado** o documento da campanha, **quando** o mestre escreve texto, põe uma imagem da galeria e um link para um mapa e para uma ficha, **então** o documento mostra a imagem **e** o link abre o mapa ou a ficha numa janela, sem sair do documento.
- **Dado** um jogador, **quando** pede o documento, **então** o servidor recusa (enquanto valer a resposta padrão: só o mestre vê).

#### Implementado
- Backend pronto em 30/09/2026 (Etapa 5, módulo `campaigns`); telas prontas em 02/10/2026 (`/campanhas/:id/documento`, o painel "Documento da campanha" na página da campanha, só para o mestre; ver [Design](../design.md#documento-da-campanha)). Ver [Arquitetura](../arquitetura.md#documento-da-campanha) e [Modelo de dados](../dados.md#esquema-implementado).
  - Contrato: `CampaignDocumentService`, com `GetCampaignDocument` e `UpdateCampaignDocument` (`campaign_document.proto`). Um documento por campanha, em Markdown, até 200 KiB; salvar confere a revisão lida (`aborted` se alguém salvou antes).
  - Banco: `campaign_documents` (`00027`).
  - Os links do próprio app: `[texto](mapa:<id>)`, `[texto](ficha:<id>)` e `![legenda](imagem:<id>)`. O servidor guarda o texto como veio e não abre os links; o app os resolve pelas chamadas de sempre, com a autorização de sempre.
- Testes, por critério:
  - Primeiro: no servidor, `TestMR018_MasterWritesTheCampaignDocument` (o documento começa vazio, o mestre salva, lê de volta com a revisão, a hora e quem editou, e salva de novo; os links e a imagem voltam byte a byte). A imagem aparecer e o link abrir a janela são da tela: `campaign-document.spec.ts` (`@MR-018`), que escreve o texto pela barra de ferramentas, salva, recarrega e abre as janelas do mapa e da ficha; o mesmo arquivo cobre o conflito entre duas abas, o aviso ao sair sem salvar e os alvos apagados.
  - Segundo: na tela, o jogador vê só "Só o mestre vê o documento da campanha" e a página da campanha não mostra o painel (Playwright); no servidor, `TestMR018_PlayersCannotReadTheDocument` (o jogador recebe `permission_denied`; quem não é membro e o membro pendente, `not_found`) e `TestCampaignDocumentAuthorizationMatrix`.
  - Do salvamento: `TestUpdateCampaignDocumentRefusesAStaleRevision`, `TestSavingTheSameDocumentTwiceIsNotAConflict`, `TestUpdateCampaignDocumentFirstSavesRace`, `TestUpdateCampaignDocumentValidates` e `TestCampaignDocumentGoesWithTheCampaign`.

### MR-019: Galeria de imagens

**Como** mestre, **quero** uma galeria de imagens **para** usar nos documentos e nos mapas.

- Prioridade: MVP
- Regras: RN-10
- Módulos: maps

Existia no app antigo (descontinuado). Confirmada no MVP pelo Samuel em 29/09/2026, na Etapa 5 do [roadmap](../roadmap.md), ao lado dos mapas.

#### Critérios de aceite
Propostos por nós, esperando o Samuel. Os limites também são proposta (pergunta 30 do documento de acompanhamento): JPEG, PNG ou WebP, até 10 MB por imagem, 300 imagens e 500 MB por campanha.

- **Dado** o mestre na galeria, **quando** envia uma imagem JPEG, PNG ou WebP de até 10 MB, **então** ela aparece na galeria **e** o arquivo guardado não tem os metadados (EXIF) do original.
- **Dado** um jogador, **quando** pede a galeria da campanha, **então** o servidor recusa.
- **Dado** uma imagem usada num mapa, **quando** o mestre tenta apagá-la, **então** o app diz em qual mapa ela está.

#### Implementado
- O servidor da galeria, na Etapa 5: o envio (`POST /uploads/images`), as imagens e as miniaturas (`GET /images/{id}` e `/images/{id}/thumb`) e o `GalleryService` (listar, renomear, apagar). O servidor aceita só JPEG, PNG e WebP, recusa imagem com pixels demais e grava a imagem codificada de novo, sem nenhum metadado (ver [Arquitetura](../arquitetura.md#módulo-maps-galeria-e-imagens)).
- As telas, com o desenho aprovado (E5-20, E5-21, E5-22 e o painel de E5-09; ver [Design](../design.md#galeria-e-imagens)):
  - `/campanhas/:id/galeria` (`web/src/app/pages/gallery/`), só do mestre: a cota, a área de envio (o botão "Enviar imagem" ou arrastar arquivos para a página, vários de uma vez, enviados um depois do outro, cada um com o próprio progresso e "Cancelar envio"), o lembrete de privacidade, a grade da mais nova para a mais antiga, renomear no lugar, apagar com confirmação no lugar e a janela da imagem, com anterior e próxima. O app confere o tipo e o tamanho antes de enviar, e cada recusa, do app ou do servidor, vira um aviso em português com o nome do arquivo. O jogador vê "Só o mestre vê a galeria da campanha."; quem não é membro, "Campanha não encontrada".
  - O painel "Galeria" da página da campanha, só para o mestre: as 5 imagens mais novas, a cota e "Abrir galeria".
  - O seletor de imagem da galeria (`web/src/app/shared/gallery-picker/`), para o formulário "Novo mapa", o editor do documento e "Mostrar imagem": escolhe uma imagem ou envia uma nova e já a escolhe.
- Testes:
  - No servidor: `TestMR019_MasterUploadsAnImageWithoutItsMetadata` (primeiro critério: envia um JPEG com EXIF e GPS e lê o arquivo guardado), `TestMR019_PlayersCannotListTheGallery` (segundo) e `TestMR019_AnImageAMapUsesCannotBeDeleted` (terceiro: `failed_precondition` com o detalhe `ImageInUse`, que diz em quais mapas a imagem está). `TestDeletingAnImageAMapUses` confere que os arquivos ficam e que, trocada a imagem dos mapas, ela pode sair.
  - Pela tela (`@MR-019`, `e2e/tests/gallery.spec.ts`): o mestre envia um JPEG com EXIF e GPS, a imagem aparece, e o arquivo baixado de `/images/<id>` não tem o bloco EXIF (primeiro critério); um texto com nome `.png` e um GIF mostram o erro em português e a galeria continua vazia; o jogador da campanha vê só o aviso, a página da campanha não mostra a Galeria para ele, e o servidor recusa a lista (segundo); renomear e apagar, com a confirmação no lugar; a janela da imagem, com as setas, e o foco voltando ao cartão. O `a11y.spec.ts` passa o axe na galeria vazia, com imagens, com um envio recusado, na confirmação de apagar, na janela da imagem e na página da campanha com o painel.
  - Terceiro critério, pela tela: quando o servidor recusa apagar, o cartão diz "Essa imagem está num mapa. Troque a imagem do mapa antes de apagá-la." (teste unitário). Nomear o mapa no cartão, com o detalhe `ImageInUse`, chega com as telas dos mapas.
- Com os mapas, o servidor diz em cada imagem os mapas que a usam (`GalleryImage.used_in_maps`; `TestListGalleryImagesShowsWhereEachIsUsed`); o cartão mostra "Usada em Mirathel e arredores" com as telas dos mapas.

### MR-024: Aprovar o personagem do convite

**Como** mestre, **quero** aprovar ou recusar o personagem que um jogador criou pelo convite, **para** manter na campanha só os personagens que fazem sentido para a mesa.

- Prioridade: MVP
- Regras: RN-15
- Módulos: campaigns, characters

#### Critérios de aceite
Propostos por nós, a partir das decisões padrão do integrador (o convite escolhe se exige aprovação; a recusa apaga o personagem e a participação), esperando o Samuel:

- **Dado** que sou mestre de "Mirathel", **quando** gero um convite, **então** posso marcar "Exigir aprovação do mestre" **e**, sem marcar, o convite funciona como antes: quem aceita entra direto.
- **Dado** um convite que exige aprovação, **quando** o jogador o aceita (já logado, ou fazendo login pelo convite), **então** vai direto criar o personagem, que nasce "Pendente de aprovação" **e**, enquanto espera, ele só vê o nome da campanha e o próprio personagem, que continua editando.
- **Dado** um personagem pendente, **quando** o mestre abre a campanha, **então** o vê em "Esperando aprovação", abre a ficha e, **quando** aprova, o personagem vira rascunho **e** o jogador passa a ser jogador da campanha.
- **Dado** um personagem pendente, **quando** o mestre o recusa, **então** o personagem é apagado, o jogador não entra na campanha **e** precisa de um convite novo para tentar de novo.
- **Dado** que sou jogador, ou jogador pendente, **quando** tento aprovar ou recusar um personagem, **então** o servidor recusa.

#### Implementado
- Pronto em 29/09/2026 (Etapa 4), backend, tela e testes. Ver [RN-15](regras.md), [Arquitetura](../arquitetura.md#membro-pendente) e [Modelo de dados](../dados.md#esquema-implementado).
  - Contratos: `CreateInviteRequest.requires_approval` e `Invite.requires_approval`; `Campaign.awaiting_approval`; `CharacterService.ApproveCharacter` e `RejectCharacter`; `Character.can_approve`; os motivos `NOT_PENDING` e `AWAITING_APPROVAL` de `CharacterBlocked`.
  - Banco: `campaign_invites.requires_approval` (`00021`) e `campaign_members.status` (`00022`, `active` ou `pending`).
  - O membro pendente só passa pelas chamadas da lista `pendingMayCall` do `authz`; em todas as outras, recebe `not_found`.
  - Telas: a caixa "Exigir aprovação do mestre" no formulário de convite; o convite leva o membro pendente direto a "Criar personagem"; o aviso "Esperando a aprovação do mestre" na campanha e na ficha; a lista "Esperando aprovação" do mestre, na seção "Personagens"; os botões "Aprovar personagem" e "Recusar personagem" (com "Confirmar recusa") na ficha.
- Testes, por critério:
  - Primeiro: `TestMR024_MasterChoosesWhetherAnInviteRequiresApproval` e `TestRN15_InviteWithoutApprovalMakesAPlayerAtOnce` (`campaigns`); na tela, a caixa e a coluna "Aprovação" nos dois testes de `e2e/tests/character-approval.spec.ts`.
  - Segundo: `TestRN15_AcceptingAnInviteWithApprovalMakesAPendingMember` e `TestSignInWithAnApprovalInviteGoesToCreateTheCharacter` (`campaigns`), `TestMR024_PendingPlayerCreatesTheirCharacterAndWaits` (`characters`); na tela, `personagem criado por convite com aprovação fica pendente até o mestre aprovar` (`@MR-024`).
  - Terceiro: `TestMR024_MasterApprovesAndThePlayerJoins` (`characters`); na tela, o mesmo teste Playwright.
  - Quarto: `TestMR024_MasterRejectsAndThePlayerStaysOut` (`characters`); na tela, `o mestre recusa o personagem pendente e o jogador não entra` (`@MR-024`).
  - Quinto: `TestMR024_OnlyTheMasterApprovesOrRejects` e as colunas do membro pendente em `TestAuthorizationMatrix` (`campaigns` e `characters`).
  - Da regra: `TestRN15_PendingCharacterIsNotPartOfTheCampaignYet` (a sessão não trava o pendente, ele não morre, só o pendente é recusado), `TestRN15_ApproveAndRejectRace` (aprovar e recusar ao mesmo tempo), e no `authz`, `TestRN15_PendingMemberOnlyGetsThroughTheAllowedCalls`, `TestRN15_PendingMemberLooksLikeAStranger` e `TestPendingMayCallIsTheAgreedList`.

#### Relacionadas
- Estende [MR-003](#mr-003-entrar-pelo-convite): o personagem nasce pendente de aprovação (ver [Ciclo de vida da ficha](regras.md#ciclo-de-vida-da-ficha), RN-01).
- Estende [MR-002](#mr-002-gerar-convite): o mestre escolhe, em cada convite, se ele exige aprovação.

### MR-028: Mostrar uma imagem aos jogadores

**Como** mestre, **quero** mostrar aos jogadores uma imagem da galeria durante a sessão, com uma ação própria, separada do mapa atual, **para** apresentar um retrato, uma carta ou uma cena.

- Prioridade: MVP (pedido do Vinicius em 30/09/2026, na Etapa 5)
- Regras: RN-10
- Módulos: play, maps

#### Critérios de aceite
Propostos por nós, esperando o Samuel:

- **Dado** uma sessão aberta, **quando** o mestre mostra uma imagem da galeria, **então** quem está na sessão a vê na hora, sem recarregar, **e** o mapa atual continua lá; **quando** o mestre para de mostrar, a imagem some da tela dos jogadores.
- **Dado** que a sessão acabou ou o mestre parou de mostrar, **então** os jogadores não recebem mais o ID da imagem.

#### Implementado
- O servidor, na Etapa 5: `PlayService.SetShownImage` (só o mestre, só com a sessão aberta, só uma imagem da galeria da campanha) guarda a imagem em `game_sessions.shown_image_id`; `GetLiveSession` a devolve (`shown_image`, com o nome como legenda) e o stream leva `shown_image_changed` a todos. Uma imagem por vez, independente do mapa atual. Apagar a imagem da galeria para de mostrá-la. Uma sessão nova começa sem imagem. Ver [Arquitetura](../arquitetura.md#o-que-a-sessão-mostra).
- O jogador baixa a imagem (`GET /images/{id}`) só enquanto ela é mostrada, ou enquanto é o fundo de um mapa que ele vê; depois disso, `404`, mesmo com o ID guardado, e o navegador dele pergunta de novo a cada uso (`Cache-Control: private, no-cache`). É o "não" da pergunta 32, decidido pelo integrador em 30/09/2026 para todas as imagens (RN-10, ver [Servir as imagens](../arquitetura.md#servir-as-imagens)).
- A tela: o painel "Imagem para os jogadores" do mestre ("Mostrar imagem", o seletor da galeria com "Mostrar aos jogadores", "Trocar imagem" e "Parar de mostrar") e o bloco "O mestre está mostrando" do jogador, com a imagem, o nome como legenda e "Ver em tela cheia", que aparece e some ao vivo e é anunciado. Teste: `shown-image.spec.ts` (`@MR-028`: o mestre mostra, troca e para; a página aberta do jogador mostra e tira o bloco sem recarregar). Ver [Design](../design.md#mapas-e-imagem-mostrada).
- Testes do servidor: `TestMR028_MasterShowsAnImageToThePlayers` (mostrar, parar, apagar, e as recusas: jogador, imagem de outra campanha, sem sessão aberta), `TestRN10_PlayersOnlyFetchImagesTheyCanSee` (a imagem para de ser servida ao jogador quando para de ser mostrada) e as linhas de `SetShownImage` nas matrizes de autorização do `play`.

#### Dúvidas
- Pergunta 32 do documento de acompanhamento: o jogador continua com acesso à imagem depois que o mestre para de mostrar? Padrão, já implementado: não. O servidor para de mandar o ID, a tela tira a imagem e a rota responde `404` ao jogador. Se o Samuel quiser que o jogador guarde as imagens que viu (um "baú" de handouts), isso vira uma lista por jogador, numa história nova.
- O nome da imagem aparece para os jogadores como legenda. O nome vem do nome do arquivo enviado, então pode trazer uma anotação do mestre ("covil-secreto-do-lich"): o mestre pode renomear antes de mostrar.

## Prioridade: MVP (pré-requisito)

Pré-requisitos do MVP, decidido em 29/09/2026. Não têm implementação prévia para reaproveitar — o app antigo é descontinuado —, mas outras histórias do MVP dependem delas.

### MR-002: Gerar convite

**Como** mestre, **quero** gerar um link de convite, **para** os jogadores entrarem na campanha.

- Prioridade: MVP (pré-requisito)
- Regras: RN-07
- Módulos: campaigns

#### Critérios de aceite
Aceitos pelo Samuel em 29/09/2026, junto com o backend: o mestre adora poder escolher o número de usos e a validade do convite, e o comportamento implementado vira a RN-07.

- **Dado** que sou mestre de "Mirathel", **quando** gero um convite, **então** recebo um link que vale para uma pessoa por 7 dias **e** o servidor guarda só o hash do token.
- **Dado** um convite ainda não usado, **quando** o mestre o revoga, **então** o link para de funcionar **e** quem já entrou continua na campanha.
- **Dado** que sou jogador de "Mirathel", **quando** tento gerar, listar ou revogar convites, **então** o servidor recusa.

#### Implementado
- Backend pronto em 29/09/2026: o mestre escolhe de 1 a 20 usos (padrão 1) e de 5 minutos a 30 dias (padrão 7 dias), e pode revogar. Testes: `TestMR002_MasterGetsASingleUseSevenDayInviteStoredAsAHash`, `TestMR002_RevokedInviteStopsWorking`, `TestMR002_OnlyTheMasterManagesInvites`.
- Tela pronta em 29/09/2026: a seção "Convites" de `/campanhas/:id` (`web/src/app/pages/campaign-detail/invites/`), só para o mestre — cria convite (usos e validade com os presets 1/7/30 dias), mostra o link uma vez com aviso e botão de copiar, lista cada convite numa linha (usos, validade, se exige aprovação e o estado: Ativo, Usado, Expirado ou Revogado) e revoga. Teste Playwright: `o mestre gera um convite, vê o link uma vez e o revoga` (`@MR-002`, `e2e/tests/campaigns.spec.ts`).

#### Relacionadas
- RN-07: decidida em 29/09/2026 (ver [Regras de negócio](regras.md)).
- RN-15 (convite com aprovação) e [MR-024](#mr-024-aprovar-o-personagem-do-convite) estendem esta história: o mestre marca "Exigir aprovação do mestre" no convite, o jogador já cria o personagem pelo convite, e o mestre aprova. Implementado em 29/09/2026.

### MR-005: Criar NPCs

**Como** mestre, **quero** criar NPCs de cada tipo (inimigo, boss, minion, história), com ficha completa ou básica conforme o tipo.

- Prioridade: MVP (pré-requisito)
- Regras: RN-04
- Módulos: characters

#### Critérios de aceite
Propostos por nós e aceitos (respondida pelo Vinicius em 29/09/2026):

- **Dado** que sou mestre de "Mirathel", **quando** crio um inimigo ou um boss, **então** ele tem ficha completa; **quando** crio um minion ou um NPC de história, **então** ele tem ficha básica.
- **Dado** que sou jogador de "Mirathel", **quando** abro a campanha, **então** não vejo nenhum NPC **e** o servidor recusa se eu tentar criar um.
- **Dado** um NPC criado em "Mirathel", **quando** o mestre abre outra campanha, **então** o NPC não aparece lá: usar o mesmo NPC em outras campanhas é a [MR-022](#mr-022-reutilizar-npcs).

#### Implementado
- Backend pronto em 29/09/2026 (Etapa 4): o mestre cria NPCs com `CharacterService.CreateCharacter`; inimigo e boss levam a ficha completa (com os valores calculados), minion e NPC de história a ficha básica, e o servidor recusa a ficha do tipo errado. O NPC fica na campanha em que foi criado, e o dono é o mestre. Testes: `TestMR005_MasterCreatesNpcsOfEachKind` (primeiro critério) e `TestMR005_PlayersCannotSeeOrCreateNpcs` (segundo e terceiro). A tela veio no PR das telas da Etapa 4, com os testes Playwright "o mestre cria um inimigo com ficha completa e um minion com ficha básica" e "o jogador não vê os NPCs da campanha".

## Prioridade: Depois

Fora do MVP. Entram na Etapa 8 do [roadmap](../roadmap.md).

### MR-007: Importar ficha em PDF

**Como** jogador, **quero** importar minha ficha de um PDF escolhendo o formato (D&D Beyond ou ficha em português), **para** não digitar tudo de novo.

- Prioridade: Depois
- Regras: RN-08
- Módulos: characters

#### Relacionadas
- RN-08: respondida em 29/09/2026 — sem DOCX; só PDF editável, no formato do D&D Beyond ou da ficha em português.

### MR-010: Desenhar masmorras

**Como** mestre, **quero** desenhar masmorras com paredes (inclusive falsas), piso, água, portas, armadilhas e baús (normais ou mímicos), e colocar personagens da minha lista.

- Prioridade: Depois
- Regras: —
- Módulos: maps

### MR-017: Subir de nível

**Como** jogador, ao subir de nível, **quero** escolher o que ganho pela minha classe, subclasse ou uma segunda classe, seguindo as regras do D&D 5e.

- Prioridade: Depois
- Regras: RN-12
- Módulos: progression, rules

#### Relacionadas
- O motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026) é pré-requisito direto desta história. Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-020: Consultar o livro de regras

**Como** mestre, **quero** consultar o livro de regras com uma busca inteligente.

- Prioridade: Depois
- Regras: —
- Módulos: rules

### MR-021: Copiar personagem

**Como** jogador, **quero** copiar meu personagem para outra campanha, **para** jogar as duas ao mesmo tempo ou uma continuação.

- Prioridade: Depois
- Regras: RN-03
- Módulos: characters

#### Relacionadas
- RN-03: respondida em 29/09/2026 (ver [Regras de negócio](regras.md)).

### MR-022: Reutilizar NPCs

**Como** mestre, **quero** usar meus NPCs em várias campanhas, **para** não recriar o mesmo vilão.

- Prioridade: Depois
- Regras: RN-04
- Módulos: characters

### MR-023: Passar ou dividir a campanha

**Como** mestre, **quero** passar minha campanha para outro mestre, ou ter um segundo mestre nela, **para** a campanha continuar mesmo se eu sair.

- Prioridade: Depois
- Regras: RN-13
- Módulos: campaigns

#### Consequência
Hoje, excluir a conta de quem criou a campanha apaga a campanha inteira (ver [Privacidade](../privacidade.md#excluir-a-conta)). Com mais de um mestre, ou depois de uma passagem de campanha, isso muda: a campanha só é apagada quando o último mestre sai. Ver [ADR-0011](../adr/0011-autorizacao-papeis-por-campanha.md), como proposta.

## Prioridade: A definir

Novas, da ideia do Samuel de 29/09/2026 para o conteúdo que não vem pronto no SRD 5.1. A prioridade está perguntada no documento de acompanhamento.

### MR-025: Cadastrar conteúdo da mesa

**Como** mestre, **quero** cadastrar raças, classes, subclasses, antecedentes e regras que não vêm no SRD 5.1, **para** a campanha usar o material que a mesa joga.

- Prioridade: A definir
- Regras: —
- Módulos: rules, campaigns

#### Critérios de aceite (proposta)
- **Dado** que sou mestre de "Mirathel", **quando** cadastro uma classe nova com os dados, as perícias e as características dela, **então** a classe aparece no editor de personagem só em "Mirathel" **e** a ficha calcula os números com ela.
- **Dado** um conteúdo cadastrado em "Mirathel", **quando** abro outra campanha minha, **então** ele não aparece lá: o conteúdo vale por campanha (decidido em 29/09/2026).

### MR-026: Propor uma raça ou classe nova

**Como** jogador, **quero** propor uma raça ou uma classe que não existe no app ao criar o personagem, com o PDF ou o link das regras, **para** o mestre ler e decidir.

- Prioridade: A definir
- Regras: RN-15 (a mesma ideia de aprovação do convite)
- Módulos: rules, characters

#### Critérios de aceite (proposta)
- **Dado** que quero jogar de cozinheiro, uma classe feita por fãs, **quando** crio o personagem e proponho a classe com o link do PDF, **então** o mestre vê o pedido **e** o personagem fica esperando a decisão.
- **Dado** um pedido de classe nova, **quando** o mestre aprova e cadastra as regras dela (MR-025), **então** o personagem passa a usar a classe; **quando** o mestre recusa, **então** o jogador escolhe outra classe.

#### Exemplo do Samuel
O jogador quer jogar de cozinheiro, uma classe não oficial. Ele cadastra a classe ao criar o personagem e põe o link do PDF (ou o PDF) para o mestre ler, aprovar ou recusar, e cadastrar como funcionam as regras dela.

### MR-027: Ler as regras de um PDF

**Como** mestre, **quero** mandar o PDF com as regras e ver o app cadastrar sozinho as classes, raças e regras dele, **para** não digitar tudo.

- Prioridade: A definir
- Regras: —
- Módulos: rules

#### Dúvidas
- Ler um PDF de regras automaticamente precisa de um serviço de IA, que custa por uso e recebe o PDF. Um livro oficial tem direito autoral: o app não pode redistribuir o texto, e o resultado só pode aparecer para a mesa. Quando não der para ler o PDF, o cadastro fica com o mestre (MR-025). Perguntado no documento de acompanhamento.

## Ver também

- [Regras de negócio](regras.md)
- [Visão do produto](visao.md)
- [Perguntas em aberto](perguntas-em-aberto.md)
- [Roadmap](../roadmap.md)
