# Glossário

Usar as mesmas palavras nas conversas, nos documentos e no código evita muita confusão. A coluna "No código" diz o nome em inglês que aparece nas tabelas, no `.proto` e no Go.

| Termo | Significado | No código |
| --- | --- | --- |
| Campanha | A história contínua de uma mesa. Sessões, personagens do jogador, mapas e documentos pertencem a ela. | `campaign` |
| Membro | Um usuário numa campanha, com papel de mestre ou jogador. O papel vale só para aquela campanha. | `campaign_member`, `role` |
| Mestre | O papel de quem conduz a campanha: cria convites e, depois, as sessões. Quem cria a campanha vira mestre dela. Uma campanha pode ter mais de um mestre, e um mestre pode passar a campanha para outro (RN-13). | `master` |
| Jogador | O papel de quem joga na campanha. Entra por um convite. | `player` |
| Nome de exibição | O nome que os outros membros veem. A própria pessoa digita no app; nunca vem do Google. | `display_name` |
| Mesa | O conjunto de campanhas de um mestre. O handle do jogador sem Google é único dentro da mesa, não por campanha (RN-17). | `home_dm_user_id`, `dm_user_id` |
| Handle | O apelido que identifica o jogador sem Google: o apelido do mestre junto do apelido do jogador, único dentro da mesa (RN-17). | `table_handles.handle_norm` |
| Convite | Link que **adiciona** um jogador à campanha. Guardamos só o hash do token. Pode exigir aprovação: o jogador já cria o personagem pelo convite, e o mestre aprova ou recusa (RN-15). | `campaign_invite` |
| Link da sessão | Link que **leva** um membro direto para a sessão ao vivo. Não dá acesso a quem não é membro. | `session_link` |
| Sessão de jogo | Um encontro da mesa, com começo e fim. | `game_session` |
| Sessão de login | O login de um usuário no app. Não confundir com a sessão de jogo. | `auth_session` |
| Intenção de login | Algo que a pessoa pediu antes de entrar e que o servidor conclui logo depois do login, como aceitar um convite. Fica só no servidor, dentro do estado do login. | `intent`, `IntentHandler` |
| Cena de RP | Momento fora de combate, aberto por um ponto de interesse. | `scene` |
| Ação da cena | Um item da lista simples do que o jogador pode fazer ou rolar numa cena. | `scene_action` |
| Encontro | Um combate num mapa, com iniciativa, rodadas e turnos. | `encounter` |
| Combatente | Um personagem dentro de um encontro. Guarda PV atual, iniciativa e posição daquele combate. | `combatant` |
| Personagem | Tem um tipo: jogador, inimigo, boss, minion ou NPC de história. O personagem de jogador que morre não é apagado: fica no sistema, como base de outro personagem ou como NPC do mestre em outra campanha (RN-03, RN-04). | `character`, `kind` |
| Ficha | Os dados de regra de um personagem. Completa para jogador, inimigo e boss; básica para minion e NPC de história. | `sheet` |
| Trava da ficha | Momento em que o jogador deixa de editar a própria ficha: o início da primeira sessão da campanha. | `sheet_locked_at` |
| Cópia de personagem | Um personagem novo feito a partir de outro, para jogar em outra campanha. Depois de copiado, cada um segue sozinho. | `copied_from_id` |
| Ponto de interesse | Lugar no mapa que abre uma batalha, um submapa ou uma cena de RP. | `point_of_interest` |
| Masmorra | Mapa desenhado com paredes (inclusive falsas), piso, água, portas, armadilhas e baús (normais ou mímicos). | `dungeon` |
| Modo de XP | Como a campanha dá XP: por inimigos derrotados, por ouro ou por marcos. | `xp_mode` |
| Marco (milestone) | No modo de XP por marcos, o mestre sobe o nível do grupo quando a história chega num ponto combinado. | `milestone` |
| Etapa | Uma fase do roadmap deste guia. Usamos "etapa" para não confundir com o marco de XP. | — |

## Ver também

- [Visão do produto](visao.md)
- [Regras de negócio](regras.md)
- [Modelo de dados](../dados.md): onde cada termo vira tabela.
