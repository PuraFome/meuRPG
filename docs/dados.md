# Modelo de dados

A campanha é o centro do banco novo. O schema começa do zero: a migration `00001_init` está vazia, e cada módulo cria as próprias tabelas conforme é construído (decidido em 29/09/2026). Não há migração de dados do app antigo — o diagrama deste documento é o alvo proposto, montado módulo por módulo, não um destino que os dados de hoje precisam alcançar.

## O que muda em relação ao app antigo (referência)

No app antigo (`server/src/db/schema.sql`, descontinuado), a campanha não existia, e tudo pertencia direto ao usuário. A tabela abaixo serve só para quem conhece o schema antigo entender as escolhas novas — nenhuma linha dele é migrada.

| App antigo | Novo (proposta) | Por quê |
| --- | --- | --- |
| `users.role` é global (padrão `master`) | O papel vai para `campaign_members.role` | Um usuário pode ser mestre numa campanha e jogador em outra (RN-05). |
| `characters.type` aceita `npc`, `player`, `boss`, `minion` | `characters.kind` aceita `player`, `enemy`, `boss`, `minion`, `story` | `npc` vira dois tipos: inimigo (ficha completa) e história (ficha básica). |
| O personagem liga ao mestre por `master_user_id` | `campaign_characters` liga personagens a campanhas | Um índice único parcial deixa o personagem de jogador em uma campanha só (RN-03), e o NPC em várias (RN-04). |
| Sem cópia de personagem | `characters.copied_from_id` e `characters.sheet_locked_at` | A cópia lembra de onde veio (RN-03), e a trava tem data (RN-01). |
| `master_notes` na mesma linha do personagem | Tabela própria, `character_master_notes` | A consulta que monta a ficha do jogador nem toca nessa tabela, então não tem como vazar (RN-11). |
| `character_join_tokens.token` em texto puro, por personagem | `campaign_invites.token_hash`, por campanha, com expiração | Quem lê o banco não consegue usar o convite (RN-07). |
| `maps.user_id` | `maps.campaign_id` | O mapa pertence à campanha. |
| `maps.background_image` guarda a imagem em base64 | Imagem no Cloud Storage; a tabela guarda só a URL | Cada leitura do mapa mandaria a imagem inteira. Sair de São Paulo custa US$ 0,19 por GiB, e a URL deixa o navegador usar cache. |
| Galeria só no navegador de quem usa | `gallery_items`, ligada à campanha | Fica disponível em qualquer aparelho. |

Tabelas novas para a mesa ao vivo:

- `game_sessions`: começo e fim de cada sessão. Iniciar a primeira preenche `sheet_locked_at` das fichas dos jogadores, na mesma transação.
- `encounters` e `combatants`: rodada, turno, iniciativa, PV atual, espaços de magia usados e posição na grade de cada combate.
- `session_events`: cada ação da sessão (dano, cura, magia, XP) vira uma linha que nunca é alterada. É o histórico da mesa e o que o stream manda para os celulares.
- `xp_awards`: quem deu XP, quanto, quando e por quê (MR-016). O modo de XP fica em `campaigns.xp_mode`.
- `scenes` e `scene_actions`: a cena de RP e a lista de ações dela (MR-015).

Os pontos de interesse continuam em `jsonb` dentro do mapa por enquanto, cada um com um campo `revealed`. O servidor filtra os escondidos antes de responder (RN-10). A notificação no app não precisa de tabela no MVP: uma `game_session` sem `ended_at` já é o aviso de "sessão em andamento".

## Tabelas do schema novo, e a equivalente no app antigo

Toda tabela abaixo é nova — nasce numa migration do goose de algum módulo, nenhuma vem de `ALTER` sobre uma tabela existente. A coluna da direita só ajuda quem conhece o app antigo a encontrar a tabela equivalente de lá.

| Tabela | Equivalente no app antigo |
| --- | --- |
| `users` | Parecida com `users` de lá, mas perde `role` (vai para `campaign_members`) |
| `auth_sessions` | Mesma ideia de `auth_sessions` de lá (hash do token) |
| `oauth_handshakes` | Mesma ideia de `oauth_handshakes` de lá |
| `characters` | Parecida com `characters` de lá: `type` vira `kind`, ganha `copied_from_id` e `sheet_locked_at` |
| `maps` | Parecida com `maps` de lá: `user_id` vira `campaign_id`, imagem vira URL do Cloud Storage |
| `campaign_invites` | Substitui a ideia de `character_join_tokens` de lá, por campanha e com token só em hash |
| `campaigns` | Sem equivalente lá |
| `campaign_members` | Sem equivalente lá |
| `campaign_characters` | Sem equivalente lá |
| `character_master_notes` | Sem equivalente lá |
| `gallery_items` | Sem equivalente lá |
| `game_sessions` | Sem equivalente lá |
| `encounters` | Sem equivalente lá |
| `combatants` | Sem equivalente lá |
| `session_events` | Sem equivalente lá |
| `scenes` | Sem equivalente lá |
| `scene_actions` | Sem equivalente lá |
| `xp_awards` | Sem equivalente lá |

