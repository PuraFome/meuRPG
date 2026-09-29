# Glossário

Usar as mesmas palavras nas conversas, nos documentos e no código evita muita confusão. A coluna "No código" diz o nome em inglês que aparece nas tabelas, no `.proto` e no Go.

| Termo | Significado | No código |
| --- | --- | --- |
| Campanha | A história contínua de uma mesa. Sessões, personagens do jogador, mapas e documentos pertencem a ela. | `campaign` |
| Membro | Um usuário numa campanha, com papel de mestre ou jogador. O papel vale só para aquela campanha. | `campaign_member`, `role` |
| Convite | Link que **adiciona** um jogador à campanha. Guardamos só o hash do token. | `campaign_invite` |
| Link da sessão | Link que **leva** um membro direto para a sessão ao vivo. Não dá acesso a quem não é membro. | `session_link` |
| Sessão de jogo | Um encontro da mesa, com começo e fim. | `game_session` |
| Sessão de login | O login de um usuário no app. Não confundir com a sessão de jogo. | `auth_session` |
| Cena de RP | Momento fora de combate, aberto por um ponto de interesse. | `scene` |
| Ação da cena | Um item da lista simples do que o jogador pode fazer ou rolar numa cena. | `scene_action` |
| Encontro | Um combate num mapa, com iniciativa, rodadas e turnos. | `encounter` |
| Combatente | Um personagem dentro de um encontro. Guarda PV atual, iniciativa e posição daquele combate. | `combatant` |
| Personagem | Tem um tipo: jogador, inimigo, boss, minion ou NPC de história. | `character`, `kind` |
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
