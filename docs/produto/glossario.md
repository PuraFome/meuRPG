# Glossário

Usar as mesmas palavras nas conversas, nos documentos e no código evita muita confusão. A coluna "No código" diz o nome em inglês que aparece nas tabelas, no `.proto` e no Go.

| Termo | Significado | No código |
| --- | --- | --- |
| Campanha | A história contínua de uma mesa. Sessões, personagens do jogador, mapas e documentos pertencem a ela. | `campaign` |
| Membro | Um usuário numa campanha, com papel de mestre ou jogador. O papel vale só para aquela campanha. | `campaign_member`, `role` |
| Membro pendente | Quem aceitou um convite com aprovação e espera o mestre aprovar o personagem que criou (RN-15). Ainda não é membro: vê só o nome da campanha e o próprio personagem. Vira jogador quando o mestre aprova; sai da campanha quando o mestre recusa. | `status = 'pending'`, `awaiting_approval` |
| Mestre | O papel de quem conduz a campanha: cria convites e, depois, as sessões. Quem cria a campanha vira mestre dela. Uma campanha pode ter mais de um mestre, e um mestre pode passar a campanha para outro (RN-13). | `master` |
| Jogador | O papel de quem joga na campanha. Entra por um convite. | `player` |
| Nome de exibição | O nome que os outros membros veem. A própria pessoa digita no app; nunca vem do Google. | `display_name` |
| Mesa | O conjunto de campanhas de um mestre. O handle do jogador sem Google é único dentro da mesa, não por campanha (RN-17). | `home_dm_user_id`, `dm_user_id` |
| Handle | O apelido que identifica o jogador sem Google: o apelido do mestre junto do apelido do jogador, único dentro da mesa (RN-17). | `table_handles.handle_norm` |
| Convite | Link que **adiciona** um jogador à campanha. Guardamos só o hash do token. Pode exigir aprovação ("Exigir aprovação do mestre"): quem o aceita vira membro pendente, já cria o personagem pelo convite, e o mestre aprova ou recusa (RN-15). | `campaign_invite`, `requires_approval` |
| Link da sessão | Link que **leva** um membro direto para a sessão ao vivo: `/campanhas/<id>/sessao`, sem segredo nenhum. Não dá acesso a quem não é membro, nem ao membro pendente. | `/campanhas/<id>/sessao` |
| Sessão de jogo | Um encontro da mesa, com começo e fim, numerado a partir de 1 em cada campanha. Só uma fica aberta por vez, e iniciar uma sessão trava as fichas. | `game_session`, `session_number` |
| Sessão ao vivo | A sessão de jogo aberta, vista pela página da sessão: o mestre e os jogadores veem as mudanças na hora, sem recarregar, por um stream que só fica aberto com a página visível. | `GetLiveSession`, `WatchGameSession` |
| Aviso de sessão | O aviso no app de que começou uma sessão numa campanha da pessoa (RN-06). O app pergunta ao servidor a cada 30 segundos, com a aba visível; não há notificação com o app fechado. | `ListOpenGameSessions` |
| Foto da sessão | Tudo o que a página da sessão mostra, lido de uma vez (a sessão e os PV). O app lê a foto depois que o stream fica pronto, e de novo a cada reconexão; os eventos do stream só atualizam a foto. | `GetLiveSession` (snapshot) |
| PV, espaços e dados de vida | Os números do personagem que mudam no jogo e duram de uma sessão para outra: PV atual, PV temporários (somem primeiro no dano e não entram no máximo), espaços de magia usados por círculo, espaços de pacto usados (bruxo) e dados de vida usados. Os máximos vêm da ficha; o mestre corrige os valores durante a sessão (RN-02). | `CharacterVitals`, `character_vitals` |
| Evento da sessão | Uma mudança feita na mesa, guardada numa linha que nunca muda, numerada na ordem em que aconteceu. É o histórico da sessão: a correção do mestre nos PV e as mudanças do combate (os ataques, o dano, as ações, o desfazer). | `session_events`, `seq` |
| Chave de idempotência | O UUID que o app gera para uma mudança e manda de novo se precisar repetir o pedido. O servidor reconhece a chave e não aplica a mudança duas vezes. | `idempotency_key` |
| Sessão de login | O login de um usuário no app. Não confundir com a sessão de jogo. | `auth_session` |
| Intenção de login | Algo que a pessoa pediu antes de entrar e que o servidor conclui logo depois do login, como aceitar um convite. Fica só no servidor, dentro do estado do login. | `intent`, `IntentHandler` |
| Cena de RP | Momento fora de combate, aberto por um ponto de interesse. | `scene` |
| Ação da cena | Um item da lista simples do que o jogador pode fazer ou rolar numa cena. | `scene_action` |
| Economia de ação | O que o personagem pode gastar num turno: uma ação, uma ação bônus, uma reação e o movimento (o deslocamento, dobrado depois da Disparada). O motor calcula o que ainda está disponível (MR-014). | `Economy`, `TurnOptions` |
| Recurso | Uma capacidade com usos limitados, como Retomar o Fôlego (1 uso por descanso curto), Ki ou Fúria. A ficha diz o máximo e quando volta; a sessão conta os usos gastos. | `Resource`, `Derived.Resources` |
| Código de motivo | O motivo de uma opção estar desabilitada, como `NO_SLOT` ou `ACTION_USED`. O servidor manda só o código; o app escreve a frase em português. | `DisabledReason` |
| Encontro | Um combate num mapa, com iniciativa, rodadas e turnos. Tem três estados: preparação (`setup`, escolhendo quem luta e rolando a iniciativa), em andamento (`active`) e encerrado (`ended`). Uma sessão tem no máximo um que não terminou. | `encounter` |
| Combatente | Um personagem dentro de um encontro; um NPC entra em cópias ("Goblin 1", "Goblin 2"), cada uma com o próprio rótulo. Guarda a iniciativa, a posição, o movimento, o PV (só o NPC) e se está escondido daquele combate. | `combatant` |
| Iniciativa | A rolagem que define a ordem dos turnos num encontro. Cada combatente rola a própria, inclusive NPCs iguais (RN-19). | `initiative` |
| Grade | O quadriculado do mapa no combate. Cada quadrado vale 1,5 m, inclusive na diagonal (RN-21). O mestre define quantos quadrados cabem na largura do mapa; as linhas seguem a proporção da imagem. | `grid`, `grid_columns` |
| Dano pendente | O dano de um ataque que acertou, entre o d20 e o fim: falta rolar, foi rolado e espera o mestre (num personagem de jogador) ou já foi aplicado ou descartado. O mestre aplica ("Aplicar 5 de dano") ou descarta ("Não aplicar"); passar o turno com dano aberto pede confirmação (RN-02). | `pending_damages`, `PendingDamage` |
| Registro do combate | A lista, da última rodada para a primeira, do que aconteceu no combate: ataques, ações, movimentos, começo e fim. O jogador só recebe o que vê (nada de um combatente escondido, dos dados do mestre ou dos PV de um NPC, RN-20); o mestre recebe tudo, com "Só o mestre vê" nas linhas que os jogadores não têm. Vem dos eventos da sessão. | `ListCombatLog`, `CombatLogEntry` |
| Desfazer | A volta de um passo da última ação (ataque, dano, aplicar, ação, magia, reação, teste contra a morte, condição ou PV de um NPC; a confirmação da morte não), só do mestre: um evento compensatório repõe o que havia antes, e o histórico guarda os dois. | `UndoLastAction`, `action_undone` |
| Derrubado | A condição do SRD `condition:prone` (deitado no chão), que o mestre marca em "Condições…". Chama-se "Derrubado", e não "Caído", para não confundir com o personagem a 0 PV (pergunta 43, padrão adotado). | `condition:prone` |
| Caído | Um personagem de jogador a 0 PV que ainda faz testes contra a morte. Todos que o veem recebem a palavra, nunca os números. Não é a condição "derrubado" (deitado no chão, `condition:prone`), que o mestre marca à parte. | `COMBATANT_STATE_DOWN` |
| Estável | Um personagem a 0 PV com três sucessos no teste contra a morte: para de rolar e continua a 0 PV, inconsciente. Todos veem a palavra. Dano o faz recomeçar (os sucessos zeram e conta uma falha) (RN-03). | `COMBATANT_STATE_STABLE` |
| Morrendo | Um personagem a 0 PV com três falhas, esperando o mestre confirmar a morte. Só o mestre vê a palavra; os jogadores veem "Caído" e as contagens (RN-03). | `COMBATANT_STATE_DYING` |
| Vez do mestre | O que o jogador vê quando quem joga é um combatente escondido dele: sem nome, só "Vez do mestre" (RN-10, RN-20). | `master_turn` |
| Estado do inimigo | A palavra que o jogador vê no lugar do PV de um NPC: Ileso, Ferido, Muito ferido (metade ou menos) ou Derrotado (RN-20). | `CombatantState` |
| Empate de iniciativa | Dois combatentes com o mesmo total e o mesmo bônus: o mestre decide quem vai primeiro (RN-19). | `tie_unresolved` |
| Teste contra a morte | O teste do personagem com 0 PV, no começo da vez dele: 10 ou mais é sucesso, menos é falha, 1 natural são duas falhas e 20 natural volta com 1 PV. Dano a 0 PV conta uma falha (duas no crítico). Na terceira falha, ele só morre quando o mestre confirma (RN-03). | `RollDeathSave`, `death_save_due` |
| Concentração | Manter uma magia que pede isso. Conjurar outra de concentração termina a primeira; quando o personagem concentrado leva dano, o app lembra o teste: CD 10 ou metade do dano, a maior (RN-22). O app só lembra; quem rola é a mesa. | `concentration_spell`, `concentration_dc` |
| Condição | Um rótulo do SRD que o mestre marca num combatente (envenenado, amedrontado, derrubado...). O app mostra e lembra, e não aplica efeito nenhum (RN-22). | `conditions` |
| Reação | A ação que cabe uma vez por rodada, fora da vez ou na dela, e que volta no começo da vez do combatente. No app: o Escudo quando um golpe acerta e o ataque de oportunidade. | `reaction_used`, `UseReaction` |
| Aviso de reação | O aviso "Você foi atingido: usar Escudo Arcano?" que o jogador atingido e o mestre recebem enquanto o golpe espera. Diz quem atacou e com quê, se o jogador vê o atacante; nunca traz o total do golpe nem a CA do atacante. | `reaction_prompts` |
| Ataque de oportunidade | Um ataque corpo a corpo fora da vez, que gasta a reação no lugar da ação. | `as_reaction` |
| Ataque Extra | O recurso que deixa a Ação de Atacar fazer mais de um ataque (dois no 5º nível do guerreiro, bárbaro, monge, paladino e patrulheiro). | `attacks_per_action`, `attacks_left` |
| Personagem | Tem um tipo: jogador, inimigo, boss, minion ou NPC de história. O personagem de jogador que morre não é apagado: fica no sistema, como base de outro personagem ou como NPC do mestre em outra campanha (RN-03, RN-04). | `character`, `kind` |
| Ficha | Os dados de regra de um personagem. Completa para jogador, inimigo e boss; básica (PV, CA, deslocamento, iniciativa e até três ataques com dados) para minion e NPC de história. | `sheet`, `FullSheet`, `BasicSheet` |
| Trava da ficha | Momento em que o jogador deixa de editar a própria ficha: o início da primeira sessão da campanha, ou da próxima sessão para um personagem criado depois. A história do personagem tem trava própria, que o mestre libera. | `sheet_locked_at`, `story_editing_allowed` |
| Estado do personagem | Onde o personagem está no ciclo de vida: rascunho, travada, morto ou pendente de aprovação. O servidor calcula a cada leitura; o NPC está sempre em rascunho. | `CharacterState` |
| História do personagem | Personalidade, aparência, história e aliados: o texto que descreve o personagem, fora das regras. Depois da trava da ficha, o jogador só a edita quando o mestre libera (liberação da história), até a próxima sessão. | `CharacterStory`, `story_editing_allowed` |
| Revisão do personagem | Número que sobe a cada mudança no nome, na ficha ou na história. Se o personagem mudou depois que o app o abriu, o salvamento é recusado, e a pessoa recarrega a ficha. | `revision` |
| Notas do mestre | O que o mestre anota sobre um personagem, separado por campanha. Só o mestre lê e edita; nunca chegam ao jogador (RN-11). | `character_master_notes` |
| Documento da campanha | O texto em que o mestre prepara a campanha (MR-018): um por campanha, em Markdown, com imagens da galeria e links para mapas e fichas que abrem numa janela. Só o mestre lê e edita (decidido em 02/10/2026, pergunta 27: no MVP, só o mestre). Tem revisão, como o personagem: se o documento mudou depois que o app o abriu, o salvamento é recusado. | `campaign_documents`, `CampaignDocument` |
| Conteúdo de regras (SRD 5.1) | O conjunto de raças, classes, magias e regras que o app conhece. Vem do SRD 5.1 (CC-BY-4.0) e tem uma versão, como `srd51@a8abc93b235c+fx.1`, que cada ficha guarda (a tela não mostra: é um detalhe interno). | `content_version` |
| Chave de conteúdo | O identificador estável de um item de regra, como `class:wizard` ou `spell:fire-bolt`. A ficha guarda chaves, nunca nomes. | `key` |
| Escolhas da ficha (build) | O que o jogador escolheu: atributos base, raça, classes, antecedente, perícias, magias e equipamento. Os números nunca são guardados. | `Build`, `FullSheet` |
| Valores calculados | Os números da ficha que o servidor calcula a cada leitura, a partir das escolhas e do conteúdo de regras. O navegador nunca calcula uma regra. | `DerivedSheet`, `rules.Derive` |
| Efeito | A regra de uma característica ou de um traço escrita como dado: um bônus, uma proficiência, um sentido, uma dica. | `effect` |
| Fórmula | A conta curta de um efeito, como `8 + prof() + mod("int")` para a CD de magia. Roda na biblioteca Expr, com uma lista fechada do que é permitido. | `formula` |
| Dica | Um bônus ou uma vantagem que depende da situação, como a Esperteza Gnômica. A ficha mostra; o mestre decide quando vale. | `hint` |
| Pendência da ficha | Uma escolha fora da regra ou uma chave desconhecida. A ficha abre do mesmo jeito e mostra o aviso: o app ajuda, não julga. | `issue` |
| Antecedente personalizado | Um antecedente fora do SRD, como o Sábio: nome e 2 perícias, sem texto de livro. | `custom_background` |
| Revisão de efeitos | O número que sobe a cada mudança nos efeitos escritos por nós. Uma versão de conteúdo nunca muda no lugar. | `fx.<n>` |
| Cópia de personagem | Um personagem novo feito a partir de outro, para jogar em outra campanha. Depois de copiado, cada um segue sozinho. | `copied_from_id` |
| Galeria | As imagens de uma campanha, que o mestre envia para usar nos mapas e no documento da campanha (MR-019). Só o mestre vê a galeria; o jogador só vê uma imagem enquanto ela aparece para ele, num mapa que ele vê mostrada na sessão ou deixada com eles. Cada imagem é guardada sem metadados (EXIF, GPS) e tem uma miniatura. | `gallery_images`, `GalleryService` |
| Mapa | Uma imagem da galeria com pontos de interesse e tokens por cima (MR-008). Nasce escondido; o jogador só o vê depois que o mestre revela, ou enquanto é o mapa atual da sessão. | `maps`, `Map`, `MapService` |
| Ponto de interesse | Lugar no mapa que abre uma batalha, um submapa ou uma cena de RP. Tem nome, uma descrição para os jogadores e a posição na imagem. Nasce escondido. | `map_points`, `MapPoint` |
| Submapa | Um mapa aberto a partir de um ponto de interesse de outro mapa, como a torre dentro da região. O ponto de submapa leva a ele; o jogador só entra se também vê o submapa. | `MAP_POINT_KIND_SUBMAP`, `target_map_id`, `parent_maps` |
| Token | O marcador de um personagem num mapa: o do jogador nasce visível, o do NPC nasce escondido, e o mestre o revela quando quiser (decidido em 02/10/2026, pergunta 31). O mestre põe, move, esconde e tira; o jogador vê o token visível se mexer na hora. | `map_tokens`, `MapToken` |
| Revelar, escondido | Mostrar aos jogadores um mapa, um ponto ou um token, ou esconder de novo (RN-10). O que está escondido nunca sai do servidor para o jogador: nem o ID. Na tela do mestre, escondido é tracejado, com o olho riscado e a palavra "Escondido". | `revealed_at`, `hidden`, `SetMapRevealed` |
| Mapa atual | O mapa que o mestre mostra na sessão aberta, a todos. Escolher o mapa atual o revela; uma sessão nova começa sem mapa atual. | `game_sessions.current_map_id`, `SetCurrentMap` |
| Imagem mostrada | Uma imagem da galeria que o mestre mostra aos jogadores durante a sessão, como um retrato ou uma carta (MR-028). Uma por vez, ao lado do mapa atual; não revela mais nada. Quando o mestre para de mostrar, a imagem some da tela dos jogadores e eles perdem o acesso a ela, a menos que o mestre a tenha deixado com eles (decidido em 02/10/2026, pergunta 32; ver Imagem deixada). | `game_sessions.shown_image_id`, `SetShownImage` |
| Imagem deixada | Uma imagem mostrada que o mestre deixou com os jogadores ("Deixar com os jogadores", MR-028): continua na página da sessão deles, em "Imagens que o mestre deixou", até o mestre tirar ("Tirar"), mesmo depois da sessão acabar. É da campanha, não da sessão. | `campaign_left_images`, `ListLeftImages`, `TakeBackLeftImage` |
| Posição em pontos-base | Onde fica um ponto ou um token na imagem: de 0 a 10000 na largura e na altura (5000 é o meio). Não depende do tamanho da imagem em pixels. | `x_bp`, `y_bp` |
| Masmorra | Mapa desenhado com paredes (inclusive falsas), piso, água, portas, armadilhas e baús (normais ou mímicos). | `dungeon` |
| Gerador de masmorras | A ferramenta do mestre que gera uma masmorra (salas, corredores, portas, escadas, com opções de tamanho e estilo) como um mapa que ele pode editar (MR-010). Feito por design de sala limpa. | — |
| Ganchos e pistas | As anotações privadas do mestre para cada cena de RP: ganchos, pistas e o que dizer. Uma pista pode ser revelada aos jogadores (MR-029). | — |
| Anotações do jogador | O bloco de notas privado de cada jogador, sempre à mão; uma nota pode levar a etiqueta de uma cena que o jogador já descobriu (MR-030). | — |
| Destaques do combate | A tela do fim do combate para a mesa: quem curou mais, quem causou mais dano e quem levou mais ("tanque") (MR-032). | — |
| Ataque conjunto | Combatentes com a mesma iniciativa, como um grupo de goblins, marcados na ordem dos turnos para o mestre jogá-los juntos. É um lembrete, não uma automação (MR-013). | — |
| Armadilha | Um ponto escondido no mapa, com a CD para notar (Percepção passiva) e para achar (Investigação), um gatilho e um efeito. O mestre vê todas; o jogador só vê a que achou ou disparou (MR-035). | — |
| Névoa de guerra | O mapa mostra a cada jogador só o que o personagem dele enxerga: luz, escuro e visão no escuro. O mestre vê tudo (MR-036). | — |
| Criatura do personagem | Uma criatura que o jogador controla, como a forma selvagem do druida, os mortos-vivos do necromante ou um familiar. Tem ficha própria, tirada das criaturas do SRD, e age no combate (MR-037). | — |
| Quebra-cabeça | Um desafio que o mestre cria e os jogadores resolvem no app, ao vivo numa cena, como o "lights out" ou uma fechadura de combinação (MR-038). | — |
| Modo de XP | Como a campanha dá XP: por inimigos derrotados, por ouro ou por marcos. | `xp_mode` |
| Conteúdo da mesa (homebrew) | Raças, classes, subclasses, antecedentes e regras que não vêm no SRD e que a mesa cadastra. Vale por campanha; o jogador pode propor, e o mestre aprova (MR-025, MR-026). | `rules_pack` |
| Dado físico ou do app | Como o jogador rola: no app, ou no dado de verdade, digitando o resultado. O mestre decide se a campanha deixa escolher (RN-18); a escolha de cada jogador é a preferência de dados. | `dice_mode`, `dice_preference` |
| Marco (milestone) | No modo de XP por marcos, o mestre sobe o nível do grupo quando a história chega num ponto combinado. Ao "Registrar marco", os personagens que ele escolhe ficam marcados "Pode subir de nível", sem contar XP; a marca dura até o nível da ficha subir (RN-12). | `milestone`, `MarkMilestone` |
| Nível de desafio (ND) | A força de uma criatura, de 0 a 30 ("1/8", "1/4", "1/2", "1"...). A tabela do SRD diz quanto XP ela dá ao ser derrotada (ND 1/4 = 50 XP); o mestre preenche o ND na ficha do NPC e pode digitar outro valor de XP. | `challenge_rating`, `xp_value` |
| Pode subir de nível | O aviso, na ficha e na lista do grupo, de que o personagem já pode subir de nível: o XP da ficha chegou ao do próximo nível (300, 900, 2.700...), ou o mestre registrou um marco. O mestre aplica o novo nível na ficha; a tela de subir de nível fica para depois do MVP (RN-12). | `can_level_up`, `LevelUpReason` |
| Prêmio de XP | Cada "Dar XP" do mestre (por inimigos, por ouro ou avulso) ou marco registrado, com quem deu, quando, por quê e a parte de cada personagem. O histórico é lido por toda a campanha; só o último pode ser desfeito, e o prêmio desfeito continua lá, com a etiqueta "Desfeito". | `xp_awards`, `XPAward` |
| Etapa | Uma fase do roadmap deste guia. Usamos "etapa" para não confundir com o marco de XP. | — |

## Ver também

- [Visão do produto](visao.md)
- [Regras de negócio](regras.md)
- [Modelo de dados](../dados.md): onde cada termo vira tabela.
