# Histórias e critérios de aceite

O MVP tem 29 histórias, mais 2 pré-requisitos — 31 no total até o MVP. Em 03/10/2026, o Vinicius ampliou o MVP: entraram a MR-010 e a MR-025 e as novas MR-029 a MR-039 (Etapas 8 a 10, ver [Roadmap](../roadmap.md)), além de critérios novos na MR-013 e na MR-014. Das 18 de antes: as 10 marcadas como MVP no plano, as duas que o Samuel acrescentou em 28/09/2026 (MR-015 e MR-016), as três que o Samuel pôs no MVP em 29/09/2026 (MR-018 e MR-019, documento de campanha e galeria de imagens, e MR-024, aprovar o personagem do convite), a que o Vinicius pôs no MVP em 30/09/2026 (MR-028, mostrar uma imagem aos jogadores), e duas marcadas "MVP (pré-requisito)" desde 29/09/2026: o convite (MR-002), que leva à MR-003, e os NPCs (MR-005), que são os inimigos do combate. "Já existe em parte" não é mais uma prioridade: o app antigo é descontinuado, então nenhuma história "já existe" no sistema novo — MR-002 e MR-005 entram como qualquer outra história do MVP, com os próprios testes.

Uma história está pronta quando todos os critérios dela passam. Cada critério vira um teste automático: Playwright para o que aparece na tela, teste em Go para a regra no servidor. Não há testes de caracterização do app antigo — o sistema novo só precisa provar os próprios critérios de aceite.

