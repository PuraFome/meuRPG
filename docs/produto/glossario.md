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
| Evento da sessão | Uma mudança feita na mesa, guardada numa linha que nunca muda, numerada na ordem em que aconteceu. É o histórico da sessão; na Etapa 5, só a correção do mestre nos PV. | `session_events`, `seq` |
| Chave de idempotência | O UUID que o app gera para uma mudança e manda de novo se precisar repetir o pedido. O servidor reconhece a chave e não aplica a mudança duas vezes. | `idempotency_key` |
| Sessão de login | O login de um usuário no app. Não confundir com a sessão de jogo. | `auth_session` |
| Intenção de login | Algo que a pessoa pediu antes de entrar e que o servidor conclui logo depois do login, como aceitar um convite. Fica só no servidor, dentro do estado do login. | `intent`, `IntentHandler` |
| Cena de RP | Momento fora de combate, aberto por um ponto de interesse. | `scene` |
| Ação da cena | Um item da lista simples do que o jogador pode fazer ou rolar numa cena. | `scene_action` |
| Encontro | Um combate num mapa, com iniciativa, rodadas e turnos. | `encounter` |
| Combatente | Um personagem dentro de um encontro. Guarda PV atual, iniciativa e posição daquele combate. | `combatant` |
| Iniciativa | A rolagem que define a ordem dos turnos num encontro. Cada combatente rola a própria, inclusive NPCs iguais (RN-19). | `initiative` |
| Grade | O quadriculado do mapa no combate. Cada quadrado vale 1,5 m, inclusive na diagonal (RN-21). | `grid` |
| Teste contra a morte | O teste do personagem com 0 PV. Na terceira falha, ele só morre quando o mestre confirma (RN-03). | `death_save` |
| Personagem | Tem um tipo: jogador, inimigo, boss, minion ou NPC de história. O personagem de jogador que morre não é apagado: fica no sistema, como base de outro personagem ou como NPC do mestre em outra campanha (RN-03, RN-04). | `character`, `kind` |
| Ficha | Os dados de regra de um personagem. Completa para jogador, inimigo e boss; básica para minion e NPC de história. | `sheet`, `FullSheet`, `BasicSheet` |
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
| Galeria | As imagens de uma campanha, que o mestre envia para usar nos mapas e no documento da campanha (MR-019). Só o mestre vê a galeria; o jogador só vê uma imagem enquanto ela aparece para ele, num mapa que ele vê ou mostrada na sessão. Cada imagem é guardada sem metadados (EXIF, GPS) e tem uma miniatura. | `gallery_images`, `GalleryService` |
| Mapa | Uma imagem da galeria com pontos de interesse e tokens por cima (MR-008). Nasce escondido; o jogador só o vê depois que o mestre revela, ou enquanto é o mapa atual da sessão. | `maps`, `Map`, `MapService` |
| Ponto de interesse | Lugar no mapa que abre uma batalha, um submapa ou uma cena de RP. Tem nome, uma descrição para os jogadores e a posição na imagem. Nasce escondido. | `map_points`, `MapPoint` |
| Submapa | Um mapa aberto a partir de um ponto de interesse de outro mapa, como a torre dentro da região. O ponto de submapa leva a ele; o jogador só entra se também vê o submapa. | `MAP_POINT_KIND_SUBMAP`, `target_map_id`, `parent_maps` |
| Token | O marcador de um personagem num mapa: o do jogador nasce visível, o do NPC nasce escondido, e o mestre o revela quando quiser (decidido em 02/10/2026, pergunta 31). O mestre põe, move, esconde e tira; o jogador vê o token visível se mexer na hora. | `map_tokens`, `MapToken` |
| Revelar, escondido | Mostrar aos jogadores um mapa, um ponto ou um token, ou esconder de novo (RN-10). O que está escondido nunca sai do servidor para o jogador: nem o ID. Na tela do mestre, escondido é tracejado, com o olho riscado e a palavra "Escondido". | `revealed_at`, `hidden`, `SetMapRevealed` |
| Mapa atual | O mapa que o mestre mostra na sessão aberta, a todos. Escolher o mapa atual o revela; uma sessão nova começa sem mapa atual. | `game_sessions.current_map_id`, `SetCurrentMap` |
| Imagem mostrada | Uma imagem da galeria que o mestre mostra aos jogadores durante a sessão, como um retrato ou uma carta (MR-028). Uma por vez, ao lado do mapa atual; não revela mais nada. Quando o mestre para de mostrar, a imagem some da tela dos jogadores e eles perdem o acesso a ela (decidido em 02/10/2026, pergunta 32). | `game_sessions.shown_image_id`, `SetShownImage` |
| Posição em pontos-base | Onde fica um ponto ou um token na imagem: de 0 a 10000 na largura e na altura (5000 é o meio). Não depende do tamanho da imagem em pixels. | `x_bp`, `y_bp` |
| Masmorra | Mapa desenhado com paredes (inclusive falsas), piso, água, portas, armadilhas e baús (normais ou mímicos). | `dungeon` |
| Modo de XP | Como a campanha dá XP: por inimigos derrotados, por ouro ou por marcos. | `xp_mode` |
| Conteúdo da mesa (homebrew) | Raças, classes, subclasses, antecedentes e regras que não vêm no SRD e que a mesa cadastra. Vale por campanha; o jogador pode propor, e o mestre aprova (MR-025, MR-026). | `rules_pack` |
| Dado físico ou do app | Como o jogador rola: no app, ou no dado de verdade, digitando o resultado. O mestre decide se a campanha deixa escolher (RN-18). | `dice_mode` |
| Marco (milestone) | No modo de XP por marcos, o mestre sobe o nível do grupo quando a história chega num ponto combinado. | `milestone` |
| Etapa | Uma fase do roadmap deste guia. Usamos "etapa" para não confundir com o marco de XP. | — |

## Ver também

- [Visão do produto](visao.md)
- [Regras de negócio](regras.md)
- [Modelo de dados](../dados.md): onde cada termo vira tabela.