## Diagrama: modelo proposto

As colunas abaixo são as citadas no guia mais as chaves primárias e estrangeiras necessárias para o diagrama fechar. O nome exato de cada coluna nova é fechado na migration do goose que a introduz, revisada no PR como qualquer código.

```mermaid
erDiagram
    users {
        uuid id PK
        text google_sub
        text email
        text name
    }

    auth_sessions {
        text token_hash PK
        uuid user_id FK
        timestamptz expires_at
    }

    oauth_handshakes {
        text id PK
        timestamptz expires_at
    }

    campaigns {
        uuid id PK
        text name
        text xp_mode
    }

    campaign_members {
        uuid id PK
        uuid campaign_id FK
        uuid user_id FK
        text role
    }

    campaign_invites {
        uuid id PK
        uuid campaign_id FK
        text token_hash
        timestamptz expires_at
    }

    characters {
        uuid id PK
        text kind
        uuid copied_from_id FK
        timestamptz sheet_locked_at
    }

    campaign_characters {
        uuid id PK
        uuid campaign_id FK
        uuid character_id FK
    }

    character_master_notes {
        uuid id PK
        uuid character_id FK
        text master_notes
    }

    maps {
        uuid id PK
        uuid campaign_id FK
        text name
        text background_image_url
    }

    gallery_items {
        uuid id PK
        uuid campaign_id FK
        text image_url
    }

    game_sessions {
        uuid id PK
        uuid campaign_id FK
        timestamptz started_at
        timestamptz ended_at
    }

    encounters {
        uuid id PK
        uuid game_session_id FK
        uuid map_id FK
        integer round
        integer turn_index
    }

    combatants {
        uuid id PK
        uuid encounter_id FK
        uuid character_id FK
        integer current_hp
        integer initiative
        jsonb spell_slots_used
        jsonb position
    }

    session_events {
        uuid id PK
        uuid game_session_id FK
        text kind
        jsonb payload
        timestamptz created_at
    }

    scenes {
        uuid id PK
        uuid campaign_id FK
        text name
    }

    scene_actions {
        uuid id PK
        uuid scene_id FK
        text label
    }

    xp_awards {
        uuid id PK
        uuid campaign_id FK
        uuid awarded_by FK
        integer amount
        text reason
        timestamptz created_at
    }

    users ||--o{ campaign_members : "participa como"
    users ||--o{ auth_sessions : "autentica"
    users ||--o{ xp_awards : "concede"

    campaigns ||--o{ campaign_members : "tem"
    campaigns ||--o{ campaign_invites : "gera"
    campaigns ||--o{ campaign_characters : "reune"
    campaigns ||--o{ maps : "possui"
    campaigns ||--o{ gallery_items : "guarda"
    campaigns ||--o{ game_sessions : "realiza"
    campaigns ||--o{ scenes : "abre"
    campaigns ||--o{ xp_awards : "registra"

    characters ||--o{ campaign_characters : "entra em"
    characters ||--o| character_master_notes : "tem"
    characters ||--o{ combatants : "atua como"
    characters |o--o| characters : "copiado de"

    game_sessions ||--o{ session_events : "gera"
    game_sessions ||--o{ encounters : "tem"
    maps ||--o{ encounters : "é palco de"

    encounters ||--o{ combatants : "inclui"

    scenes ||--o{ scene_actions : "lista"
```

## Diagrama: tabelas por módulo

```mermaid
flowchart TD
    subgraph identity["Módulo identity"]
        t_users["users"]
        t_auth_sessions["auth_sessions"]
        t_oauth_handshakes["oauth_handshakes"]
    end

    subgraph campaigns_mod["Módulo campaigns"]
        t_campaigns["campaigns"]
        t_campaign_members["campaign_members"]
        t_campaign_invites["campaign_invites"]
    end

    subgraph characters_mod["Módulo characters"]
        t_characters["characters"]
        t_campaign_characters["campaign_characters"]
        t_character_master_notes["character_master_notes"]
    end

    subgraph play["Módulo play"]
        t_game_sessions["game_sessions"]
        t_encounters["encounters"]
        t_combatants["combatants"]
        t_session_events["session_events"]
        t_scenes["scenes"]
        t_scene_actions["scene_actions"]
    end

    subgraph maps_mod["Módulo maps"]
        t_maps["maps"]
        t_gallery_items["gallery_items"]
    end

    subgraph progression["Módulo progression"]
        t_xp_awards["xp_awards"]
    end
```

## Ver também

- [Glossário](produto/glossario.md)
- [Regras de negócio](produto/regras.md)
- [Arquitetura](arquitetura.md): os módulos donos de cada tabela.
- `server/src/db/schema.sql`: o schema do app antigo (descontinuado), só como referência histórica.
