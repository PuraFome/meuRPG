# Histórias e critérios de aceite

O MVP tem 14 histórias, mais 2 pré-requisitos — 16 no total até o MVP: as 10 marcadas como MVP no plano, as duas que o Samuel acrescentou em 28/09/2026 (MR-015 e MR-016), as duas que o Samuel confirmou no MVP em 29/09/2026 (MR-018 e MR-019, documento de campanha e galeria de imagens), e duas marcadas "MVP (pré-requisito)" desde 29/09/2026: o convite (MR-002), que leva à MR-003, e os NPCs (MR-005), que são os inimigos do combate. "Já existe em parte" não é mais uma prioridade: o app antigo é descontinuado, então nenhuma história "já existe" no sistema novo — MR-002 e MR-005 entram como qualquer outra história do MVP, com os próprios testes.

Uma história está pronta quando todos os critérios dela passam. Cada critério vira um teste automático: Playwright para o que aparece na tela, teste em Go para a regra no servidor. Não há testes de caracterização do app antigo — o sistema novo só precisa provar os próprios critérios de aceite.

MR-021 e MR-022 são novas e saíram das respostas do Samuel de 28/09/2026. Ficaram como "Depois", mas o modelo de dados já nasce preparado para as duas (ver [Modelo de dados](../dados.md)). MR-023 e MR-024 são novas, das respostas do Samuel de 29/09/2026 (mais de um mestre, RN-13; convite com aprovação, RN-15); a prioridade das duas ainda está a definir.

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
| [MR-002](#mr-002-gerar-convite) | Campanha | MVP (pré-requisito) |
| [MR-005](#mr-005-criar-npcs) | Personagem | MVP (pré-requisito) |
| [MR-007](#mr-007-importar-ficha-em-pdf) | Personagem | Depois |
| [MR-010](#mr-010-desenhar-masmorras) | Masmorra | Depois |
| [MR-017](#mr-017-subir-de-nível) | Progressão | Depois |
| [MR-020](#mr-020-consultar-o-livro-de-regras) | Apoio | Depois |
| [MR-021](#mr-021-copiar-personagem) | Personagem | Depois |
| [MR-022](#mr-022-reutilizar-npcs) | Personagem | Depois |
| [MR-023](#mr-023-passar-ou-dividir-a-campanha) | Campanha | A definir |
| [MR-024](#mr-024-aprovar-o-personagem-do-convite) | Personagem | A definir |

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
- Tela pronta em 29/09/2026: `/campanhas` (`web/src/app/pages/campaigns/`), lista com o papel (mestre/jogador) e o formulário "Nova campanha". Teste Playwright: `o mestre cria uma campanha pela tela e a vê como mestre na lista` (`@MR-001`, `e2e/tests/campaigns.spec.ts`).

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
- Falta: criar o personagem do tipo jogador, que vem com o módulo `characters` (Etapa 4).

#### Relacionadas
- RN-03: respondida em 29/09/2026 — o jogador só cria um personagem novo nesta campanha quando o atual morre; o personagem morto fica no sistema (ver [Regras de negócio](regras.md)).
- RN-17 decide o login do jogador sem Google (handle por mesa); falta só saber se uma senha passa a ser exigida antes dos primeiros 30 dias (ver [Perguntas em aberto](perguntas-em-aberto.md)).
- Quando o convite exige aprovação (RN-15), o personagem criado aqui nasce pendente até o mestre aprovar ou recusar. Ver [MR-024](#mr-024-aprovar-o-personagem-do-convite).

### MR-004: Ficha no formato do PDF

**Como** jogador, **quero** ver minha ficha num formato parecido com o PDF oficial, **para** achar tudo onde estou acostumado.

- Prioridade: MVP
- Regras: —
- Módulos: characters, rules

#### Critérios de aceite
- **Dado** um personagem completo, **quando** o jogador abre a ficha no celular, **então** vê as seções da ficha oficial (atributos, perícias, combate, magias, equipamento) **e** os valores calculados, como modificadores e CD de magia, vêm prontos do servidor.

#### Dúvidas
- As regras de classe e raça ainda estão em discussão e afetam como a ficha é calculada. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

### MR-006: Ficha travada

**Como** mestre, **quero** que a ficha do jogador fique só para visualização a partir da primeira sessão, **para** só eu e o sistema alterarmos.

- Prioridade: MVP
- Regras: RN-01
- Módulos: characters

#### Critérios de aceite
- **Dado** que a primeira sessão da campanha já começou, **quando** o jogador tenta editar os atributos da própria ficha, **então** o servidor recusa **e** o mestre consegue editar a mesma ficha.
- **Dado** que nenhuma sessão começou, **quando** o jogador edita a ficha, **então** a alteração é salva.

### MR-008: Pontos de interesse

**Como** mestre, **quero** criar pontos de interesse que abrem uma batalha, um submapa ou uma cena de RP.

- Prioridade: MVP
- Regras: —
- Módulos: maps

#### Critérios de aceite
- **Dado** um mapa da campanha, **quando** o mestre cria um ponto do tipo batalha, submapa ou cena de RP, **então** o ponto aparece no mapa **e** abrir o ponto leva ao encontro, ao submapa ou à cena.

### MR-009: Mapa sem spoiler

**Como** jogador, **quero** ver no mapa só os pontos que meu grupo já conhece, **para** não receber spoiler.

- Prioridade: MVP
- Regras: RN-10
- Módulos: maps

#### Critérios de aceite
- **Dado** um mapa com um ponto revelado e outro escondido, **quando** o jogador abre o mapa, **então** só o revelado aparece **e** a resposta do servidor não contém o escondido.

### MR-011: Iniciar a sessão

**Como** mestre, **quero** iniciar a sessão, que os jogadores recebam uma notificação no app e ter um link da sessão para mandar a eles, **para** todos entrarem juntos.

- Prioridade: MVP
- Regras: RN-06, RN-07
- Módulos: play, campaigns, characters

#### Critérios de aceite
- **Dado** uma campanha com três jogadores, **quando** o mestre inicia a sessão, **então** quem está com o app aberto vê a notificação **e** o mestre pode copiar o link da sessão.
- **Dado** o link da sessão, **quando** alguém que não é membro abre o link, **então** vê "peça um convite ao mestre" **e** não entra.
- **Dado** que é a primeira sessão da campanha, **quando** o mestre inicia a sessão, **então** as fichas dos jogadores travam.

#### Relacionadas
- RN-06: respondida em 29/09/2026 — a notificação em tela, para quem está com o app aberto, basta no MVP; não há notificação push do navegador.
- RN-07: respondida em 29/09/2026 — padrão de 1 uso e 7 dias, o mestre escolhe de 1 a 20 usos e de 5 minutos a 30 dias, e pode revogar (ver [MR-002](#mr-002-gerar-convite)).

### MR-012: Acompanhar a sessão

**Como** jogador, **quero** acompanhar minha ficha e o mapa atual durante a sessão.

- Prioridade: MVP
- Regras: RN-02, RN-11
- Módulos: play

#### Critérios de aceite
- **Dado** uma sessão ativa, **quando** o mestre move um token ou o sistema aplica dano ao personagem, **então** o celular do jogador mostra a mudança sem recarregar a página **e** as notas do mestre nunca aparecem.

#### Relacionadas
- RN-02: respondida em 29/09/2026 — sim, o mestre pode corrigir PV e espaços de magia na mão durante a sessão; o mestre tem a palavra final (ver [Regras de negócio](regras.md)).

### MR-013: Ordem dos turnos

**Como** jogador, **quero** ver a ordem dos turnos, onde cada um está e quanto posso me mover, **para** planejar minha ação.

- Prioridade: MVP
- Regras: —
- Módulos: play, rules

#### Critérios de aceite
- **Dado** um combate com a iniciativa definida, **quando** o jogador abre a tela de combate, **então** vê a ordem dos turnos, onde está cada combatente visível e quanto ainda pode se mover neste turno.

#### Dúvidas
- As regras de classe e raça ainda estão em discussão e afetam o deslocamento disponível. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

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
- As regras de classe e raça ainda estão em discussão: quais ações, ações bônus, reações e recursos o sistema conhece depende dessa resposta. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

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
- As regras de classe e raça ainda estão em discussão e afetam quais habilidades aparecem na lista, e o cálculo do bônus de cada uma. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

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
- As regras de classe e raça ainda estão em discussão e afetam o que cada personagem ganha ao subir de nível. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

### MR-018: Documento de campanha

**Como** mestre, **quero** um documento de campanha com texto, imagens, links para mapas e fichas que abrem num modal.

- Prioridade: MVP
- Regras: —
- Módulos: campaigns

Existia no app antigo (descontinuado). Confirmada no MVP pelo Samuel em 29/09/2026, na Etapa 5 do [roadmap](../roadmap.md), ao lado dos mapas.

### MR-019: Galeria de imagens

**Como** mestre, **quero** uma galeria de imagens **para** usar nos documentos e nos mapas.

- Prioridade: MVP
- Regras: —
- Módulos: maps

Existia no app antigo (descontinuado). Confirmada no MVP pelo Samuel em 29/09/2026, na Etapa 5 do [roadmap](../roadmap.md), ao lado dos mapas.

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
- Tela pronta em 29/09/2026: a seção "Convites" de `/campanhas/:id` (`web/src/app/pages/campaign-detail/invites/`), só para o mestre — cria convite (usos e validade com os presets 1/7/30 dias), mostra o link uma vez com aviso e botão de copiar, lista com o status (ativo/usado/expirado/revogado) e revoga. Teste Playwright: `o mestre gera um convite, vê o link uma vez e o revoga` (`@MR-002`, `e2e/tests/campaigns.spec.ts`).

#### Relacionadas
- RN-07: decidida em 29/09/2026 (ver [Regras de negócio](regras.md)).
- RN-15 (convite com aprovação) e [MR-024](#mr-024-aprovar-o-personagem-do-convite) estendem esta história: o jogador já cria o personagem pelo convite, e o mestre aprova.

### MR-005: Criar NPCs

**Como** mestre, **quero** criar NPCs de cada tipo (inimigo, boss, minion, história), com ficha completa ou básica conforme o tipo.

- Prioridade: MVP (pré-requisito)
- Regras: RN-04
- Módulos: characters

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

#### Dúvidas
- As regras de classe e raça ainda estão em discussão e são pré-requisito direto desta história. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

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

## Prioridade: A definir

Novas, das respostas do Samuel de 29/09/2026. Ele não disse a prioridade destas duas; ficam "a definir" até a resposta.

### MR-023: Passar ou dividir a campanha

**Como** mestre, **quero** passar minha campanha para outro mestre, ou ter um segundo mestre nela, **para** a campanha continuar mesmo se eu sair.

- Prioridade: A definir
- Regras: RN-13
- Módulos: campaigns

#### Consequência
Hoje, excluir a conta de quem criou a campanha apaga a campanha inteira (ver [Privacidade](../privacidade.md#excluir-a-conta)). Com mais de um mestre, ou depois de uma passagem de campanha, isso muda: a campanha só é apagada quando o último mestre sai. Ver [ADR-0011](../adr/0011-autorizacao-papeis-por-campanha.md), como proposta.

### MR-024: Aprovar o personagem do convite

**Como** mestre, **quero** aprovar ou recusar o personagem que um jogador criou pelo convite, **para** manter na campanha só os personagens que fazem sentido para a mesa.

- Prioridade: A definir
- Regras: RN-15
- Módulos: campaigns, characters

#### Relacionadas
- Estende [MR-003](#mr-003-entrar-pelo-convite): o personagem nasce pendente de aprovação (ver [Ciclo de vida da ficha](regras.md#ciclo-de-vida-da-ficha), RN-01).

## Ver também

- [Regras de negócio](regras.md)
- [Visão do produto](visao.md)
- [Perguntas em aberto](perguntas-em-aberto.md)
- [Roadmap](../roadmap.md)
