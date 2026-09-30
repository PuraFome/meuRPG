# Histórias e critérios de aceite

O MVP tem 12 histórias: as 10 marcadas como MVP no plano e as duas que o Samuel acrescentou em 28/09/2026 (MR-015 e MR-016). Duas histórias que já existem em parte são pré-requisito: o convite (MR-002) leva à MR-003, e os NPCs (MR-005) são os inimigos do combate.

Uma história está pronta quando todos os critérios dela passam. Cada critério vira um teste automático: Playwright para o que aparece na tela, teste em Go para a regra no servidor.

MR-021 e MR-022 são novas e saíram das respostas do Samuel. Ficaram como "Depois", mas o modelo de dados já nasce preparado para as duas (ver [Modelo de dados](../dados.md)).

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
| [MR-002](#mr-002-gerar-convite) | Campanha | Já existe em parte |
| [MR-005](#mr-005-criar-npcs) | Personagem | Já existe em parte |
| [MR-018](#mr-018-documento-de-campanha) | Apoio | Já existe em parte |
| [MR-019](#mr-019-galeria-de-imagens) | Apoio | Já existe em parte |
| [MR-007](#mr-007-importar-ficha-em-pdf) | Personagem | Depois |
| [MR-010](#mr-010-desenhar-masmorras) | Masmorra | Depois |
| [MR-017](#mr-017-subir-de-nível) | Progressão | Depois |
| [MR-020](#mr-020-consultar-o-livro-de-regras) | Apoio | Depois |
| [MR-021](#mr-021-copiar-personagem) | Personagem | Depois |
| [MR-022](#mr-022-reutilizar-npcs) | Personagem | Depois |

## Prioridade: MVP

### MR-001: Criar campanha

**Como** mestre, **quero** criar uma campanha que reúna as sessões, os personagens e os mapas, **para** organizar cada mesa separadamente.

- Prioridade: MVP
- Regras: RN-05
- Módulos: campaigns

#### Critérios de aceite
- **Dado** que estou logado, **quando** crio a campanha "Mirathel", **então** viro mestre dela **e** só os membros a veem na lista.

#### Dúvidas
- RN-05: um mestre pode ser jogador em outra campanha? Ver [Perguntas em aberto](perguntas-em-aberto.md).

### MR-003: Entrar pelo convite

**Como** jogador, **quero** criar meu personagem pelo link do convite, **para** ele já entrar vinculado à campanha.

- Prioridade: MVP
- Regras: RN-03
- Módulos: characters, campaigns

#### Critérios de aceite
- **Dado** um convite válido para "Mirathel", **quando** o jogador abre o link e faz login com o Google, **então** vira jogador da campanha e cria um personagem do tipo jogador, que o mestre já vê na campanha.
- **Dado** um convite expirado, **quando** alguém abre o link, **então** vê uma mensagem clara **e** nada é criado.

#### Dúvidas
- RN-03: o jogador tem um personagem por campanha, ou pode ter outro, por exemplo quando o primeiro morre? Ver [Perguntas em aberto](perguntas-em-aberto.md).
- Login do jogador sem Google ainda está em discussão e pode mudar como esta história funciona. Ver [Perguntas em aberto](perguntas-em-aberto.md#login-do-jogador-sem-google).

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

#### Dúvidas
- RN-06: a notificação para quem está com o app aberto basta no MVP, ou precisa de notificação push com o app fechado? Ver [Perguntas em aberto](perguntas-em-aberto.md).
- RN-07: o convite serve para vários jogadores ou para um só? E vale por quanto tempo? Ver [Perguntas em aberto](perguntas-em-aberto.md).

### MR-012: Acompanhar a sessão

**Como** jogador, **quero** acompanhar minha ficha e o mapa atual durante a sessão.

- Prioridade: MVP
- Regras: RN-02, RN-11
- Módulos: play

#### Critérios de aceite
- **Dado** uma sessão ativa, **quando** o mestre move um token ou o sistema aplica dano ao personagem, **então** o celular do jogador mostra a mudança sem recarregar a página **e** as notas do mestre nunca aparecem.

#### Dúvidas
- RN-02: o mestre pode corrigir PV e espaços de magia na mão durante a sessão? O guia assume que sim. Ver [Perguntas em aberto](perguntas-em-aberto.md).

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

#### Dúvidas
- RN-02: ver [Perguntas em aberto](perguntas-em-aberto.md).
- As regras de classe e raça ainda estão em discussão: quais ações, ações bônus, reações e recursos o sistema conhece depende dessa resposta. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

### MR-015: Ações da cena de RP

**Como** jogador, **quero** ver numa lista simples as ações possíveis da cena,
**para** saber o que posso rolar e usar fora de combate.

- Prioridade: MVP
- Regras: —
- Módulos: play, rules

#### Critérios de aceite
- **Dado** uma cena de RP aberta pelo mestre, **quando** o jogador abre a cena, **então** vê uma lista simples de ações, cada rolagem com o bônus já calculado (ex.: Investigação) **e** as habilidades que só valem em combate não aparecem.

#### Dúvidas
- A lista de ações da cena sai sozinha da ficha, ou o mestre escolhe as ações de cada cena? Ver [Perguntas em aberto](perguntas-em-aberto.md).
- As regras de classe e raça ainda estão em discussão e afetam quais habilidades aparecem na lista. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

### MR-016: Dar XP

**Como** mestre, **quero** dar XP ao grupo por inimigos derrotados, por ouro ou por marcos, conforme a campanha, ou quando eu quiser.

- Prioridade: MVP
- Regras: RN-09, RN-12
- Módulos: progression

#### Critérios de aceite
- **Dado** uma campanha no modo por inimigos e dois goblins derrotados (50 XP cada), **quando** o encontro termina, **então** os 100 XP são divididos entre os quatro personagens do grupo, 25 para cada.
- **Dado** uma campanha no modo por marcos, **quando** o mestre registra um marco, **então** todos os personagens do grupo ficam marcados para subir de nível **e** nenhum XP é contado.
- **Dado** os modos por inimigos ou por ouro, **quando** o mestre dá XP ao grupo por conta própria, **então** o XP entra nas fichas **e** o histórico da campanha mostra quem deu, quando e por quê.

#### Dúvidas
- RN-09: no modo por ouro, quanto XP vale cada peça de ouro? Ver [Perguntas em aberto](perguntas-em-aberto.md).
- As regras de classe e raça ainda estão em discussão e afetam o que cada personagem ganha ao subir de nível. Ver [Perguntas em aberto](perguntas-em-aberto.md#regras-por-classe-e-raça).

## Prioridade: Já existe em parte

Pré-requisitos do MVP que já têm alguma implementação no app atual.

### MR-002: Gerar convite

**Como** mestre, **quero** gerar um link de convite, **para** os jogadores entrarem na campanha.

- Prioridade: Já existe em parte
- Regras: RN-07
- Módulos: campaigns

#### Dúvidas
- RN-07: ver [Perguntas em aberto](perguntas-em-aberto.md).

### MR-005: Criar NPCs

**Como** mestre, **quero** criar NPCs de cada tipo (inimigo, boss, minion, história), com ficha completa ou básica conforme o tipo.

- Prioridade: Já existe em parte
- Regras: RN-04
- Módulos: characters

### MR-018: Documento de campanha

**Como** mestre, **quero** um documento de campanha com texto, imagens, links para mapas e fichas que abrem num modal.

- Prioridade: Já existe em parte
- Regras: —
- Módulos: campaigns

### MR-019: Galeria de imagens

**Como** mestre, **quero** uma galeria de imagens **para** usar nos documentos e nos mapas.

- Prioridade: Já existe em parte
- Regras: —
- Módulos: maps

## Prioridade: Depois

Fora do MVP. Entram na Etapa 8 do [roadmap](../roadmap.md).

### MR-007: Importar ficha em PDF

**Como** jogador, **quero** importar minha ficha de um PDF escolhendo o formato (D&D Beyond ou ficha em português), **para** não digitar tudo de novo.

- Prioridade: Depois
- Regras: RN-08
- Módulos: characters

#### Dúvidas
- RN-08: a importação de DOCX continua? De qual modelo? Ver [Perguntas em aberto](perguntas-em-aberto.md).

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

#### Dúvidas
- RN-03: ver [Perguntas em aberto](perguntas-em-aberto.md).

### MR-022: Reutilizar NPCs

**Como** mestre, **quero** usar meus NPCs em várias campanhas, **para** não recriar o mesmo vilão.

- Prioridade: Depois
- Regras: RN-04
- Módulos: characters

## Ver também

- [Regras de negócio](regras.md)
- [Visão do produto](visao.md)
- [Perguntas em aberto](perguntas-em-aberto.md)
- [Roadmap](../roadmap.md)