As histórias MR-025, MR-026 e MR-027 (o conteúdo que a mesa cadastra, inclusive por PDF) ficaram como "Depois" em 02/10/2026, nesta ordem: MR-025, MR-026 e MR-027. Em 03/10/2026, a MR-025 foi para o MVP (Etapa 10); a MR-026 e a MR-027 ficam na Etapa 11.

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
| [MR-025](#mr-025-cadastrar-conteúdo-da-mesa) | Regras | MVP (Etapa 10) |
| [MR-010](#mr-010-gerar-masmorras) | Masmorra | MVP (Etapa 10) |
| [MR-029](#mr-029-ganchos-e-pistas-da-cena) | RP | MVP (Etapa 8) |
| [MR-030](#mr-030-anotações-do-jogador) | RP | MVP (Etapa 8) |
| [MR-031](#mr-031-npcs-na-cena) | RP | MVP (Etapa 8) |
| [MR-032](#mr-032-destaques-do-combate) | Combate | MVP (Etapa 8) |
| [MR-033](#mr-033-imprimir-o-mapa-com-a-grade) | Mapa | MVP (Etapa 8) |
| [MR-034](#mr-034-movimentos-especiais) | Combate | MVP (Etapa 9) |
| [MR-035](#mr-035-armadilhas) | Mapa | MVP (Etapa 9) |
| [MR-036](#mr-036-névoa-de-guerra-pela-visão) | Mapa | MVP (Etapa 9) |
| [MR-037](#mr-037-criaturas-do-personagem) | Combate | MVP (Etapa 9) |
| [MR-038](#mr-038-quebra-cabeças) | Sessão | MVP (Etapa 10) |
| [MR-039](#mr-039-imagens-geradas-para-masmorras-e-cenas) | Apoio | MVP (Etapa 10) |
| [MR-002](#mr-002-gerar-convite) | Campanha | MVP (pré-requisito) |
| [MR-005](#mr-005-criar-npcs) | Personagem | MVP (pré-requisito) |
| [MR-007](#mr-007-importar-ficha-em-pdf) | Personagem | Depois |
| [MR-017](#mr-017-subir-de-nível) | Progressão | Depois |
| [MR-020](#mr-020-consultar-o-livro-de-regras) | Apoio | Depois |
| [MR-021](#mr-021-copiar-personagem) | Personagem | Depois |
| [MR-022](#mr-022-reutilizar-npcs) | Personagem | Depois |
| [MR-023](#mr-023-passar-ou-dividir-a-campanha) | Campanha | Depois |
| [MR-026](#mr-026-propor-uma-raça-ou-classe-nova) | Regras | Depois |
| [MR-027](#mr-027-ler-as-regras-de-um-pdf) | Regras | Depois |

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
- O editor da ficha (02/10/2026) oferece "Nenhuma" na subclasse, diz em que nível a classe a escolhe ("O Bárbaro escolhe a subclasse no nível 3.") e troca a subclasse ao trocar de classe. Na etapa "Magias", lista só as magias até o maior círculo do nível atual, em ordem de círculo e depois de nome; uma magia já escolhida acima do limite continua na lista, marcada "acima do nível", para poder ser desmarcada, e classes que começam a conjurar depois (Paladino, Patrulheiro) mostram "O Paladino conjura magias a partir do nível 2." no nível 1. "Truques" só aparece para a classe que tem truques na lista dela (o Paladino e o Patrulheiro não têm), ou quando a ficha já tem um truque, para poder desmarcar. O maior círculo por nível vem do servidor (`ClassSpellcasting.max_spell_level_by_level`, em `ContentService.ListContent`). Testes: `TestMaxSpellLevelFromSlots`, `TestCatalogMaxSpellLevelByLevel` (`rules`), `TestCatalogToProtoMaxSpellLevel` (`characters`), os de `character-editor.spec.ts` ("the subclass" e "the spell lists by level") e o Playwright `o editor da ficha mostra só as magias do nível e deixa tirar a subclasse` (`@MR-004`, `e2e/tests/character-editor.spec.ts`).
- As rolagens da criação e o "?" das magias (02/10/2026, Etapa 6, a pedido do Samuel). Na etapa "Atributos", "Como definir os valores" tem três cartões: "Digitar" (os seis campos de sempre), "Rolar 4d6" e "Conjunto padrão". "Rolar 4d6" rola seis vezes 4d6 no navegador (`crypto.getRandomValues`, sem viés), risca o menor dado de cada uma, lista os resultados do maior para o menor e tem "Rolar de novo", que troca os seis e desfaz a colocação. O jogador coloca cada resultado num atributo: do tablet para cima, cada atributo vira um seletor (escolher um resultado que está em outro atributo troca os dois); no celular, toca-se no resultado ("Escolhido") e depois no atributo, com as palavras "Livre", "Escolhido" e "em Força". "Conjunto padrão" faz o mesmo com 15, 14, 13, 12, 10 e 8, sem dados. Enquanto sobrar resultado sem atributo, o editor não salva e diz "coloque cada resultado num atributo" (nunca vira seis 10 sem ninguém ver). Nada é guardado: a ficha só recebe o número colocado (pergunta 40). Em "Pontos de vida", "Rolado" mostra uma linha por nível a partir do 2º ("Nível 2 (1d12)"), "Rolar", "Rolar os níveis que faltam" (só os vazios; dá para digitar o que saiu no dado de mesa), a fórmula "1d12 (8) + 3 (Constituição) = 11 PV" com a Constituição final (valor, raça, sub-raça e bônus manual) e uma caixa com o total até agora e a faixa final; é uma prévia, o servidor recalcula o máximo ao salvar (`rules.Derive`). Ao lado de cada magia, um "?" de 44 px abre a descrição (diálogo no desktop, folha de baixo no celular): nome, círculo e escola, tempo de conjuração, alcance (em metros: 5 pés = 1,5 m), componentes e duração em português, as etiquetas Ritual e Concentração e o texto do SRD em inglês, marcado `lang="en"` ("Texto do SRD 5.1 (em inglês)", mais "Em círculos maiores" quando há), vindos de `ContentService.GetSpellDetails`, buscados na hora e guardados só enquanto a página está aberta. O que o formatador não sabe traduzir aparece como o texto cru do SRD, com a nota "(texto do SRD)". Testes: `dice.spec.ts`, `hit-points-preview.spec.ts`, `spell-details-format.spec.ts`, `ability-scores.spec.ts`, `hit-points-rolls.spec.ts`, `spell-details.spec.ts` e os de "rolls and the spell \"?\"" em `character-editor.spec.ts`; Playwright `@MR-004` em `e2e/tests/character-editor.spec.ts` ("rolar 4d6 dá seis resultados…", "o conjunto padrão pode ser colocado e trocado…", "os pontos de vida rolados preenchem todos os níveis que faltam", "o \"?\" ao lado da magia abre a descrição…") e `@a11y` em `a11y.spec.ts` ("as rolagens e a descrição da magia passam no axe…").

#### Decidido em 02/10/2026
- A pedido do Samuel (01/10), o editor da ficha rola os atributos e os PV (implementado em 02/10/2026, ver acima; perguntas 40 e 41). Para os atributos, o app oferece rolar 4d6 descartando o menor, seis vezes, com o jogador distribuindo os valores, e também o conjunto padrão (15, 14, 13, 12, 10, 8); quem preferir continua digitando. As rolagens rodam no navegador e não ficam registradas: a ficha continua editável até a primeira sessão, e o mestre revisa. Essas rolagens não entram na RN-18.

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
- Desde 03/10/2026, o endereço de edição de uma ficha travada (`/campanhas/:id/personagens/:characterId/editar`, digitado ou salvo) também mostra a trava antes de qualquer formulário: o editor lê `Character.can_edit` ao abrir e, sem permissão, mostra "Ficha travada" (ou "Personagem morto") com o motivo e "Voltar para a ficha", em vez de deixar o jogador preencher tudo e só descobrir ao salvar. O mesmo teste de `sheet-lock.spec.ts` confere.

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
- Renomear e apagar um mapa (03/10/2026, desenho E6-27): no cabeçalho do mapa, "Renomear" ao lado do nome e "Apagar mapa" na ponta direita, que pergunta ali mesmo o que vai junto (os pontos e os tokens; a imagem fica na galeria) e avisa quando é o mapa atual da sessão aberta. Apagado, o app volta para a campanha. Teste: `maps.spec.ts` (`@MR-008`: o mestre renomeia o mapa e o apaga depois de confirmar) e `a11y.spec.ts`.
- Abrir o ponto: o de submapa mostra o nome e a descrição, com "Abrir <mapa>" (decisão do desenho de 30/09/2026: primeiro a ficha do ponto, depois o submapa). Os de batalha e de cena, por enquanto, só mostram o nome e a descrição: passam a abrir o encontro com o combate (Etapa 6) e a cena de RP com as cenas (Etapa 7). Decidido em 02/10/2026 (pergunta 29 do documento de acompanhamento): o ponto de batalha e o de cena já existem antes dessas etapas, só com nome e descrição. Até lá, abrir um ponto de batalha ou de cena só mostra a ficha dele.

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
- Regras: RN-02, RN-10, RN-11, RN-20
- Módulos: play, maps

#### Critérios de aceite
- **Dado** uma sessão ativa, **quando** o mestre move um token ou o sistema aplica dano ao personagem, **então** o celular do jogador mostra a mudança sem recarregar a página **e** as notas do mestre nunca aparecem.

#### Implementado
- O servidor da metade da ficha veio na Etapa 5: o mestre corrige PV, PV temporários, espaços de magia e dados de vida durante a sessão (`AdjustCharacterVitals`, RN-02), e a mudança chega na hora, pelo stream `WatchGameSession`, ao mestre e ao dono do personagem, nunca a outro jogador (`TestRN02_MasterAdjustsVitalsDuringSession`, `TestPlayersSeeOnlyTheirOwnVitals`, `TestLiveStreamIsNotBuffered`). Nada da sessão ao vivo carrega as notas do mestre (`TestRN11_LiveSessionNeverCarriesMasterNotes`).
- As telas da metade da ficha também: o jogador vê os PV, os PV temporários, a CA, os dados de vida e os espaços de magia do próprio personagem, e o número muda na tela quando o mestre corrige, sem recarregar. O mestre vê o grupo e corrige em "Ajustar" (uma folha no celular, um diálogo no desktop), que não passa do máximo da ficha e, depois do fim da sessão, diz "A sessão acabou". Sem conexão, a página diz "Reconectando…" com a hora da última atualização, e os números continuam na tela. Testes: `live-session.spec.ts` (`@MR-012`, `@RN-02`) e `a11y.spec.ts`.
- O servidor da metade do mapa veio na Etapa 5, com os mapas: o mestre escolhe o mapa atual da sessão (`PlayService.SetCurrentMap`, que também o revela) e move os tokens (`MapService.PlaceMapToken`); o stream leva `current_map_changed`, `token_moved` e `map_changed`, e o jogador só ouve falar do que ele vê (RN-10). Teste: `TestMR012_TokenMovesReachPlayersLive` (o token visível chega ao jogador; o token escondido de um NPC, só ao mestre) e `TestSetCurrentMap`.
- As telas do mapa na sessão vieram com as telas dos mapas: o mapa atual aparece no lugar do aviso "O mestre ainda não escolheu um mapa.", para o jogador (uma prévia que abre o mapa inteiro) e para o mestre (o seletor "Mapa atual", tokens que se arrastam, "Pontos do mapa" e "Tokens no mapa" com "Revelar aos jogadores" e "Esconder"). A página lê o mapa de novo em `map_changed` (e, se o servidor responde `not_found`, o jogador perdeu a vista do mapa e volta ao aviso), troca o mapa em `current_map_changed` e move o token em `token_moved` sem ler nada. Teste: `maps.spec.ts` (`@MR-012`: o mestre escolhe o mapa e move um token pelo teclado; a página aberta do jogador mostra o mapa e a nova posição sem recarregar).
- Decidido em 02/10/2026: no combate, o jogador vê os inimigos por uma palavra (Ileso, Ferido, Muito ferido, Derrotado), não pelo PV (RN-20).
- Na 6.4b o "aplica dano" ganhou as magias, as curas e as reações: o dano de uma magia segue o caminho do dano de um ataque, uma cura é aplicada na hora (e levanta quem estava a 0 PV), o mestre aplica uma quantia diferente da rolada (`TestMasterAppliesADifferentAmount`), e o dano em quem está a 0 PV conta uma falha no teste contra a morte (RN-03).
- "O sistema aplica dano": feito no servidor na fatia 6.4a da Etapa 6 (as telas são a 6.5). Um ataque que acerta abre um dano pendente; no NPC o dano é aplicado na hora (PV temporários primeiro; a 0 PV ele fica derrotado e sai da ordem), e no personagem do jogador ele espera o mestre, que o aplica ("Aplicar 5 de dano") ou o descarta; ao aplicar, a mudança chega ao dono e ao mestre ao vivo (`vitals_changed`). O mestre também mexe nos PV de um NPC ("Dano/Cura"), desfaz a última ação e lê o registro do combate, que o jogador recebe só com o que vê (RN-20). Testes: `TestMR012_DamageToAnNPCIsAppliedAndDefeatsIt`, `TestRN02_DamageToAPlayerWaitsForTheMaster`, `TestCombatUndoRestoresTheLastAction`, `TestTimelineRound1And2Log`, `TestRN20_PlayersNeverReceiveCAOrHiddenLogEntries`, `TestCombatLogChangedPerAudience`. Ver [Arquitetura](../arquitetura.md#combate).

#### Relacionadas
- RN-02: respondida em 29/09/2026 — sim, o mestre pode corrigir PV e espaços de magia na mão durante a sessão; o mestre tem a palavra final (ver [Regras de negócio](regras.md)).

### MR-013: Ordem dos turnos

**Como** jogador, **quero** ver a ordem dos turnos, onde cada um está e quanto posso me mover, **para** planejar minha ação.

- Prioridade: MVP
- Regras: RN-19, RN-20, RN-21
- Módulos: play, rules

#### Critérios de aceite
- **Dado** um combate com a iniciativa definida, **quando** o jogador abre a tela de combate, **então** vê a ordem dos turnos, onde está cada combatente visível e quanto ainda pode se mover neste turno.
- **Dado** um combate com a grade, **quando** abro a economia do turno ou o mapa, **então** o movimento aparece também em quadrados ("7,5 m · 5 quadrados"). *(Etapa 8, 03/10/2026)*
- **Dado** que três goblins têm a mesma iniciativa, **quando** o mestre abre a ordem dos turnos, **então** eles aparecem marcados como um ataque conjunto, para jogá-los juntos. É um lembrete: o app não automatiza nada. *(Etapa 8, 03/10/2026)*

#### Implementado
- O servidor do combate está pronto em 02/10/2026 (fatia 6.3 da Etapa 6; as telas são a fatia 6.5, e os ataques, a 6.4): o `CombatService` cria o combate no mapa com grade (o mapa tem `grid_columns`, `MapService.SetMapGrid`), põe o grupo e as cópias dos NPCs, rola a iniciativa de cada NPC, recebe a do jogador (no app ou o d20 físico, RN-18), deixa o mestre ordenar os empates, começa o combate, passa a vez (a rodada sobe depois do último; o movimento, a ação e a reação voltam no começo da vez de cada um) e deixa andar na grade com o limite do movimento que sobra. O jogador nunca recebe um combatente escondido nem o número de um NPC (RN-20), e a vez de um escondido aparece como "Vez do mestre". O ponto de batalha do mapa pode apontar o mapa do combate. Ver [Arquitetura](../arquitetura.md#combate). Testes: `TestMR013_TurnOrderAndMovementLeft`, `TestRN19_EachNPCRollsItsOwnInitiative`, `TestRN20_PlayersNeverReceiveHiddenCombatantsOrNPCNumbers`, `TestRN21_PlayerMovementIsLimitedTheMasterIsNot`, `TestEndTurnIsIdempotent`, `TestStartEncounterNeedsAGrid`, `TestMR013_CombatAuthorizationMatrix`, `TestMR013_CombatEventsPerAudience`, `TestMR013_CombatantsStartOnTheirTokensAndEndWhereTheyStand`. As telas da fatia 6.5a, em 02/10/2026 (sem as ações: ataques, dano, magias e o registro vêm na 6.4 e na 6.5b): na página da sessão, o mestre vê "Combate" com "Iniciar combate" (E6-01: nome, o mapa com a grade, o grupo e como cada jogador rola, os NPCs com quantas cópias e "Escondido no início"); um mapa sem grade manda para "Grade do mapa" (E6-02, `/campanhas/:id/mapas/:mapId/grade`, de 5 a 60 quadrados na largura, as linhas pela proporção da imagem); a iniciativa do mestre (E6-04: totais com a conta `1d20 (15) + 4 = 19`, o empate uma vez só com as setas dentro do grupo, "Esperando …" com "Digitar pelo jogador", "Começar o combate" bloqueado com o motivo, "Posições iniciais") e a iniciativa do jogador (E6-03: "Rolar no app" ou "Digitar o resultado", conforme RN-18, o total em 92 px e "Esperando o mestre começar o combate"); com o combate andando, o mestre vê a barra ("Vez do …", "Rodada 2", "Próximo turno", "Encerrar combate" que pergunta na própria barra, E6-11 e E6-12), o mapa com a grade e todos os tokens, e a ordem com PV, "Dano/Cura" dos jogadores, revelar e esconder, remover e "Adicionar combatente"; o jogador vê "Vez do …" (ou "Vez do mestre" quando é um escondido), "Você é o próximo", a ordem em fichas só com os visíveis e as palavras de estado (E6-05), "Sua vez" com o movimento, "Mover" e "Encerrar turno" (E6-06, sem os grupos de ações), e a página "Mover" (E6-10: alcance, "Mover 3 m. 2 quadrados para a direita e 1 quadrado para baixo. Depois restam 4,5 m.", "Longe demais: faltam 1,5 m", "Ocupado"; no computador, arrastar o token dentro do alcance); no fim, "Combate encerrado" (E6-16) para os dois. A tela segue a sessão ao vivo: `encounter_changed` lê o combate de novo, `turn_changed` e `combatant_moved` entram no lugar. Testes: `combat.spec.ts` (`@MR-013`: a grade, o início com o grupo e três goblins escondidos, a iniciativa no app, o empate, "Próximo turno", "Vez do mestre", mover dentro do alcance e a recusa além dele, o fim) e `a11y.spec.ts` ("o combate passa no axe…"); `combat-grid.spec.ts`, `combat-view.spec.ts`, `combat-state.spec.ts`, `combat-errors.spec.ts`, `initiative-setup.spec.ts`, `turn-panel.spec.ts` e `order-list.spec.ts` no Angular. Ainda faltam, na fatia 6.5b e na 6.4, as ações da vez, o registro do combate e o "Dano/Cura" dos NPCs.

#### Relacionadas
- Decidido em 02/10/2026: cada NPC rola a própria iniciativa (RN-19); o jogador vê o estado dos inimigos por uma palavra, nunca o PV nem a CA (RN-20); cada quadrado da grade vale 1,5 m, inclusive na diagonal, e o app não deixa o jogador passar do movimento do turno (RN-21). Ver [Regras de negócio](regras.md).
- O deslocamento disponível vem do motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-014: Sua vez

**Como** jogador, na minha vez, **quero** ver minhas ações, ações bônus e ataques possíveis.

- Prioridade: MVP
- Regras: RN-02, RN-03, RN-18, RN-20, RN-22
- Módulos: play, rules

#### Critérios de aceite
- **Dado** um combate, **quando** chega a vez do Pensantus, **então** o jogador vê ação, ação bônus, reação e movimento disponíveis **e** as magias sem espaço de magia aparecem desabilitadas.
- **Dado** que o Pensantus conjura Mísseis Mágicos (Magic Missile) com um espaço de 1º círculo, **quando** a ação é confirmada, **então** o sistema marca o espaço como usado.
- **Dado** um personagem com magias, **quando** abre a lista de magias na vez dele, **então** as que ele pode conjurar agora vêm primeiro, depois as outras, cada grupo por círculo. *(Etapa 8, 03/10/2026)*
- **Dado** uma magia na lista durante a sessão, **quando** o jogador toca no "?", **então** vê a descrição completa, como no editor. *(Etapa 8, 03/10/2026)*
- **Dado** uma magia que lê pontos de vida (Sono, Dobre pelos Mortos, Palavra de Poder: Matar, Cura Completa), **quando** ela é conjurada, **então** o servidor a resolve com o PV real dos alvos **e** o jogador continua vendo só "Ileso", "Ferido" ou "Muito ferido" (RN-20). *(Etapa 8, 03/10/2026)*

#### Implementado
- Motor de regras pronto em 02/10/2026 (fatia 6.1 da Etapa 6; as telas e a sessão vêm nas próximas): `rules/combat.Options` calcula a economia (ação, ação bônus, reação, movimento com a Disparada), os ataques, as magias com os círculos possíveis e as ações padrão e das features, e marca cada opção desabilitada com um código de motivo (`NO_SLOT`, `ACTION_USED`, `NO_USES`...), ver [Arquitetura](../arquitetura.md#combate-e-detalhes-das-magias). Testes: `TestOptionsPensantus` (com 1 espaço de 1º círculo livre de 4 e 0 de 2 livres de 2, Teia e Passo Nebuloso ficam `NO_SLOT` com mínimo 2 e Mísseis Mágicos só aceita o 1º; Raio de Fogo +6 1d10), `TestOptionsToren` (Machado de batalha +5 1d8+3; Retomar o Fôlego é ação bônus, 1 uso por descanso curto), `TestOptionsWarlockPactMagic`, `TestOptionsMovement`, `TestSpendSlot`, `TestSpendResource` (`rules/combat`), `TestResourcesAndActions` e `TestAttackDice` (`rules`). Cada magia tem detalhes estruturados (`ContentService.GetSpellDetails`, `TestGetSpellDetails`, `TestSpellDetailsExamples`). Ainda falta a sessão guardar o turno e as telas.

- O servidor de "Sua vez" está pronto em 02/10/2026 (fatia 6.4a; as telas são a 6.5): `CombatService.GetTurnOptions` devolve a economia (com o movimento em pés), os ataques com os alvos à vista e a distância (RN-21), as magias e as ações padrão, e, fora da vez, tudo desabilitado com o motivo (`NOT_YOUR_TURN`). O ataque tem dois passos, `RollAttack` (gasta a ação; o d20 vai ao servidor, que compara com a CA e devolve só Acertou, Errou ou Crítico) e `RollDamage`; `TakeAction` faz as ações padrão que só gastam a economia (a Disparada dobra o movimento que sobra). Os testes `TestMR014_TurnOptionsFollowTheEconomy` e `TestRN18_PhysicalRollsAreTypedSums` cobrem a economia, o alcance, o dado físico e o crítico. 
- O resto da "Sua vez" está pronto no servidor em 03/10/2026 (fatia 6.4b; as telas são a 6.5c): `CastSpell` conjura e **gasta o espaço e a ação na hora** (critério de aceite 2: `TestMR014_CastingSpendsTheSlot`, com a retentativa que não gasta de novo, o `NO_SLOT` com o círculo mínimo e a ação bônus), resolve pelo que a magia é (ataque de magia, resistência rolada pelo servidor com uma rolagem de dano para a conjuração inteira, os dardos dos Mísseis Mágicos, cura, ou só registro) e põe a concentração. O `GetTurnOptions` ganhou os alvos de cada magia, com a distância e os dardos por círculo (`spell_targets`). O Escudo vira um aviso quando um golpe acerta o personagem (`reaction_prompts`, `UseReaction`, `DeclineReaction`), o ataque de oportunidade é o `RollAttack` com `as_reaction`, o Ataque Extra deixa mais de um ataque por ação e as ações das features (Retomar o Fôlego, Surto de Ação...) gastam o uso do recurso. Testes: `TestTimelineRound3MagicMissile`, `TestSaveSpellRollsOnceForTheCast`, `TestHealingSpellRevivesAndResetsDeathSaves`, `TestShieldTurnsAHitIntoAMiss`, `TestOpportunityAttackSpendsTheReaction`, `TestExtraAttackAllowsTwoAttacks`, `TestSecondWindAndActionSurge`. O "Caído", os testes contra a morte e a confirmação do mestre (RN-03: `TestRN03_DeathSavesAndTheMasterConfirms`) e as condições e a concentração com o lembrete (RN-22: `TestRN22_ConditionsAndTheConcentrationReminder`) também estão no servidor. **O que a tela da 6.5c mostra:** o painel de conjurar com os círculos, os alvos e os dardos, o aviso "Você foi atingido: usar Escudo?", o teste contra a morte no começo da vez, as condições e o lembrete de concentração.
- **Implementado** em 03/10/2026 (fatia 6.5c; as telas dos critérios 2 e 3 e de RN-03 e RN-22): "Conjurar" abre a folha do celular (diálogo no desktop) com o espaço de magia em rádios ("1 livre de 4"; o círculo sem espaço é tracejado, "Sem espaço livre"), os alvos de `spell_targets` (distância, "Longe demais", o limite de alvos) e os dardos dos Mísseis Mágicos em passos de 44 px com o contador "3 de 3 dardos distribuídos". O aviso "É o seu último espaço de 1º círculo" (e o do Escudo Arcano, quando é o último que ele teria) vem antes do botão cheio "Conjurar X", que **gasta o espaço e a ação na hora**; uma magia de ataque troca o botão pelo d20 em dois jeitos (RN-18), um por alvo. O resultado lista cada alvo (d20, "Falhou"/"Resistiu: metade", "Dardo 1: 1d4 (3) + 1 = 4"), o dano que falta rolar logo abaixo (uma rolagem para a conjuração toda quando é área, uma para cada alvo nos dardos), "Espaços de 1º círculo: 0 livres de 4", "Escudo Arcano indisponível", a concentração e "Sua ação foi usada"; uma magia sem efeito conhecido diz "A magia foi conjurada: o mestre resolve o efeito." Um truque de resistência (Chama Sagrada) é conjurado, não atacado. As habilidades de classe têm "Usar" (Retomar o Fôlego rola o d10 numa folha e mostra a cura; o Surto de Ação diz "Você tem outra ação"; `NO_USES` vira "Sem usos: volta num descanso curto"), e o Ataque Extra mostra "1 ataque restante" na Ação com os ataques ainda habilitados ("Ataques desta ação já usados" depois). O golpe que o Escudo Arcano pode parar abre sozinho o `alertdialog` "Você foi atingido", com quem atacou e com quê quando o jogador vê o atacante (foco em "Não usar", sem saída sem resposta), e o cartão do mestre troca ao vivo quando o jogador responde; o ataque de oportunidade é a ação de texto sob "Sua reação" (folha só com ataques corpo a corpo, gasta a reação). Quem está a 0 PV vê "Brisa está caída", as marcas e "Rolar teste contra a morte" (RN-03); o mestre confirma a morte no lugar, no topo do cartão ("Confirmar a morte" / "Ainda não"). As condições (15 do SRD; "Derrubado" é o `prone`, pergunta 43) e a concentração estão em "Condições…" no ⋮ da ordem, como etiquetas sob o nome, na faixa do jogador e como ponto no token; o jogador encerra a própria concentração. O mestre aplica **outro valor** ("Aplicar outro valor") e, quando o alvo concentra, lê "Teste de Constituição, CD 10". O registro ganhou as frases de magia, reação, teste contra a morte, morte confirmada e condições, cada uma como o servidor a manda a cada plateia. Testes: `combat.spec.ts` (`@MR-014`, `@RN-02`, `@RN-03`, `@RN-22`: Mísseis Mágicos com os dardos e o último espaço, a resistência em dois goblins, a cura, os testes contra a morte e a confirmação, o Escudo nas duas telas, as condições e a concentração, outro valor com o lembrete, o guerreiro com Ataque Extra, Retomar o Fôlego e Surto de Ação, o ataque de oportunidade), `a11y.spec.ts` ("conjurar e cair"), e os specs de `core/combat` (`cast-flow`, `death-saves`, `combat-log`, `combat-errors`).

- As telas de agir no combate estão prontas em 03/10/2026 (fatia 6.5b; conjurar, o Escudo, os testes contra a morte e as habilidades de classe são da 6.5c). Na vez do jogador, "O que você pode fazer" agrupa o que o servidor calcula (`GetTurnOptions`) em Ação (Ataques, Magias, Ações padrão), Ação bônus, Reação e Movimento, cada grupo com a palavra de estado ("Disponível", "Usada") e cada opção desabilitada com o motivo em palavras ("Ação já usada", "Sem espaço de 2º círculo ou maior"), sem tirar a opção do lugar nem do teclado; as ações padrão são uma grade de duas colunas com uma só linha de motivo. A partir de 1024 px a vez é uma faixa compacta com "Encerrar turno" à direita, e os quadros de economia ficam no painel; de 1280 px a ordem é uma coluna à esquerda. "Atacar com Raio de Fogo" abre uma folha no celular (um diálogo no desktop; ela rola por dentro, com os botões grudados embaixo) em três passos, Alvo, Rolar e Dano: o alvo é um grupo de rádio com o estado e a distância ("Longe demais: alcance de 36 m" desabilitado), o d20 e o dano rolam no app ou se digitam ("Digite o resultado do dado", de 1 a 20; o dano, de N a N × faces) conforme RN-18, e o resultado mostra `1d20 (13) + 6 = 19`, "Acertou", "Crítico" ou "Errou" e o alvo derrotado, nunca a CA. As ações padrão chamam `TakeAction` (a Disparada dobra o movimento). "Encerrar turno" é contornado enquanto a ação ou a ação bônus está livre e vira o botão cheio quando as duas acabam; com a ação livre pergunta no lugar do botão. O mestre tem o cartão "Ações do Capitão Goblin" (PV, CA, deslocamento, o ataque, o campo "Alvo", "Rolar ataque" no app ou digitado, "Acertou contra CA 18 do Toren", o dano e "Aplicar 5 de dano" / "Não aplicar", que pergunta "Descartar o dano de 5?"); no celular e no tablet o cartão é a vez inteira e termina em "Próximo turno", com "Encerrar combate" e "Desfazer última ação" no fim da página. Enquanto há dano sem aplicar, "Próximo turno" pergunta "Há dano sem aplicar. Passar o turno mesmo assim?". Na ordem, "CA n" (só o mestre) e "Dano/Cura" também nos NPCs (`AdjustCombatantHitPoints`). O registro do combate (`ListCombatLog`, lido de novo a cada `combat_log_changed` e a cada reconexão) mostra as rodadas, a mais nova primeiro, com o ícone de magia nos ataques de magia, e o mestre desfaz a última ação nomeando-a ("Desfazer o ataque do Capitão Goblin ao Toren (5 de dano)?"). A linha do cabeçalho diz "Em andamento desde 20:05" para todos. Dois campos só do mestre vieram junto: `Combatant.armor_class` e `AttackRoll.target_armor_class`. Testes: `combat.spec.ts` (`@MR-012`, `@MR-014`: o ataque com dados físicos, o ataque no app, o fim do turno com a Disparada, o ataque do mestre com aplicar, descartar e desfazer, "Dano/Cura" e o registro sem o goblin escondido), `a11y.spec.ts` ("agir no combate" no claro/desktop e no escuro/celular), os specs de `core/combat` (`combat-dice`, `combat-options`, `combat-log`, `combat-grid` com `tight`, `attack-flow`, `turn-options-state`), de componentes (`roll-picker`, `end-turn`, `next-turn`, `action-row`, `order-column`), `session-time.spec.ts` e `TestRN20_PlayersNeverReceiveCAOrHiddenLogEntries`.

#### Relacionadas
- RN-02: o mestre pode corrigir PV e espaços de magia na mão (ver [Regras de negócio](regras.md)).
- Dados (RN-18): o mestre escolhe como a campanha rola (cada jogador escolhe, todos no app ou todos com os próprios dados) e cada jogador guarda a sua preferência; já implementado, com as telas na página da campanha (`TestRN18_DiceSettings`, `TestEffectiveDiceMode`, `dice.spec.ts` `@RN-18`). Com o dado físico, o jogador digita a soma dos dados e o app soma o modificador (`dice.Physical`, `TestPhysical`); as rolagens do combate (o d20 do ataque e o dano) seguem essa regra desde a fatia 6.4a, e as da 6.4b (o d20 do ataque de magia, a cura, o d10 do Retomar o Fôlego e o teste contra a morte) também.
- Decidido em 02/10/2026: o jogador vê se acertou ou errou e o dano, não a CA nem a rolagem do NPC (RN-20; feito na 6.4a); feito na 6.4b: condições e concentração só são marcadas e lembradas, e o mestre decide os efeitos (RN-22); na terceira falha no teste contra a morte, o personagem só morre quando o mestre confirma (RN-03).
- Quais ações, ações bônus, reações e recursos o sistema conhece vem do motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-015: Ações da cena de RP

**Como** jogador, **quero** ver numa lista simples as ações que o mestre escolheu para a cena,
**para** saber o que posso rolar e usar fora de combate.

- Prioridade: MVP
- Regras: RN-10, RN-18, RN-20
- Módulos: maps, play, rules

#### Critérios de aceite
- **Dado** uma cena de RP com as ações que o mestre escolheu para ela, **quando** o jogador abre a cena, **então** vê a lista de ações escolhidas pelo mestre, cada rolagem com o bônus do próprio personagem já calculado (ex.: Investigação) **e** as habilidades que só valem em combate não aparecem.
- **Dado** um combate (MR-013, MR-014), **quando** o jogador vê as ações possíveis, **então** quem decide essa lista é o sistema, pelas regras de D&D — nunca o mestre. A cena de RP é o único lugar em que o mestre escolhe a lista.

#### Implementado
- O servidor (fatia 7.3; as telas vêm na 7.5). O mestre escolhe as ações no ponto de cena do mapa, uma por vez (`AddSceneAction`, `UpdateSceneAction`, `MoveSceneAction`, `RemoveSceneAction`): perícia, teste de atributo ou salvaguarda, com nome opcional (até 60 caracteres) e CD opcional (1 a 30), no máximo 20, e nada de combate (a chave é conferida no catálogo das regras). Ele abre a cena na sessão (`OpenScene`: qualquer ponto de cena abre, mesmo sem ações desde a Etapa 8 (pergunta 63, mudada), e pode estar escondido, e então continua escondido no mapa) e fecha sem perguntar (`CloseScene`); todos recebem `scene_changed`. O jogador lê a cena (`GetOpenScene`) com o nome e a descrição do ponto e as ações com o bônus do próprio personagem (a ficha, `rules.SceneOptions`) e a passiva de Percepção, Investigação e Intuição, **sem a CD**. Rolar (`RollSceneCheck`): o d20 do app ou o digitado (RN-18) mais o bônus; uma rolagem por personagem por ação enquanto a cena está aberta (fechar e abrir de novo zera); o mestre recebe `scene_check_rolled` e vê todas as rolagens, com o total e o "passou" quando há CD (o registro da cena, pergunta 54), e o jogador só as próprias, sem "passou". Os padrões das perguntas 51 a 55 valem até o Samuel responder (ver [RN-20](regras.md)). Testes: `TestMR015_PlayerSeesTheMastersActionsWithTheirBonus`, `TestMR015_NothingCombatOnlyInAScene`, `TestRN20_APlayerNeverGetsADCOrAnotherPlayersRoll`, `TestMR015_OneRollPerActionWhileTheSceneIsOpen`, `TestRN18_SceneRollsFollowTheDiceMode`, `TestMR015_AHiddenPointCanBeOpenedAndStaysHidden`, `TestMR015_OpeningAndClosingAScene`, `TestMR015_SceneActionRules`, `TestMR015_RollingNeedsALivingCharacter`, `TestSceneAuthorizationMatrix`. Ver [Arquitetura](../arquitetura.md#cenas-de-rp).

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

#### Implementado
- Servidor pronto em 03/10/2026 (Etapa 7, fatia 7.2; módulo `progression`, ver [Arquitetura](../arquitetura.md#módulo-progression-xp-e-marcos)): `AwardXP` (por inimigos, por ouro ou avulso), `MarkMilestone`, `UndoLastXPAward`, `ListXPAwards` e `GetCampaignExperience`; o XP de cada NPC na ficha (`challenge_rating`, `xp_value`) e no combatente (só o mestre vê); "Pode subir de nível" (`can_level_up`) em `GetCharacter` para o mestre e o dono (RN-12). As telas (o "Dar XP" no fim do combate, o marco, "Experiência" na campanha, a ficha) vêm na fatia 7.4. Testes: `TestMR016_EnemiesAwardSplitsTheDefeated` (dois goblins de 50 XP, quatro personagens, 25 para cada), `TestMR016_MilestoneMarksWithoutXP` (marca todos, nenhum XP, a marca some quando o nível sobe), `TestMR016_ManualAwardIsInTheHistory` (quem, quando, por quê, quanto), `TestGoldAwardGivesOneXPPerGoldPiece`, `TestRemainderIsLostAndReported`, `TestUndoTakesBackOnlyTheLastAward`, `TestSecondEnemiesAwardForTheSameEncounterIsRefused`, `TestModeMustFitTheCampaign`, `TestOnlyLivingPlayerCharactersGetXP`, `TestPlayersNeverWrite`, `TestAuthorizationMatrix` e `TestRN20_PlayersNeverGetAnNPCsXP`.

#### Relacionadas
- Perguntas 44 a 50 (o ND do NPC, quem divide, o arredondamento, o ouro, o marco, o aviso de subir de nível, quem vê o histórico): construído sobre a nossa resposta-padrão de cada uma, ainda sem resposta do Samuel; ver [RN-09](regras.md) e [RN-12](regras.md).
- RN-09: respondida em 29/09/2026 — no modo por ouro, 1 XP por 1 peça de ouro (PO), como nas edições antigas.
- O que cada personagem ganha ao subir de nível vem do motor de regras (regras como dados, aceitas pelo Samuel em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).

### MR-018: Documento de campanha

**Como** mestre, **quero** um documento de campanha com texto, imagens, links para mapas e fichas que abrem num modal.

- Prioridade: MVP
- Regras: —
- Módulos: campaigns (o documento); maps e characters (as imagens, os mapas e as fichas dos links)

Existia no app antigo (descontinuado). Confirmada no MVP pelo Samuel em 29/09/2026, na Etapa 5 do [roadmap](../roadmap.md), ao lado dos mapas.

#### Critérios de aceite
Aceitos em 02/10/2026 (pergunta 27 do documento de acompanhamento): no MVP, só o mestre vê o documento.

- **Dado** o documento da campanha, **quando** o mestre escreve texto, põe uma imagem da galeria e um link para um mapa e para uma ficha, **então** o documento mostra a imagem **e** o link abre o mapa ou a ficha numa janela, sem sair do documento.
- **Dado** um jogador, **quando** pede o documento, **então** o servidor recusa (só o mestre vê o documento).

#### Implementado
- Backend pronto em 30/09/2026 (Etapa 5, módulo `campaigns`); telas prontas em 02/10/2026 (`/campanhas/:id/documento`, o painel "Documento da campanha" na página da campanha, só para o mestre; ver [Design](../design.md#documento-da-campanha)). Ver [Arquitetura](../arquitetura.md#documento-da-campanha) e [Modelo de dados](../dados.md#esquema-implementado).
  - Contrato: `CampaignDocumentService`, com `GetCampaignDocument` e `UpdateCampaignDocument` (`campaign_document.proto`). Um documento por campanha, em Markdown, até 200 KiB; salvar confere a revisão lida (`aborted` se alguém salvou antes).
  - Banco: `campaign_documents` (`00027`).
  - Os links do próprio app: `[texto](mapa:<id>)`, `[texto](ficha:<id>)` e `![legenda](imagem:<id>)`. O servidor guarda o texto como veio e não abre os links; o app os resolve pelas chamadas de sempre, com a autorização de sempre.
- Testes, por critério:
  - Primeiro: no servidor, `TestMR018_MasterWritesTheCampaignDocument` (o documento começa vazio, o mestre salva, lê de volta com a revisão, a hora e quem editou, e salva de novo; os links e a imagem voltam byte a byte). A imagem aparecer e o link abrir a janela são da tela: `campaign-document.spec.ts` (`@MR-018`), que escreve o texto pela barra de ferramentas, salva, recarrega e abre as janelas do mapa e da ficha; o mesmo arquivo cobre o conflito entre duas abas, o aviso ao sair sem salvar e os alvos apagados.
  - Desde 03/10/2026, a janela do mapa segue o desenho E5-29 inteiro: o mapa com todos os pontos (os escondidos marcados "Escondido"), a legenda, "Mapa atual da Sessão N" quando a sessão aberta está nele e "Abrir no editor de mapas". O mesmo teste confere os pontos e a linha da sessão.
  - Segundo: na tela, o jogador vê só "Só o mestre vê o documento da campanha" e a página da campanha não mostra o painel (Playwright); no servidor, `TestMR018_PlayersCannotReadTheDocument` (o jogador recebe `permission_denied`; quem não é membro e o membro pendente, `not_found`) e `TestCampaignDocumentAuthorizationMatrix`.
  - Do salvamento: `TestUpdateCampaignDocumentRefusesAStaleRevision`, `TestSavingTheSameDocumentTwiceIsNotAConflict`, `TestUpdateCampaignDocumentFirstSavesRace`, `TestUpdateCampaignDocumentValidates` e `TestCampaignDocumentGoesWithTheCampaign`.

### MR-019: Galeria de imagens

**Como** mestre, **quero** uma galeria de imagens **para** usar nos documentos e nos mapas.

- Prioridade: MVP
- Regras: RN-10
- Módulos: maps

Existia no app antigo (descontinuado). Confirmada no MVP pelo Samuel em 29/09/2026, na Etapa 5 do [roadmap](../roadmap.md), ao lado dos mapas.

#### Critérios de aceite
Aceitos em 02/10/2026, com os limites (pergunta 30 do documento de acompanhamento): JPEG, PNG ou WebP, até 10 MB por imagem, 300 imagens e 500 MB por campanha.

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
Aceitos em 02/10/2026 (perguntas 23, 24 e 25 do documento de acompanhamento): o convite escolhe se exige aprovação, e a recusa apaga o personagem e a participação. Os dois últimos critérios abaixo (jogador pendente sem personagem e convite comum para quem está pendente) estão implementados no servidor desde 02/10/2026; a tela do mestre para o primeiro deles está implementada (Etapa 6):

- **Dado** que sou mestre de "Mirathel", **quando** gero um convite, **então** posso marcar "Exigir aprovação do mestre" **e**, sem marcar, o convite funciona como antes: quem aceita entra direto.
- **Dado** um convite que exige aprovação, **quando** o jogador o aceita (já logado, ou fazendo login pelo convite), **então** vai direto criar o personagem, que nasce "Pendente de aprovação" **e**, enquanto espera, ele só vê o nome da campanha e o próprio personagem, que continua editando.
- **Dado** um personagem pendente, **quando** o mestre abre a campanha, **então** o vê em "Esperando aprovação", abre a ficha e, **quando** aprova, o personagem vira rascunho **e** o jogador passa a ser jogador da campanha.
- **Dado** um personagem pendente, **quando** o mestre o recusa, **então** o personagem é apagado, o jogador não entra na campanha **e** precisa de um convite novo para tentar de novo.
- **Dado** que sou jogador, ou jogador pendente, **quando** tento aprovar ou recusar um personagem, **então** o servidor recusa.
- **Dado** um jogador pendente que ainda não criou o personagem, **quando** o mestre abre a campanha, **então** vê quem está pendente sem personagem, com um botão para remover; **e**, passados 30 dias sem personagem, a participação pendente é apagada sozinha. Servidor pronto: `ListPendingMembers`, `RemovePendingMember` e o prazo `campaign_members.pending_expires_at` com TTL (testes: `TestQ24_MasterSeesAndRemovesPendingMemberWithoutCharacter`, `TestQ24_OnlyTheCampaignsMasterManagesPendingMembers`, `TestQ24_PendingMembershipExpiresAfter30Days`, `TestQ24_CreatingTheCharacterClearsTheDeadline`). Tela: na página da campanha, em "Membros", cada pessoa sem personagem aparece com a etiqueta "Sem personagem", a data de entrada e a data em que sai sozinha, e o botão "Remover" abre a confirmação ali mesmo, com o foco em "Cancelar"; se a pessoa criou o personagem nesse meio-tempo, a lista é atualizada e a mensagem manda aprovar ou recusar em Personagens pendentes. Só o mestre vê. Teste: `o mestre vê quem entrou pelo convite e ainda não criou o personagem, e o remove` (`e2e/tests/character-approval.spec.ts`, `@MR-024`), mais o axe em `a11y.spec.ts`.
- **Dado** um jogador pendente de "Mirathel", **quando** ele aceita um convite comum (sem aprovação) da mesma campanha, **então** vira jogador na hora, porque o convite comum conta como a aprovação do mestre; se ele já tinha um personagem esperando, o personagem também é aprovado, e o convite gasta um uso. Um convite com aprovação, ou que não vale mais, não muda nada. Servidor pronto (testes: `TestQ25_PlainInvitePromotesPendingMember`, nos pacotes `campaigns` e `characters`, e `TestQ25_OnlyAWorkingPlainInvitePromotes`).

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
Aceitos em 02/10/2026 (pergunta 32 do documento de acompanhamento); o último critério foi desenhado e construído na Etapa 6:

- **Dado** uma sessão aberta, **quando** o mestre mostra uma imagem da galeria, **então** quem está na sessão a vê na hora, sem recarregar, **e** o mapa atual continua lá; **quando** o mestre para de mostrar, a imagem some da tela dos jogadores.
- **Dado** que a sessão acabou ou o mestre parou de mostrar, **então** os jogadores não recebem mais o ID da imagem.
- **Dado** uma imagem que o mestre quer que os jogadores continuem vendo, **quando** ele liga "Deixar com os jogadores" e depois para de mostrar ou troca a imagem, **então** ela continua com os jogadores, numa lista "Imagens que o mestre deixou" na página da sessão deles, até o mestre tirá-la ("Tirar"); a lista é da campanha e continua depois que a sessão acaba.

#### Implementado
- O servidor, na Etapa 5: `PlayService.SetShownImage` (só o mestre, só com a sessão aberta, só uma imagem da galeria da campanha) guarda a imagem em `game_sessions.shown_image_id`; `GetLiveSession` a devolve (`shown_image`, com o nome como legenda) e o stream leva `shown_image_changed` a todos. Uma imagem por vez, independente do mapa atual. Apagar a imagem da galeria para de mostrá-la. Uma sessão nova começa sem imagem. Ver [Arquitetura](../arquitetura.md#o-que-a-sessão-mostra).
- O jogador baixa a imagem (`GET /images/{id}`) só enquanto ela é mostrada, enquanto está deixada com os jogadores (abaixo) ou enquanto é o fundo de um mapa que ele vê; fora disso, `404`, mesmo com o ID guardado, e o navegador dele pergunta de novo a cada uso (`Cache-Control: private, no-cache`). É o padrão da pergunta 32, aceito em 02/10/2026 (RN-10, ver [Servir as imagens](../arquitetura.md#servir-as-imagens)).
- **Deixar com os jogadores (Etapa 6).** `SetShownImage` ganhou `keep` (o interruptor da imagem mostrada, em `game_sessions.shown_image_keep`; desligado ao mostrar uma imagem nova): ligado, parar de mostrar, trocar a imagem ou encerrar a sessão passa a imagem para a lista da campanha (`campaign_left_images`), na mesma transação. `ListLeftImages` (qualquer membro ativo, com ou sem sessão) lê a lista, `TakeBackLeftImage` (só o mestre) tira uma imagem dela, e o stream manda `left_images_changed`, uma dica sem conteúdo, a todos. Apagar a imagem da galeria a tira da lista. Testes do servidor: `TestMR028_MasterLeavesAnImageWithThePlayers` (o interruptor, parar, trocar, encerrar a sessão, tirar, apagar, os eventos e as recusas) e a parte nova de `TestRN10_PlayersOnlyFetchImagesTheyCanSee`; as linhas de `ListLeftImages` e `TakeBackLeftImage` nas matrizes de autorização do `play`.
- A tela da lista: o interruptor "Deixar com os jogadores" (com "Ligado" e "Desligado") e a lista "Deixadas com os jogadores", com "Tirar", no painel do mestre; "Imagens que o mestre deixou", com "Ver em tela cheia", na página da sessão do jogador, escondida enquanto vazia e sem aviso de leitor de tela quando muda. Teste: `shown-image.spec.ts` (`@MR-028`: o mestre mostra com o interruptor ligado e para; a lista do jogador mostra a imagem, abre em tela cheia, e some sem recarregar quando o mestre a tira, com `404` na URL). Ver [Design](../design.md#mapas-e-imagem-mostrada).
- A tela: o painel "Imagem para os jogadores" do mestre ("Mostrar imagem", o seletor da galeria com "Mostrar aos jogadores", "Trocar imagem" e "Parar de mostrar") e o bloco "O mestre está mostrando" do jogador, com a imagem, o nome como legenda e "Ver em tela cheia", que aparece e some ao vivo e é anunciado. Teste: `shown-image.spec.ts` (`@MR-028`: o mestre mostra, troca e para; a página aberta do jogador mostra e tira o bloco sem recarregar). Ver [Design](../design.md#mapas-e-imagem-mostrada).
- Testes do servidor: `TestMR028_MasterShowsAnImageToThePlayers` (mostrar, parar, apagar, e as recusas: jogador, imagem de outra campanha, sem sessão aberta), `TestRN10_PlayersOnlyFetchImagesTheyCanSee` (a imagem para de ser servida ao jogador quando para de ser mostrada) e as linhas de `SetShownImage` nas matrizes de autorização do `play`.

#### Dúvidas
- Pergunta 32 do documento de acompanhamento, respondida em 02/10/2026: o padrão fica (a imagem some da tela dos jogadores e eles perdem o acesso quando o mestre para de mostrar), e o mestre ganha um controle para deixar a imagem com os jogadores quando precisar: o "Deixar com os jogadores", construído na Etapa 6. Guardar automaticamente todas as imagens já mostradas numa lista por jogador (um "baú" de handouts) continua fora, e seria uma história nova: a lista de hoje só tem o que o mestre escolheu deixar, e é a mesma para todos os jogadores da campanha. A lista aparece só na página da sessão; na página da campanha, para o jogador, não há desenho, e ficou para uma decisão do Vinicius.
- O nome da imagem aparece para os jogadores como legenda. O nome vem do nome do arquivo enviado, então pode trazer uma anotação do mestre ("covil-secreto-do-lich"): o mestre pode renomear antes de mostrar.

### MR-025: Cadastrar conteúdo da mesa

**Como** mestre, **quero** cadastrar raças, classes, subclasses, antecedentes, magias e regras que não vêm no SRD 5.1, e definir as regras da minha mesa, **para** a campanha usar o material que a mesa joga e eu ter total controle do produto.

- Prioridade: MVP (Etapa 10, desde 03/10/2026; era "Depois")
- Regras: —
- Módulos: rules, campaigns

Prioridade decidida em 02/10/2026 (pergunta 20): primeiro o cadastro pelo mestre, antes da MR-026 e da MR-027. Em 03/10/2026, o Vinicius pôs a história no MVP, na Etapa 10, e a ampliou: magias próprias, a grade da campanha (o tamanho, ou nenhuma), regras de dados além da RN-18 e regras da casa. A MR-026 (proposta do jogador) e a MR-027 (ler um PDF) ficam depois do MVP.

#### Critérios de aceite (proposta)
- **Dado** que sou mestre de "Mirathel", **quando** cadastro uma classe nova com os dados, as perícias e as características dela, **então** a classe aparece no editor de personagem só em "Mirathel" **e** a ficha calcula os números com ela.
- **Dado** um conteúdo cadastrado em "Mirathel", **quando** abro outra campanha minha, **então** ele não aparece lá: o conteúdo vale por campanha (decidido em 29/09/2026).
- **Dado** que sou mestre de "Mirathel", **quando** cadastro uma magia própria, **então** ela aparece para os personagens de "Mirathel" **e** não aparece em outra campanha.
- **Dado** que sou mestre de "Mirathel", **quando** defino a grade da campanha com outro tamanho de quadrado, ou sem grade, **então** o combate e os mapas dela seguem essa escolha.

#### Dúvidas
- Quais regras da casa e quais regras de dados entram primeiro, e o que "sem grade" muda no movimento (RN-21): a definir no planejamento da Etapa 10.

### MR-010: Gerar masmorras

**Como** mestre, **quero** gerar uma masmorra (salas, corredores, portas e escadas, com opções de tamanho e estilo) e ter um mapa que eu possa editar, **para** não desenhar tudo na mão.

- Prioridade: MVP (Etapa 10, desde 03/10/2026; era "Depois", como "Desenhar masmorras")
- Regras: —
- Módulos: maps

#### Critérios de aceite (proposta)
- **Dado** que sou mestre de "Mirathel", **quando** peço uma masmorra com um tamanho e um estilo, **então** o app gera um mapa com salas, corredores, portas e escadas, conectado (dá para chegar a todas as salas).
- **Dado** uma masmorra gerada, **quando** eu a abro, **então** posso editar o mapa (mover, apagar e acrescentar paredes, portas e salas) **e** ele passa a ser um mapa da campanha, escondido dos jogadores (RN-10).
- **Dado** o mesmo tamanho, estilo e semente, **quando** gero duas vezes, **então** o resultado é o mesmo.

#### Dúvidas
- **Sala limpa.** O gerador `dungeon.pl` do donjon (https://donjon.bin.sh/code/dungeon/dungeon.pl) é CC BY-NC 3.0, incompatível com a Apache 2.0 do MeuRPG. O código e os dados dele nunca são copiados. Um agente lê o programa e escreve uma especificação do comportamento, com as nossas palavras; outro agente, diferente, implementa o nosso gerador só a partir dessa especificação.
- A ideia de desenhar a masmorra à mão (paredes falsas, água, baús e mímicos) segue valendo para o editor do mapa; as armadilhas são a [MR-035](#mr-035-armadilhas).
- Quais opções de tamanho e de estilo entram primeiro: a definir no planejamento da Etapa 10.
- A [MR-039](#mr-039-imagens-geradas-para-masmorras-e-cenas) usa a masmorra gerada para fazer a imagem.

### MR-029: Ganchos e pistas da cena

**Como** mestre, **quero** anotar, para cada cena de RP, os ganchos, as pistas e o que dizer, **para** conduzir a cena sem perder o fio.

- Prioridade: MVP (Etapa 8)
- Regras: RN-10, RN-20
- Módulos: play, maps, notes

#### Critérios de aceite
- **Dado** uma cena de RP no mapa, **quando** o mestre escreve nela os ganchos, as pistas e o que dizer, **então** só o mestre vê essas notas.
- **Dado** uma pista numa cena, **quando** o mestre a revela, **então** ela aparece para os jogadores **e** nas anotações deles ([MR-030](#mr-030-anotações-do-jogador)).
- **Dado** uma pista não revelada, **quando** um jogador abre a cena ou a lista de anotações, **então** o servidor não manda a pista nem o nome dela (RN-10).

#### Implementado
- O servidor (fatia 8.2; as telas vêm na 8.5), com as respostas-padrão das perguntas 59 e 63 até o Samuel responder.
  - **Ganchos:** a coluna `map_points.hooks` ("Ganchos e anotações"), Markdown de até 4.000 caracteres, só num ponto de cena, salva com o ponto (`UpdateMapPoint`) e só o mestre a recebe (no mapa e na cena aberta).
  - **Pistas:** `AddSceneClue`, `UpdateSceneClue`, `MoveSceneClue` e `RemoveSceneClue`, uma mudança por chamada, de 1 a 500 caracteres, no máximo 30 por cena; o mestre recebe cada uma com quem a tem ("Todos", "Só a Brisa", "Ninguém ainda" a tela calcula).
  - **Revelar:** `RevealSceneClue(pista, character_ids)`, só o mestre; cada jogador a recebe uma vez e não há como desfazer; o servidor guarda uma cópia do texto, então editar ou apagar a pista depois não muda o que o jogador recebeu; com sessão aberta vira o evento `clue_revealed` (só IDs) e só quem a recebeu ouve `notes_changed`; quem está offline a lê na próxima vez.
  - **Cena sem ações** (pergunta 63, mudada): `OpenScene` abre qualquer ponto de cena. O `SceneBlocked` `NO_ACTIONS` continua no enum, mas não é mais enviado.
- Testes: `TestMR029_TheMastersHooksAndClues`, `TestMR029_ClueLimits`, `TestMR029_ARevealedClueReachesOnlyTheChosenPlayers`, `TestMR029_RevealRules`, `TestClueAuthorizationMatrix`, `TestScenesWithNoActionsOpen` e `TestRN20_PlayersNeverGetHooksOrUnrevealedClues` (lê o que o jogador recebe como JSON).

#### Dúvidas
- Respondida por padrão (pergunta 59, aguardando o Samuel): uma pista revelada não pode ser escondida de novo, e o mestre escolhe a quem revelar (todos vêm marcados).

### MR-030: Anotações do jogador

**Como** jogador, **quero** um bloco de notas sempre à mão, na página da sessão e na ficha, **para** anotar o que acontece sem sair do app.

- Prioridade: MVP (Etapa 8)
- Regras: RN-10, RN-20
- Módulos: notes, maps, play

#### Critérios de aceite
- **Dado** que estou na campanha, **quando** escrevo uma nota na página da sessão ou na ficha, **então** só eu vejo a nota **e** ela fica guardada para a próxima sessão.
- **Dado** uma cena que eu já descobri (revelada ou aberta), **quando** etiqueto a nota com ela, **então** a nota mostra a cena.
- **Dado** uma cena que eu ainda não descobri, **quando** abro a lista de cenas para etiquetar, **então** ela não aparece, e o nome dela nunca chega ao meu celular.

#### Implementado
- O servidor (fatia 8.2; as telas vêm na 8.5), com as respostas-padrão das perguntas 60 e 61 até o Samuel responder.
  - O módulo `notes` e o `NotesService` (`ListNotes`, `CreateNote`, `UpdateNote`, `DeleteNote`, `ListNoteScenes`), só para o jogador ativo e só nas próprias anotações: de 1 a 2.000 caracteres, no máximo 300 por jogador por campanha. A lista traz também as pistas reveladas ("Pista do mestre", só leitura, fora das 300), da mais nova à mais antiga, com filtro por cena.
  - O mestre e os outros jogadores nunca leem uma anotação (`not_found`). Excluir a conta apaga as anotações.
  - Uma cena é descoberta quando o ponto é revelado no mapa ou a cena é aberta numa sessão (mesmo escondida), para o grupo todo, e continua descoberta se o ponto for escondido. Só uma cena descoberta serve de etiqueta, e o seletor (`ListNoteScenes`) só lista essas; uma cena não descoberta é recusada igual a uma que não existe.
- Testes: `TestMR030_PlayerNotesArePrivate`, `TestMR030_TagsOnlyDiscoveredScenes`, `TestMR030_NoteLimits`, `TestMR030_DeletingTheAccountDeletesTheNotes`, `TestNotesAuthorizationMatrix` e `TestRN20_PlayersNeverGetHooksOrUnrevealedClues`.

#### Relacionadas
- Uma pista revelada pela [MR-029](#mr-029-ganchos-e-pistas-da-cena) aparece nas anotações do jogador.

#### Dúvidas
- Respondidas por padrão (perguntas 60 e 61, aguardando o Samuel): o mestre não lê as anotações dos jogadores; o limite é de 2.000 caracteres por nota e 300 por jogador (proposta do plano); "cena descoberta" é a cena revelada ou aberta, para o grupo todo.

### MR-031: NPCs na cena

**Como** mestre, **quero** mostrar os retratos dos NPCs entrando e saindo da imagem durante uma cena, como numa visual novel, **para** os jogadores verem quem está "ali".

- Prioridade: MVP (Etapa 8)
- Regras: RN-10
- Módulos: play, maps

#### Critérios de aceite
- **Dado** uma cena de RP aberta, **quando** o mestre põe o retrato de um NPC (da galeria) na cena, **então** os jogadores o veem entrar, ao vivo.
- **Dado** um NPC na cena, **quando** o mestre o tira, **então** ele sai da imagem dos jogadores ao vivo.
- **Dado** um NPC que ainda não entrou, **quando** um jogador consulta a sessão, **então** o servidor não manda o retrato nem o nome dele.

#### Relacionadas
- Usa as imagens da [galeria](#mr-019-galeria-de-imagens), como a [MR-028](#mr-028-mostrar-uma-imagem-aos-jogadores).

### MR-032: Destaques do combate

**Como** mesa, **quero** uma tela de destaques no fim do combate, **para** celebrar quem fez o quê: qual jogador causou mais dano, curou mais e levou mais dano.

- Prioridade: MVP (Etapa 8)
- Regras: RN-20
- Módulos: play

#### Critérios de aceite
- **Dado** um combate que termina, **quando** o mestre o encerra, **então** a mesa vê, por categoria, o jogador cujo personagem causou mais dano, o que curou mais e o que levou mais dano (o "tanque").
- **Dado** que um NPC causou ou levou dano, **quando** a tela de destaques aparece para o jogador, **então** ela mostra só os números dos jogadores, sem o PV, a CA nem as rolagens dos NPCs (RN-20).

#### Dúvidas
- Pedido do Vinicius ("which player dealt the most damage, healed most, tanked more, etc."), esclarecido em 03/10/2026: os destaques são por jogador. Quais outros destaques entram além dos três (por exemplo, mais inimigos derrotados, mais críticos) fica para escolher no planejamento da Etapa 8.

### MR-033: Imprimir o mapa com a grade

**Como** mestre, **quero** imprimir o mapa atual, ou salvar em PDF, com a grade na escala da mesa, **para** jogar com miniaturas.

- Prioridade: MVP (Etapa 8)
- Regras: RN-21
- Módulos: maps

#### Critérios de aceite
- **Dado** um mapa com grade, **quando** o mestre manda imprimir, **então** sai um PDF com a grade na escala da mesa (2,5 cm por quadrado de 1,5 m).
- **Dado** um mapa maior que uma página, **quando** o mestre imprime, **então** o mapa é dividido em várias páginas, com a grade alinhada entre elas.
- **Dado** um mapa escondido dos jogadores, **quando** o mestre imprime, **então** sai o mapa inteiro: a impressão é do mestre.

#### Dúvidas
- Leitura nossa do pedido do Vinicius ("imprimir grid"), que ele não corrigiu. O tamanho do papel (A4 ou Carta) e se pontos e tokens saem na impressão: a definir.

### MR-034: Movimentos especiais

**Como** jogador, **quero** que o app trate salto, terreno difícil e cobertura, **para** as regras de movimento valerem na tela como valem na mesa.

- Prioridade: MVP (Etapa 9)
- Regras: RN-21
- Módulos: play, rules

#### Critérios de aceite
- **Dado** um personagem com Força 16, **quando** ele salta, **então** o app mostra o alcance do salto em distância e em altura, calculado a partir da Força, e desconta do movimento.
- **Dado** um quadrado marcado como terreno difícil, **quando** o personagem entra nele, **então** cada quadrado custa o dobro do movimento.
- **Dado** um alvo com meia cobertura ou três quartos de cobertura (o mestre marca), **quando** alguém ataca ou resiste, **então** o servidor soma +2 ou +5 à CA, conforme a cobertura.
- **Dado** a Disparada, **quando** o jogador a usa, **então** ela continua funcionando junto com o terreno difícil.

#### Dúvidas
- Quem marca o terreno difícil e a cobertura é o mestre, no mapa ou no combate: a definir no desenho da Etapa 9.

### MR-035: Armadilhas

**Como** mestre, **quero** pôr armadilhas escondidas no mapa, com a CD para notar e para achar, o gatilho e o efeito, **para** surpreender a mesa com as regras do jogo.

- Prioridade: MVP (Etapa 9)
- Regras: RN-10
- Módulos: maps, play

#### Critérios de aceite
- **Dado** um mapa, **quando** o mestre põe uma armadilha com a CD para notar (Percepção passiva), a CD para achar (Investigação), o gatilho e o efeito (dano, uma resistência), **então** só o mestre a vê.
- **Dado** uma armadilha escondida, **quando** um personagem tem a Percepção passiva igual ou maior que a CD, ou passa no teste de Investigação, **então** o jogador passa a ver a armadilha.
- **Dado** uma armadilha disparada, **quando** o gatilho acontece, **então** o efeito é aplicado pelo servidor (dano, resistência) **e** a armadilha passa a aparecer para os jogadores.
- **Dado** uma armadilha que ainda não foi achada nem disparada, **quando** um jogador consulta o mapa, **então** o servidor não manda nem a posição (RN-10).

#### Relacionadas
- Vem do que o desenho de masmorras (a antiga MR-010) previa: armadilhas e baús.

### MR-036: Névoa de guerra pela visão

**Como** jogador, **quero** ver no mapa só o que o meu personagem enxerga, **para** a exploração ter o suspense da mesa.

- Prioridade: MVP (Etapa 9)
- Regras: RN-10
- Módulos: maps, play, rules

#### Critérios de aceite
- **Dado** um mapa com áreas claras e escuras que o mestre marcou, **quando** o jogador o abre, **então** ele vê só o que o personagem dele enxerga.
- **Dado** uma fonte de luz (uma tocha, uma magia), **quando** o mestre a põe no mapa, **então** a área ao redor fica visível.
- **Dado** um personagem com Visão no escuro de 18 m na ficha, **quando** ele está numa área escura, **então** ele enxerga até 18 m, em tons de cinza.
- **Dado** o mestre, **quando** abre o mapa, **então** vê tudo. O que o jogador não enxerga nunca sai do servidor (RN-10).

#### Dúvidas
- Se a visão de um jogador é compartilhada com o grupo e se as paredes bloqueiam a vista (o que depende de o mapa saber onde ficam as paredes, ver a [MR-010](#mr-010-gerar-masmorras)): a definir no desenho da Etapa 9.

### MR-037: Criaturas do personagem

**Como** jogador, **quero** controlar uma criatura minha (a forma selvagem do druida, os mortos-vivos do necromante, um familiar), **para** jogá-la no combate com a ficha dela.

- Prioridade: MVP (Etapa 9)
- Regras: RN-02, RN-20
- Módulos: play, rules, characters

#### Critérios de aceite
- **Dado** um druida com forma selvagem, **quando** ele a usa, **então** o jogador ganha a ficha da criatura, tirada das criaturas do SRD, e age com ela no combate.
- **Dado** uma criatura do personagem, **quando** chega a vez dela, **então** ela entra na ordem do combate, com a própria economia de ação, e o jogador a controla.
- **Dado** uma criatura do personagem, **quando** ela sofre dano, **então** o PV dela é separado do PV do personagem, e o mestre pode corrigir (RN-02).

#### Relacionadas
- Precisa das criaturas do SRD importadas para o módulo `rules` (5e-database, como o resto do SRD).

#### Dúvidas
- Como a forma selvagem devolve o dano que sobra ao personagem, e quais criaturas e magias (Conjurar Animais, Animar Mortos, Convocar Familiar) entram primeiro: a definir no desenho da Etapa 9.

### MR-038: Quebra-cabeças

**Como** mestre, **quero** criar quebra-cabeças que os jogadores resolvem no app, ao vivo numa cena, **para** variar o ritmo da sessão.

- Prioridade: MVP (Etapa 10)
- Regras: —
- Módulos: play

#### Critérios de aceite
- **Dado** que sou mestre de "Mirathel", **quando** crio um quebra-cabeça de um tipo disponível, **então** ele fica guardado na campanha e só eu o vejo.
- **Dado** um quebra-cabeça numa cena aberta, **quando** o mestre o mostra, **então** os jogadores o veem e o resolvem ao vivo.
- **Dado** um quebra-cabeça mostrado, **quando** os jogadores o resolvem, **então** o servidor confere a solução (o jogador nunca recebe a resposta) **e** avisa o mestre.

#### Dúvidas
- Quais tipos entram primeiro (a ideia: "lights out", fechadura de combinação, símbolos giratórios como os de Skyrim): a definir na Etapa 10.
- Se a solução vale como um teste de perícia ou se é só o que o mestre decide: a definir.

### MR-039: Imagens geradas para masmorras e cenas

**Como** mestre, **quero** gerar uma imagem a partir da masmorra ou da descrição de uma cena, e pedir ajustes, **para** mostrar à mesa o lugar de que falo.

- Prioridade: MVP (Etapa 10)
- Regras: —
- Módulos: maps, gallery

#### Critérios de aceite
- **Dado** uma masmorra gerada ([MR-010](#mr-010-gerar-masmorras)) ou a descrição de uma cena, **quando** o mestre pede uma imagem, **então** o app gera a imagem e a guarda na galeria da campanha.
- **Dado** uma imagem gerada, **quando** o mestre escreve um novo pedido ("mais escura", "com uma ponte"), **então** o app a edita sabendo da cena, sem recomeçar do zero.
- **Dado** que o teto de custo do mês foi atingido, **quando** o mestre pede outra imagem, **então** o app recusa e diz por quê.
- **Dado** um pedido de imagem, **quando** o app o envia ao serviço de IA, **então** vão só o layout gerado e o texto do mestre, nunca dado pessoal.

#### Relacionadas
- Usa o Gemini (imagem, o "Nano Banana") no Vertex AI, no nosso projeto do Google Cloud, atrás de uma interface pequena. Precisa de um ADR, do operador novo em [Privacidade](../privacidade.md) (Google Vertex AI), de um segredo e de um teto de custo mensal.

#### Dúvidas
- O valor do teto de custo mensal e o que cada mestre pode gerar: a definir (pergunta nova, com o custo por imagem medido no Vertex AI).
- Candidatos a planejar na Etapa 10, a partir das outras ferramentas do donjon, sob a mesma regra de sala limpa: lista de monstros (das criaturas do SRD da MR-037), geração de encontros e de tesouro. As tabelas de dificuldade de encontro e de tesouro aleatório são do DMG, não do SRD: seriam tabelas nossas ou só conteúdo do SRD. Ainda sem história.

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

Fora do MVP. Entram na Etapa 11 do [roadmap](../roadmap.md).

### MR-007: Importar ficha em PDF

**Como** jogador, **quero** importar minha ficha de um PDF escolhendo o formato (D&D Beyond ou ficha em português), **para** não digitar tudo de novo.

- Prioridade: Depois
- Regras: RN-08
- Módulos: characters

#### Relacionadas
- RN-08: respondida em 29/09/2026 — sem DOCX; só PDF editável, no formato do D&D Beyond ou da ficha em português.

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

### MR-026: Propor uma raça ou classe nova

**Como** jogador, **quero** propor uma raça ou uma classe que não existe no app ao criar o personagem, com o PDF ou o link das regras, **para** o mestre ler e decidir.

- Prioridade: Depois (Etapa 11, em 02/10/2026)
- Regras: RN-15 (a mesma ideia de aprovação do convite)
- Módulos: rules, characters

Prioridade decidida em 02/10/2026 (pergunta 20): vem depois da MR-025, e a proposta do jogador só vale depois que o mestre aprova.

#### Critérios de aceite (proposta)
- **Dado** que quero jogar de cozinheiro, uma classe feita por fãs, **quando** crio o personagem e proponho a classe com o link do PDF, **então** o mestre vê o pedido **e** o personagem fica esperando a decisão.
- **Dado** um pedido de classe nova, **quando** o mestre aprova e cadastra as regras dela (MR-025), **então** o personagem passa a usar a classe; **quando** o mestre recusa, **então** o jogador escolhe outra classe.

#### Exemplo do Samuel
O jogador quer jogar de cozinheiro, uma classe não oficial. Ele cadastra a classe ao criar o personagem e põe o link do PDF (ou o PDF) para o mestre ler, aprovar ou recusar, e cadastrar como funcionam as regras dela.

### MR-027: Ler as regras de um PDF

**Como** mestre, **quero** mandar o PDF com as regras e ver o app cadastrar sozinho as classes, raças e regras dele, **para** não digitar tudo.

- Prioridade: Depois (Etapa 11, em 02/10/2026)
- Regras: —
- Módulos: rules

Prioridade decidida em 02/10/2026 (pergunta 21): sim, mas depois do cadastro pelo mestre (MR-025), e o mestre revisa tudo antes de valer.

#### Critérios de aceite (proposta)
- **Dado** um PDF de regras enviado pelo mestre, **quando** o app o lê, **então** mostra o que encontrou ao mestre **e** nada vale na campanha até o mestre revisar e aprovar.
- **Dado** um PDF enviado, **quando** o processamento termina ou falha, **então** o arquivo é apagado; um prazo curto (TTL) no arquivo guardado garante o apagamento mesmo se o processamento falhar (pergunta 22, 02/10/2026).

#### Dúvidas
- Ler um PDF de regras automaticamente precisa de um serviço de IA, que custa por uso e recebe o PDF. Um livro oficial tem direito autoral: o app não pode redistribuir o texto, e o resultado só pode aparecer para a mesa. Quando não der para ler o PDF, o cadastro fica com o mestre (MR-025). O app não guarda o PDF: ele fica só enquanto é processado (ver [Privacidade](../privacidade.md#a-definir)).

## Ver também

- [Regras de negócio](regras.md)
- [Visão do produto](visao.md)
- [Perguntas em aberto](perguntas-em-aberto.md)
- [Roadmap](../roadmap.md)
