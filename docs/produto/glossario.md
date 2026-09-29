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
| Link da sessão | Link que **leva** um membro direto para a sessão ao vivo. Não dá acesso a quem não é membro. | `session_link` |
| Sessão de jogo | Um encontro da mesa, com começo e fim, numerado a partir de 1 em cada campanha. Só uma fica aberta por vez, e iniciar uma sessão trava as fichas. | `game_session`, `session_number` |
| Sessão de login | O login de um usuário no app. Não confundir com a sessão de jogo. | `auth_session` |
| Intenção de login | Algo que a pessoa pediu antes de entrar e que o servidor conclui logo depois do login, como aceitar um convite. Fica só no servidor, dentro do estado do login. | `intent`, `IntentHandler` |
| Cena de RP | Momento fora de combate, aberto por um ponto de interesse. | `scene` |
| Ação da cena | Um item da lista simples do que o jogador pode fazer ou rolar numa cena. | `scene_action` |
| Encontro | Um combate num mapa, com iniciativa, rodadas e turnos. | `encounter` |
| Combatente | Um personagem dentro de um encontro. Guarda PV atual, iniciativa e posição daquele combate. | `combatant` |
| Personagem | Tem um tipo: jogador, inimigo, boss, minion ou NPC de história. O personagem de jogador que morre não é apagado: fica no sistema, como base de outro personagem ou como NPC do mestre em outra campanha (RN-03, RN-04). | `character`, `kind` |
| Ficha | Os dados de regra de um personagem. Completa para jogador, inimigo e boss; básica para minion e NPC de história. | `sheet`, `FullSheet`, `BasicSheet` |
| Trava da ficha | Momento em que o jogador deixa de editar a própria ficha: o início da primeira sessão da campanha, ou da próxima sessão para um personagem criado depois. A história do personagem tem trava própria, que o mestre libera. | `sheet_locked_at`, `story_editing_allowed` |
| Estado do personagem | Onde o personagem está no ciclo de vida: rascunho, travada, morto ou pendente de aprovação. O servidor calcula a cada leitura; o NPC está sempre em rascunho. | `CharacterState` |
| História do personagem | Personalidade, aparência, história e aliados: o texto que descreve o personagem, fora das regras. Depois da trava da ficha, o jogador só a edita quando o mestre libera (liberação da história), até a próxima sessão. | `CharacterStory`, `story_editing_allowed` |
| Revisão do personagem | Número que sobe a cada mudança no nome, na ficha ou na história. Se o personagem mudou depois que o app o abriu, o salvamento é recusado, e a pessoa recarrega a ficha. | `revision` |
| Notas do mestre | O que o mestre anota sobre um personagem, separado por campanha. Só o mestre lê e edita; nunca chegam ao jogador (RN-11). | `character_master_notes` |
| Conteúdo de regras (SRD 5.1) | O conjunto de raças, classes, magias e regras que o app conhece. Vem do SRD 5.1 (CC-BY-4.0) e tem uma versão, como `srd51@a8abc93b235c+fx.1`, que a ficha mostra. | `content_version` |
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
| Ponto de interesse | Lugar no mapa que abre uma batalha, um submapa ou uma cena de RP. | `point_of_interest` |
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
