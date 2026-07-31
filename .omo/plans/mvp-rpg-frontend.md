# MVP Frontend: Sistema de Gerenciamento de RPG (Angular)

## TL;DR

> **Quick Summary**: Construir um MVP frontend Angular 21 para gerenciamento de campanhas de RPG, com 6 módulos lazy-loaded, Angular Material + CDK, RxJS state, localStorage + Repository Pattern.
>
> **Deliverables**:
> - Módulo Personagens (CRUD, fichas, rich text, notas, falas)
> - Módulo Campanha (árvore de pastas com drag-drop, associação de entidades)
> - Módulo Galeria (upload, grid, lightbox, audio player)
> - Módulo Regras (upload PDF, leitor com índice e paginação)
> - Módulo Mapa (OpenLayers 2D + Three.js 2.5D, layers, submapas)
> - Módulo Sessão (split-screen mestre, toolbar atalhos, busca rápida)
> - Global Search (Ctrl+K), Sidebar de navegação
> - Testes (Vitest + Angular Testing Library)
>
> **Estimated Effort**: XL
> **Parallel Execution**: YES — 5 waves + Final verification
> **Critical Path**: Task 1 (setup) → Task 3 (core) → Task 4 (shell) → Waves 2-4 → Task F1-F4

---

## Context

### Original Request
Criar um MVP frontend para um Sistema de Gerenciamento de RPG, com análise completa de requisitos (RF-01 a RF-22), lazy loading, gerenciamento de estado reativo e busca global.

### Interview Summary
**Key Discussions**:
- **UI Library**: Angular Material 21.2 + CDK (provê mat-tab-group, mat-dialog, mat-tree, drag-drop)
- **State**: RxJS + BehaviorSubjects com Repository Pattern para localStorage
- **Rich Text**: ngx-quill v30.1 (v31 requer Angular 22)
- **Mapa**: OpenLayers 2D + Three.js 2.5D (projeção isométrica, sem 3D completo)
- **PDF**: ng2-pdf-viewer 10.x com bookmarks e paginação
- **Testes**: Vitest + @testing-library/angular 19
- **Persistência**: localStorage para dados, IndexedDB + Object URLs para binários
- **Device**: Desktop-first com responsivo para tablet
- **Campanha**: Arrastar entidade = cria referência (não move/copia)
- **Upload**: Máx 10MB, tipos: png/jpg/gif/webp/svg + mp3/ogg/wav + .pdf

**Research Findings**:
- Angular 21.2.19, standalone components, SCSS. Projeto limpo sem Angular Material ou router.
- Todas as libs verificadas quanto à compatibilidade com Angular 21.

---

## Work Objectives

### Core Objective
Construir MVP Angular 21 de Gerenciamento de Campanhas RPG com 6 módulos lazy-loaded.

### Concrete Deliverables
- 6 feature modules (Characters, Campaign, Gallery, Rules, Map, Session)
- App shell com sidebar e Global Search (Ctrl+K)
- Testes unitários com Vitest
- Dados persistidos em localStorage/IndexedDB

### Definition of Done
- [ ] `ng serve` → app carrega com sidebar e home
- [ ] Navegação lazy entre todos os 6 módulos
- [ ] CRUD Personagens funcional com todas as abas
- [ ] Árvore Campanha com drag-drop e associação funcional
- [ ] Galeria com upload e visualização
- [ ] Regras com upload PDF e leitor integrado
- [ ] Mapa com OpenLayers + toggle 2.5D
- [ ] Sessão com split-screen e toolbar
- [ ] Ctrl+K abre busca global
- [ ] Testes passando: `ng test`
- [ ] Build produção: `ng build --configuration production` sem erros
- [ ] Bundle budgets: initial ≤ 3MB

### Must Have
- Lazy loading em todos os 6 módulos
- Global Search (Ctrl+K) acessível de qualquer tela
- Sidebar de navegação entre módulos
- Persistência localStorage + IndexedDB para binários
- Abas no detalhe do personagem (História, Ficha, Notas Mestre, Falas)
- FormArray para atributos dinâmicos na ficha
- Formulário de personagem com validação
- Upload de imagem com preview e crop
- Rich Text Editor (Quill) na aba História
- Árvore de pastas com drag-drop (CDK) na Campanha
- Upload de PDF com restrição de tipo
- Leitor de PDF com bookmarks e paginação
- Mapa com OpenLayers (zoom, pan, markers)
- Toggle 2.5D com Three.js
- Camadas configuráveis (grid, fog of war, markers do mestre)
- Submapas via pins
- Split-screen na Sessão (entidade + referência)
- Toolbar com atalhos na Sessão
- Modal de busca rápida na Sessão
- Testes: Vitest + Testing Library configurados
- TDD para CRUD Personagens e árvore Campanha
- Repository Pattern para troca futura para backend

### Must NOT Have (Guardrails)
- Dashboard (RF-21 a RF-22) — postergado pós-MVP
- Autenticação/login — role "Mestre" é flag booleana no estado
- Backend/API — 100% client-side
- Construtor de fichas genérico — campos pré-definidos
- Playlists de áudio — player de faixa única
- Full-text search — busca apenas nomes e tags
- 3D real — apenas projeção 2.5D isométrica
- Cross-module direct imports — dados fluem via core/
- npm packageManager version mismatch — verificado compatível

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: NO (precisa configurar)
- **Automated tests**: YES — TDD para Personagens CRUD e Campanha tree; tests-after para demais
- **Framework**: Vitest + @testing-library/angular
- **TDD**: Task de Personagens segue RED (escreve teste) → GREEN (implementa) → REFACTOR

### QA Policy
Every task MUST include agent-executed QA scenarios.
- **UI/Component**: Playwright + `ng serve` — navigate, interact, assert DOM, screenshot
- **Service/Logic**: Vitest — run tests, assert behavior
- **Build**: `ng build --configuration production` — verify no errors, check budgets
- **Evidence**: `.omo/evidence/task-{N}-{scenario}.{ext}`

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — sequential, unblocks everything):
├── Task 1: Project setup + dependency installation [quick]
├── Task 2: Test infrastructure (Vitest + Testing Library) [quick]
├── Task 3: Core infrastructure (models, store, repo, router) [deep]
├── Task 4: App shell (layout, sidebar, global search modal) [visual-engineering]
└── Task 5: Shared UI components [visual-engineering]

Wave 2 (Feature foundations — MAX PARALLEL, 5 tasks):
├── Task 6: Characters — routing + list view [visual-engineering]
├── Task 7: Campaign — tree view + drag-drop [visual-engineering]
├── Task 8: Gallery — dropzone + thumbnail grid [visual-engineering]
├── Task 9: Map — OpenLayers integration [unspecified-high]
└── Task 10: Rules — upload + PDF viewer setup [unspecified-high]

Wave 3 (Feature depth — MAX PARALLEL, 6 tasks):
├── Task 11: Characters — detail with tabs + rich text [deep]
├── Task 12: Characters — ficha (FormArray, atributos, perícias, inventário) [deep]
├── Task 13: Campaign — entity association [visual-engineering]
├── Task 14: Gallery — lightbox + audio player [visual-engineering]
├── Task 15: Map — layers panel (grid, fog of war, markers) [unspecified-high]
└── Task 16: Map — 2.5D Three.js toggle [unspecified-high]

Wave 4 (Completion — MAX PARALLEL, 5 tasks):
├── Task 17: Characters — image upload/crop + master notes + quotes [visual-engineering]
├── Task 18: Map — submap pins [unspecified-high]
├── Task 19: Rules — leitor (bookmarks, paginação, busca) [unspecified-high]
├── Task 20: Session — split-screen layout + toolbar [visual-engineering]
└── Task 21: Session — quick search dialog [visual-engineering]

Wave 5 (Integration):
├── Task 22: Global search (Ctrl+K) integration [deep]
├── Task 23: Polish, error states, empty states, loading states [visual-engineering]
└── Task 24: npm scripts + build optimization [quick]

Wave FINAL (After ALL tasks — 4 parallel reviews, then user okay):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real manual QA (unspecified-high)
└── Task F4: Scope fidelity check (deep)
-> Present results -> Get explicit user okay

Critical Path: Task 1 → Task 3 → Task 4 → Tasks 6-10 → Tasks 11-16 → Tasks 17-21 → Task 22-24 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 6 (Waves 2-4)
```

### Dependency Matrix
- **1-5**: — — 6-10, 1
- **6**: 3,4,5 — 11,12, 2
- **7**: 3,4,5 — 13, 2
- **8**: 3,4,5 — 14, 2
- **9**: 3,4,5 — 15,16,18, 2
- **10**: 3,4,5 — 19, 2
- **11,12**: 6 — 17, 3
- **13**: 7 — 4
- **14**: 8 — 4
- **15,16**: 9 — 18, 3
- **17**: 11,12 — 22, 4
- **18**: 15,16 — 22, 4
- **19**: 10 — 4
- **20,21**: 3,4 — 22, 4
- **22**: 17,18,20,21 — 23, 5
- **23**: 22 — 24, 5
- **24**: 22,23 — F1-F4, 5

---

## TODOs

- [x] 1. **Project Setup + Dependency Installation**

  **What to do**:
  - Instalar dependências: `@angular/material@21.2`, `@angular/cdk`, `ngx-quill@30`, `quill`, `ol`, `three`, `ng2-pdf-viewer`, `pdfjs-dist`
  - Configurar `angular.json`: aumentar budgets para `"maximumWarning": "2MB", "maximumError": "3MB"`
  - Configurar `angular.json`: adicionar `"allowedCommonJsDependencies": ["quill", "pdfjs-dist"]`
  - Atualizar `index.html` com lang="pt-BR"

  **Must NOT do**:
  - Não instalar ngx-quill@31 (incompatível com Angular 21)
  - Não alterar standalone component config

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: N/A

  **Parallelization**: Can Run In Parallel: NO | Blocks: Tasks 2-24 | Blocked By: None

  **Acceptance Criteria**:
  - [ ] `npm install` completa sem erros
  - [ ] `ng build` compila com sucesso
  - [ ] Budget aumento verificado em angular.json

  **QA Scenarios**:
  ```
  Scenario: Build bem-sucedido
    Tool: Bash
    Steps:
      1. Run `npx ng build --configuration production`
    Expected Result: Exit code 0, sem erros, mensagem "Application bundle generation complete"
    Evidence: .omo/evidence/task-1-build.log

  Scenario: Budget atualizado
    Tool: Bash
    Steps:
      1. Grep angular.json por "maximumWarning" e "maximumError"
    Expected Result: initial maximumWarning = "2MB", maximumError = "3MB"
    Evidence: .omo/evidence/task-1-budget.log
  ```

  **Commit**: YES
  - Message: `chore(deps): install Angular Material, Quill, OpenLayers, Three.js, ng2-pdf-viewer`
  - Files: `package.json`, `angular.json`, `index.html`

---

- [x] 2. **Test Infrastructure (Vitest + Angular Testing Library)**

  **What to do**:
  - Instalar `vitest`, `@testing-library/angular`, `@testing-library/dom`, `jsdom`
  - Criar `vitest.config.ts` com preset Angular + jsdom
  - Criar `tsconfig.spec.json` com referência ao vitest
  - Adicionar script `"test": "vitest"` e `"test:watch": "vitest --watch"` em `package.json`
  - Criar exemplo de teste para AppComponent e HomeComponent
  - Atualizar `angular.json` schematics para não pular testes

  **Must NOT do**:
  - Não remover configuração existente do Angular CLI test (não há)
  - Não configurar Karma ou Jasmine

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Configuração de test runner com Angular requer conhecimento de ferramentas
  - **Skills**: N/A

  **Parallelization**: Can Run In Parallel: NO | Blocks: Tasks with TDD (6, 7, 11, 13) | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] `npx vitest run` passa (pelo menos 1 teste de exemplo)
  - [ ] Exemplo de teste com `@testing-library/angular` renderiza componente
  - [ ] `tsconfig.spec.json` existe com paths corretos

  **QA Scenarios**:
  ```
  Scenario: Teste de exemplo passa
    Tool: Bash
    Preconditions: Vitest configurado
    Steps:
      1. Run `npx vitest run`
    Expected Result: Exit code 0, pelo menos 1 test suite passed
    Evidence: .omo/evidence/task-2-vitest.log

  Scenario: Testing Library funcional
    Tool: Bash
    Preconditions: Vitest configurado
    Steps:
      1. Grep vitest.config.ts por "jsdom"
    Expected Result: jsdom está configurado como environment
    Evidence: .omo/evidence/task-2-jsdom.log
  ```

  **Commit**: YES (group with 1)
  - Message: `chore(test): configure Vitest + Angular Testing Library`
  - Files: `vitest.config.ts`, `tsconfig.spec.json`, `package.json`

---

- [x] 3. **Core Infrastructure (Models, Store, Repositories, Router)**

  **What to do**:
  - Criar interfaces/models em `src/app/core/models/`:
    - `character.ts`: Character (id, name, description, type, imageUrl, history, masterNotes, attributes, skills, inventory, quotes, createdAt, updatedAt)
    - `campaign.ts`: CampaignFolder (id, name, parentId, children, entityIds: {characterIds, mapIds, sessionIds})
    - `gallery.ts`: GalleryItem (id, name, type, url, thumbnailUrl, folderId, size, createdAt)
    - `map.ts`: MapData, MapLayer (grid, fogOfWar, markers), SubmapPin
    - `session.ts`: SessionState (activeEntity, quickReference, toolbarShortcuts)
    - `rules.ts`: RuleBook (id, name, pdfUrl, description, createdAt)
    - `shared.ts`: EntityType, SearchResult, SidebarItem
  - Criar `src/app/core/store/` com StoreService base:
    - Generic BehaviorSubject store com get/set/update/patch
    - Event/notification pattern para cross-module updates
  - Criar `src/app/core/repositories/` com Repository Pattern:
    - `BaseRepository<T>` interface (getAll, getById, create, update, delete)
    - `LocalStorageRepository<T>` implementação concreta
    - `IndexedDbFileRepository` para arquivos binários
  - Criar `src/app/core/services/`:
    - `SearchService`: indexed search across entities (name + tags)
    - `SidebarService`: sidebar items state
    - `PersistenceService`: init + sync localStorage/IndexedDB on app start
  - Criar `src/app/app.routes.ts` com lazy loading routes:
    - `/personagens` → loadComponent `() => import('./features/characters')`
    - `/campanha` → loadComponent...
    - `/galeria` → loadComponent...
    - `/regras` → loadComponent...
    - `/sessao` → loadComponent...
    - `/mapa` → loadComponent...
    - `**` → redirect to home
  - Configurar `provideRouter()` + `withComponentInputBinding()` + `withRouterConfig()` em `app.config.ts`

  **Must NOT do**:
  - Não implementar HTTP services — usar localStorage
  - Não criar NgRx ou signals state — manter BehaviorSubjects

  **Pattern References**:
  - Angular standalone routing: `https://angular.dev/guide/routing/common-router-tasks#lazy-loading`

  **Parallelization**: Can Run In Parallel: NO | Blocks: Tasks 4-24 | Blocked By: Task 1

  **Acceptance Criteria**:
  - [ ] `ng serve` → navegação entre rotas funciona (lazy modules carregados)
  - [ ] StoreService aceita get/set/update/patch
  - [ ] LocalStorageRepository salva e recupera dados entre reloads
  - [ ] SearchService indexa entidades e retorna resultados
  - [ ] Lazy loading confirmado via network tab (chunks separados)

  **QA Scenarios**:
  ```
  Scenario: Lazy loading funcional
    Tool: Bash + Playwright
    Steps:
      1. Start `npx ng serve` in background
      2. Navigate to /personagens
    Expected Result: Route carrega, chunk separado baixado
    Evidence: .omo/evidence/task-3-lazy-load.log
  ```

  **Commit**: YES (group with 1, 2)
  - Message: `feat(core): add models, store, repositories, and router with lazy loading`
  - Files: `src/app/core/**`, `src/app/app.routes.ts`, `src/app/app.config.ts`

---

- [x] 4. **App Shell (Layout, Sidebar, Global Search Modal)**

  **What to do**:
  - Criar `src/app/layout/shell.component.ts` — layout principal:
    - `<mat-sidenav-container>` com `<mat-sidenav>` (sidebar) + `<mat-sidenav-content>` (router-outlet)
    - `<mat-toolbar>` no topo com:
      - Menu hamburger toggle sidebar
      - Título "MeuRPG"
      - Botão de busca (Ctrl+K)
    - Sidebar com `<mat-nav-list>` de links:
      - 📖 Personagens → /personagens
      - 🗺️ Mapa → /mapa
      - 📂 Campanha → /campanha
      - 🖼️ Galeria → /galeria
      - 📜 Regras → /regras
      - 🎮 Sessão → /sessao
  - Criar `src/app/shared/search-modal/search-modal.component.ts`:
    - `<mat-dialog>` aberto via Ctrl+K ou clique no botão
    - Input de busca com debounce
    - Lista de resultados via SearchService
    - Ao selecionar, navega para o item
    - Fechar com Escape
  - Atualizar `app.ts` para usar ShellComponent
  - Registrar `ShellComponent` nas rotas como componente pai com `<router-outlet>`

  **Must NOT do**:
  - Não implementar busca full-text (apenas nomes e tags)
  - Não animar transições de rota (pode ser adicionado depois)

  **Pattern References**:
  - `src/app/home.component.ts` — componente standalone existente para referência de padrão

  **Parallelization**: Can Run In Parallel: NO | Blocks: Tasks 6-21 | Blocked By: Task 3

  **Acceptance Criteria**:
  - [ ] Sidebar aparece com todos os links
  - [ ] Clicar nos links navega via lazy loading
  - [ ] Ctrl+K abre modal de busca
  - [ ] Escape fecha modal de busca
  - [ ] Resultados de busca aparecem ao digitar

  **QA Scenarios**:
  ```
  Scenario: Sidebar navegação
    Tool: Playwright
    Steps:
      1. Load page
      2. Click "Personagens" na sidebar
    Expected Result: URL muda para /personagens
    Evidence: .omo/evidence/task-4-sidebar-nav.png

  Scenario: Ctrl+K abre busca
    Tool: Playwright
    Steps:
      1. Press Control+K
    Expected Result: Modal de busca aparece com input focado
    Evidence: .omo/evidence/task-4-search-modal.png

  Scenario: Escape fecha busca
    Tool: Playwright
    Steps:
      1. Press Control+K
      2. Press Escape
    Expected Result: Modal de busca fechado
    Evidence: .omo/evidence/task-4-search-close.png
  ```

  **Commit**: YES
  - Message: `feat(shell): add app layout with sidebar and global search modal`
  - Files: `src/app/layout/**`, `src/app/shared/search-modal/**`, `src/app/app.ts`

---

- [x] 5. **Shared UI Components**

  **What to do**:
  - Criar componentes compartilhados em `src/app/shared/components/`:
    - `confirm-dialog.component.ts` — MatDialog de confirmação reutilizável
    - `empty-state.component.ts` — Estado vazio com ícone + mensagem + ação
    - `loading-spinner.component.ts` — Spinner de carregamento
    - `file-upload.component.ts` — Dropzone com drag-drop, validação de tipo e tamanho
    - `audio-player.component.ts` — Player de áudio (play/pause, seek, volume, loop)
    - `image-crop.component.ts` — Crop de imagem com canvas
    - `page-header.component.ts` — Título + breadcrumbs + ações da página
  - Exportar do barrel `src/app/shared/index.ts`

  **Must NOT do**:
  - Não implementar funcionalidades específicas de módulo (só genéricos)

  **Pattern References**:
  - Angular Material component examples

  **Parallelization**: Can Run In Parallel: NO | Blocks: Tasks 6-21 | Blocked By: Task 3

  **Acceptance Criteria**:
  - [ ] Cada componente renderiza sem erros
  - [ ] FileUpload aceita/configura tipos MIME e maxSize
  - [ ] AudioPlayer toca áudio com controles

  **QA Scenarios**:
  ```
  Scenario: EmptyState renderiza
    Tool: Vitest
    Steps:
      1. Render EmptyStateComponent com @testing-library/angular
      2. Check mensagem aparece
    Expected Result: Componente renderizado com texto
    Evidence: .omo/evidence/task-5-empty-state.log
  ```

  **Commit**: YES (group with 4)
  - Message: `feat(shared): add shared UI components (dialog, empty-state, upload, audio, crop)`
  - Files: `src/app/shared/components/**`

---

- [x] 6. **Characters — Routing + List View (TDD)**

  **What to do**:
  - Criar `src/app/features/characters/` como módulo lazy
  - Criar `character-list.component.ts`:
    - Grade de cards com `<mat-card>` por personagem
    - Badge de tipo (NPC/Jogador/Boss) com `<mat-chip>` colorido
    - Avatar (imagem ou placeholder)
    - Botão "Novo Personagem" → navega para criação
  - Filtros rápidos com `<mat-chip-listbox>`: Todos | NPC | Jogador | Boss
  - Barra de busca por nome
  - **TDD**: Escrever teste primeiro (RED), depois implementar (GREEN)

  **Must NOT do**:
  - Não implementar detalhe ainda (só lista)
  - Não implementar formulário de criação (só rota)

  **Parallelization**: Can Run In Parallel: YES | Wave 2 (with 7, 8, 9, 10) | Blocks: Tasks 11, 12, 17 | Blocked By: Tasks 3, 4, 5

  **Acceptance Criteria**:
  - [ ] `/personagens` carrega com lazy chunk
  - [ ] Lista de cards renderiza personagens do store
  - [ ] Filtro por tipo funciona (chips)
  - [ ] Busca por nome filtra resultados
  - [ ] TDD: Teste escrito ANTES da implementação (verificar git log)

  **QA Scenarios**:
  ```
  Scenario: Lista de personagens carrega
    Tool: Playwright
    Steps:
      1. Navigate to /personagens
    Expected Result: Cards de personagens visíveis (ou empty state)
    Evidence: .omo/evidence/task-6-characters-list.png

  Scenario: Filtro por tipo funciona
    Tool: Playwright
    Steps:
      1. Click chip "NPC"
    Expected Result: Lista filtrada apenas por NPCs
    Evidence: .omo/evidence/task-6-filter-chip.png
  ```

  **Commit**: YES
  - Message: `feat(characters): add list view with type filters (TDD)`
  - Files: `src/app/features/characters/**`

---

- [x] 7. **Campaign — Tree View + Drag-Drop (TDD)**

  **What to do**:
  - Criar `src/app/features/campaign/` como módulo lazy
  - Criar `campaign-tree.component.ts`:
    - `<mat-tree>` com dados aninhados infinitos
    - CDK drag-drop: reordenar pastas, aninhamento
    - Botões: Nova Pasta (raiz), Renomear (inline edit), Excluir (com confirmação)
    - Indicador de profundidade visual (indent + linhas conectoras)
    - Ao clicar em pasta: exibe conteúdo no painel ao lado
  - Criar `folder-content.component.ts`:
    - Lista de entidades associadas (personagens, mapas, sessões)
    - Botão "Associar" abre seletor de entidade
  - **TDD**: Escrever teste primeiro para CRUD de pastas

  **Must NOT do**:
  - Não implementar navegação por breadcrumbs (só tree)
  - Não limitar profundidade de aninhamento

  **Parallelization**: Can Run In Parallel: YES | Wave 2 (with 6, 8, 9, 10) | Blocks: Task 13 | Blocked By: Tasks 3, 4, 5

  **Acceptance Criteria**:
  - [ ] `/campanha` carrega lazy
  - [ ] Criação de pasta funciona
  - [ ] Drag-drop reordena pastas
  - [ ] Subpastas podem ser criadas
  - [ ] Exclusão de pasta com confirmação
  - [ ] TDD: Teste para CRUD de pasta escrito primeiro

  **QA Scenarios**:
  ```
  Scenario: Criação de pasta
    Tool: Playwright
    Steps:
      1. Click "Nova Pasta"
      2. Type "Missão 1"
      3. Press Enter
    Expected Result: Pasta "Missão 1" aparece na árvore
    Evidence: .omo/evidence/task-7-folder-create.png

  Scenario: Drag-drop reordena
    Tool: Playwright
    Steps:
      1. Drag pasta "Missão 1" para dentro de "Campanha"
    Expected Result: Pasta move para dentro, indentação aumenta
    Evidence: .omo/evidence/task-7-drag-drop.png
  ```

  **Commit**: YES
  - Message: `feat(campaign): add tree view with CDK drag-drop (TDD)`
  - Files: `src/app/features/campaign/**`

---

- [x] 8. **Gallery — Dropzone + Thumbnail Grid**

  **What to do**:
  - Criar `src/app/features/gallery/` como módulo lazy
  - Criar `gallery-upload.component.ts`:
    - Dropzone com `@angular/cdk/drag-drop` para arquivos
    - Validação: tipos (png, jpg, gif, webp, svg, mp3, ogg, wav), max 10MB
    - Barra de progresso de upload (simulada, pois é client-side)
    - Preview do arquivo antes de confirmar
  - Criar `gallery-grid.component.ts`:
    - Grid responsivo com thumbnails
    - Imagens: thumbnail gerada via canvas (max 200px)
    - Áudio: ícone de espectro
    - Ao clicar: abre lightbox (Task 14) ou audio player
    - Ordenação: nome, data, tipo
  - Integrar com IndexedDbFileRepository para salvar binários
  - Reaproveitar CampaignTree como seletor de pasta (opcional no MVP)

  **Must NOT do**:
  - Não gerar thumbnails server-side
  - Não implementar edição de imagem

  **Parallelization**: Can Run In Parallel: YES | Wave 2 (with 6, 7, 9, 10) | Blocks: Task 14 | Blocked By: Tasks 3, 4, 5

  **Acceptance Criteria**:
  - [ ] `/galeria` carrega lazy
  - [ ] Drag-drop de arquivo abre preview
  - [ ] Upload de imagem inválida (exe, >10MB) mostra erro
  - [ ] Thumbnails aparecem no grid
  - [ ] Áudio mostra ícone de espectro

  **QA Scenarios**:
  ```
  Scenario: Upload de imagem válida
    Tool: Playwright
    Steps:
      1. Drop arquivo "mapa-dungeon.png" na dropzone
    Expected Result: Preview aparece, thumbnail gerada
    Evidence: .omo/evidence/task-8-upload-preview.png

  Scenario: Upload de arquivo inválido
    Tool: Playwright
    Steps:
      1. Drop "virus.exe" na dropzone
    Expected Result: Mensagem de erro "Tipo de arquivo não permitido"
    Evidence: .omo/evidence/task-8-upload-error.png
  ```

  **Commit**: YES
  - Message: `feat(gallery): add dropzone upload and thumbnail grid`
  - Files: `src/app/features/gallery/**`

---

- [x] 9. **Map — OpenLayers Integration**

  **What to do**:
  - Criar `src/app/features/map/` como módulo lazy
  - Criar `map-view.component.ts`:
    - Inicializar mapa OpenLayers em container div
    - Configurar view (zoom, center, projection)
    - Adicionar tile layer (OSM ou similar)
    - Botão fullscreen
  - Criar `map.service.ts` (serviço local do módulo):
    - Wrapper para operações OpenLayers
    - Gerenciar layers (base, grid, markers, fog)
    - Zoom, pan, center control
  - **IMPORTANTE**: OpenLayers deve ser importado dinamicamente (nunca no bundle inicial)
    ```ts
    const ol = await import('ol');
    const Map = await import('ol/Map');
    ```

  **Must NOT do**:
  - Não implementar Three.js ainda (Task 16)
  - Não implementar camadas configuráveis (Task 15)
  - OL nunca no bundle inicial (dynamic import obrigatório)

  **Parallelization**: Can Run In Parallel: YES | Wave 2 (with 6, 7, 8, 10) | Blocks: Tasks 15, 16, 18 | Blocked By: Tasks 3, 4, 5

  **Acceptance Criteria**:
  - [ ] `/mapa` carrega lazy
  - [ ] OpenLayers não aparece no chunk inicial (verificar bundle)
  - [ ] Mapa renderiza com tile layer
  - [ ] Zoom e pan funcionam
  - [ ] Botão fullscreen alterna

  **QA Scenarios**:
  ```
  Scenario: Mapa renderiza
    Tool: Playwright
    Steps:
      1. Navigate to /mapa
    Expected Result: Mapa OpenLayers visível com tiles
    Evidence: .omo/evidence/task-9-map-render.png

  Scenario: Dynamic import confirmado
    Tool: Bash
    Steps:
      1. Run `npx ng build --configuration production --stats-json`
      2. Grep stats.json por "ol"
    Expected Result: "ol" NÃO aparece no chunk inicial (initial.js)
    Evidence: .omo/evidence/task-9-dynamic-import.log
  ```

  **Commit**: YES
  - Message: `feat(map): add OpenLayers map with dynamic import`
  - Files: `src/app/features/map/**`

---

- [x] 10. **Rules — Upload + PDF Viewer Setup**

  **What to do**:
  - Criar `src/app/features/rules/` como módulo lazy
  - Criar `rules-list.component.ts`:
    - Lista de PDFs com nome, descrição, data de upload
    - Botão "Adicionar Livro" → upload restrito a .pdf
    - Reaproveitar FileUploadComponent com validação de tipo .pdf
  - Criar `rules-reader.component.ts`:
    - `<ng2-pdf-viewer>` para renderizar PDF
    - Configurar `[pdfSrc]` com URL do arquivo
    - Configurar `pdfWorkerSrc` em app.config.ts
    - Botões de navegação: anterior/próximo página
    - Input de página atual: `[page]` / `(pageChange)`
  - Integrar com IndexedDbFileRepository para salvar PDFs

  **Must NOT do**:
  - Não implementar busca no conteúdo do PDF
  - Não implementar bookmarks ainda (Task 19)

  **Parallelization**: Can Run In Parallel: YES | Wave 2 (with 6, 7, 8, 9) | Blocks: Task 19 | Blocked By: Tasks 3, 4, 5

  **Acceptance Criteria**:
  - [ ] `/regras` carrega lazy
  - [ ] Upload de .pdf funciona
  - [ ] Upload de não-.pdf mostra erro
  - [ ] Lista de PDFs aparece
  - [ ] pdfWorkerSrc configurado em app.config.ts

  **QA Scenarios**:
  ```
  Scenario: Upload de PDF válido
    Tool: Playwright
    Steps:
      1. Upload arquivo "manual-dnd.pdf"
    Expected Result: PDF aparece na lista
    Evidence: .omo/evidence/task-10-pdf-upload.png

  Scenario: Upload de arquivo não-PDF
    Tool: Playwright
    Steps:
      1. Upload "notas.txt"
    Expected Result: Erro "Apenas arquivos .pdf são permitidos"
    Evidence: .omo/evidence/task-10-pdf-error.png
  ```

  **Commit**: YES
  - Message: `feat(rules): add PDF upload and ng2-pdf-viewer setup`
  - Files: `src/app/features/rules/**`

---

- [x] 11. **Characters — Detail View with Tabs + Rich Text (TDD)**

  **What to do**:
  - Criar `character-detail.component.ts`:
    - Layout de cabeçalho: avatar, nome, descrição, tipo (chip)
    - `<mat-tab-group>` com 4 abas:
      - **História** (RF-03): Editor Quill (`<quill-editor>`)
      - **Ficha** (RF-04): Placeholder para Task 12
      - **Notas do Mestre** (RF-05): Placeholder para Task 17
      - **Falas** (RF-06): Placeholder para Task 17
  - Aba História:
    - ngx-quill `<quill-editor>` com toolbar básico (bold, italic, lists, headers)
    - Sanitize config para segurança
    - Salvamento automático com debounce
  - Breadcrumbs: Personagens > Nome do Personagem
  - Botão Voltar para lista
  - **TDD**: Teste para renderização das abas e do Quill editor

  **Must NOT do**:
  - Não implementar formulário de ficha aqui (Task 12)
  - Quill não aparecer no bundle inicial (lazy module already)

  **Parallelization**: Can Run In Parallel: YES | Wave 3 (with 12, 13, 14, 15, 16) | Blocks: Task 17 | Blocked By: Task 6

  **Acceptance Criteria**:
  - [ ] `/personagens/:id` carrega detalhe
  - [ ] 4 abas visíveis no mat-tab-group
  - [ ] Aba História tem Quill editor funcional
  - [ ] Salvamento automático com debounce
  - [ ] TDD: Teste escrito primeiro

  **QA Scenarios**:
  ```
  Scenario: Abas do personagem
    Tool: Playwright
    Steps:
      1. Navigate to /personagens/1 (create via store se vazio)
      2. Click aba "História"
    Expected Result: Quill editor aparece com toolbar
    Evidence: .omo/evidence/task-11-tab-history.png

  Scenario: Quill editor salva
    Tool: Playwright
    Steps:
      1. Type "Aragorn, filho de Arathorn" no Quill
      2. Wait 2s (debounce)
      3. Reload page
    Expected Result: Texto persiste após reload
    Evidence: .omo/evidence/task-11-quill-save.png
  ```

  **Commit**: YES
  - Message: `feat(characters): add detail view with tabs and Quill rich text`
  - Files: `src/app/features/characters/**`

---

- [x] 12. **Characters — Character Sheet (FormArray FORM/DES/CON/INT/SAB/CAR + Skills + Inventory)**

  **What to do**:
  - Criar `character-sheet.component.ts`:
    - Formulário com ReactiveFormsModule
    - Seção Atributos (6 inputs numéricos): FOR, DES, CON, INT, SAB, CAR
      - Cada atributo: `<mat-form-field>` type="number", min=1, max=30
      - Bônus calculado automaticamente: `Math.floor((value - 10) / 2)`
    - Seção Perícias (FormArray):
      - Cada perícia: nome + bônus + atributo associado (select)
      - Botão "Adicionar Perícia", "Remover"
      - Input autocomplete para perícias comuns (Acrobacia, Atletismo, Percepção, etc)
    - Seção Inventário (FormArray):
      - Cada item: nome, quantidade, peso, descrição
      - Botões "Adicionar Item", "Remover"
    - Botão Salvar (salva no store + localStorage)
    - Validação: atributos obrigatórios, nome da perícia obrigatório

  **Must NOT do**:
  - Não implementar template builder (campos fixos)
  - Não calcular nível ou XP (só atributos e perícias)

  **Parallelization**: Can Run In Parallel: YES | Wave 3 (with 11, 13, 14, 15, 16) | Blocks: Task 17 | Blocked By: Task 6

  **Acceptance Criteria**:
  - [ ] Formulário com 6 inputs de atributo
  - [ ] Bônus calculado automaticamente
  - [ ] Adicionar/remover perícias funciona
  - [ ] Adicionar/remover itens de inventário funciona
  - [ ] Salvamento persiste dados

  **QA Scenarios**:
  ```
  Scenario: Formulário de atributos
    Tool: Playwright
    Steps:
      1. Set FOR=18
    Expected Result: Bônus calculado = +4
    Evidence: .omo/evidence/task-12-attribute-bonus.png

  Scenario: Adicionar perícia
    Tool: Playwright
    Steps:
      1. Click "Adicionar Perícia"
      2. Type "Acrobacia"
      3. Select "DES" como atributo
      4. Set bônus = 5
    Expected Result: Perícia aparece na lista
    Evidence: .omo/evidence/task-12-add-skill.png
  ```

  **Commit**: YES (group with 11)
  - Message: `feat(characters): add character sheet with FormArray (attributes, skills, inventory)`
  - Files: `src/app/features/characters/**`

---

- [x] 13. **Campaign — Entity Association UI**

  **What to do**:
  - Criar `entity-selector-dialog.component.ts`:
    - `<mat-dialog>` para selecionar entidades (personagens, mapas, sessões)
    - Abas ou chips para filtrar por tipo
    - Lista selecionável com checkboxes
    - Ao confirmar: adiciona referência na pasta
  - Atualizar `folder-content.component.ts`:
    - Exibir entidades associadas: nome + tipo + link para navegar
    - Botão "Desassociar" com confirmação
    - Drag-drop de entidades da lista para outras pastas (cria referência, não move)
  - Integrar com CampaignStore para persistência

  **Must NOT do**:
  - Não mover entidade ao arrastar (criar referência)
  - Não limitar número de associações por pasta

  **Parallelization**: Can Run In Parallel: YES | Wave 3 (with 11, 12, 14, 15, 16) | Blocked By: Task 7

  **Acceptance Criteria**:
  - [ ] Clicar "Associar" abre dialog com entidades
  - [ ] Selecionar entidade adiciona referência na pasta
  - [ ] Link da entidade navega para o módulo correto
  - [ ] Desassociar remove referência

  **QA Scenarios**:
  ```
  Scenario: Associar personagem à pasta
    Tool: Playwright
    Steps:
      1. Create personagem "Aragorn"
      2. Navigate to /campanha
      3. Click pasta "Missão 1"
      4. Click "Associar"
      5. Select "Aragorn"
      6. Click Confirmar
    Expected Result: "Aragorn" aparece na lista de entidades da pasta
    Evidence: .omo/evidence/task-13-associate-entity.png
  ```

  **Commit**: YES
  - Message: `feat(campaign): add entity association dialog and folder content`
  - Files: `src/app/features/campaign/**`

---

- [x] 14. **Gallery — Lightbox + Audio Player**

  **What to do**:
  - Criar `gallery-lightbox.component.ts`:
    - Modal fullscreen com overlay escuro
    - Imagem centralizada com `max-width: 90vw`
    - Navegação: teclado (setas esquerda/direita), botões prev/next
    - Fechar: Escape, click outside, botão X
    - Transição suave entre imagens
    - Contador: "3 / 15"
  - Criar `gallery-audio-player.component.ts` (ou usar shared AudioPlayer):
    - Reutilizar `audio-player.component.ts` do shared
    - Lista de arquivos de áudio com play/pause
    - Highlight no áudio atualmente tocando
    - Ao finalizar, passa para próximo (opcional)

  **Must NOT do**:
  - Não implementar playlist avançada
  - Não editar imagem no lightbox

  **Parallelization**: Can Run In Parallel: YES | Wave 3 (with 11, 12, 13, 15, 16) | Blocked By: Task 8

  **Acceptance Criteria**:
  - [ ] Clicar em imagem abre lightbox
  - [ ] Setas do teclado navegam entre imagens
  - [ ] Escape fecha lightbox
  - [ ] Áudio toca com controles play/pause
  - [ ] Volume e seek funcionam

  **QA Scenarios**:
  ```
  Scenario: Lightbox navigation
    Tool: Playwright
    Steps:
      1. Click primeira imagem na galeria
    Expected Result: Lightbox abre com imagem ampliada
      2. Press ArrowRight
    Expected Result: Próxima imagem
      3. Press Escape
    Expected Result: Lightbox fecha
    Evidence: .omo/evidence/task-14-lightbox.gif

  Scenario: Audio player reproduce
    Tool: Playwright
    Steps:
      1. Click arquivo de áudio na galeria
    Expected Result: AudioPlayer abre com play/pause
      2. Click Play
    Expected Result: Áudio toca, botão muda para Pause
    Evidence: .omo/evidence/task-14-audio-player.png
  ```

  **Commit**: YES
  - Message: `feat(gallery): add lightbox and audio player`
  - Files: `src/app/features/gallery/**`, `src/app/shared/components/audio-player*`

---

- [x] 15. **Map — Layer Config Panel (Grid, Fog of War, Markers)**

  **What to do**:
  - Criar `map-config-panel.component.ts`:
    - Painel lateral flutuante (offcanvas/mat-drawer)
    - Toggle de camadas (checkbox):
      - **Grade Hexagonal**: renderizar hex grid sobre o mapa
      - **Grade Quadrada**: renderizar square grid
      - **Fog of War**: overlay escuro com áreas descobertas
      - **Marcadores do Mestre**: pins visíveis apenas para mestre (role flag)
    - Slider de opacidade para fog of war
    - Botão "Resetar Fog of War"
  - Implementar layers no OpenLayers:
    - HexGridLayer: vector layer com hexágonos
    - SquareGridLayer: vector layer com quadrados
    - FogLayer: tile layer escuro com canvas clipping
    - MarkerLayer: vector layer com pins
  - Integrar com MapService

  **Must NOT do**:
  - Não implementar fog of war dinâmico por personagem
  - Grid não precisa ser perfeitamente alinhado ao tile (MVP)
  - Não implementar Three.js aqui (Task 16)

  **Parallelization**: Can Run In Parallel: YES | Wave 3 (with 11, 12, 13, 14, 16) | Blocked By: Task 9

  **Acceptance Criteria**:
  - [ ] Painel lateral abre/fecha com toggle
  - [ ] Toggle grid hex mostra grade no mapa
  - [ ] Toggle fog of war escurece áreas não descobertas
  - [ ] Marcadores aparecem no mapa
  - [ ] Slider de opacidade funciona

  **QA Scenarios**:
  ```
  Scenario: Toggle hex grid
    Tool: Playwright
    Steps:
      1. Open config panel
      2. Toggle "Grade Hexagonal" ON
    Expected Result: Grid hexagonal visível sobre mapa
    Evidence: .omo/evidence/task-15-hex-grid.png

  Scenario: Fog of war toggle
    Tool: Playwright
    Steps:
      1. Toggle "Névoa da Guerra" ON
    Expected Result: Mapa escurecido, apenas áreas descobertas visíveis
    Evidence: .omo/evidence/task-15-fog-of-war.png
  ```

  **Commit**: YES (group with 16)
  - Message: `feat(map): add layer config panel (grid, fog of war, markers)`
  - Files: `src/app/features/map/**`

---

- [x] 16. **Map — 2.5D Three.js Toggle**

  **What to do**:
  - Criar `map-three.service.ts` (wrapping Three.js):
    - Inicializar cena Three.js com renderer WebGL
    - Receber snapshot do estado atual do OpenLayers (tiles, markers, grid)
    - Projetar o mapa 2D em um plano 3D com rotação/inclinação
    - Controles de órbita (rotate, zoom, pan)
  - Criar botão flutuante "Alternar 2D/3D" no canto do mapa:
    - Tooltip: "Alternar visualização 2D/3D"
    - Ícone toggle
  - **IMPORTANTE**: Three.js importado dinamicamente como OpenLayers
  - Transição suave entre 2D e 2.5D
  - Sincronizar view: zoom/center do 2D refletido no 3D

  **Must NOT do**:
  - Não implementar terreno 3D real (height map)
  - Não implementar tokens 3D
  - Three.js nunca no bundle inicial

  **Parallelization**: Can Run In Parallel: YES | Wave 3 (with 11, 12, 13, 14, 15) | Blocks: Task 18 | Blocked By: Task 9

  **Acceptance Criteria**:
  - [ ] Botão "Alternar 2D/3D" visível no mapa
  - [ ] Clicar alterna para visualização 2.5D isométrica
  - [ ] Clicar novamente volta para 2D
  - [ ] Three.js não aparece no bundle inicial
  - [ ] Grid e markers visíveis no modo 3D

  **QA Scenarios**:
  ```
  Scenario: Toggle 2.5D
    Tool: Playwright
    Steps:
      1. Navigate to /mapa
      2. Click "Alternar 2D/3D"
    Expected Result: Mapa muda para projeção isométrica 3D (rotacionável)
    Evidence: .omo/evidence/task-16-3d-toggle.png

  Scenario: Dynamic import Three.js
    Tool: Bash
    Steps:
      1. Run `npx ng build --configuration production --stats-json`
      2. Grep stats.json por "three"
    Expected Result: "three" NÃO aparece no chunk inicial
    Evidence: .omo/evidence/task-16-dynamic-import.log
  ```

  **Commit**: YES (group with 15)
  - Message: `feat(map): add 2.5D Three.js toggle with dynamic import`
  - Files: `src/app/features/map/**`

---

- [x] 17. **Characters — Image Upload/Crop + Master Notes + Quotes**

  **What to do**:
  - Criar/Atualizar cabeçalho do personagem (RF-02):
    - Upload de imagem com `image-crop.component.ts`
    - Preview com ✂️ botão de crop
    - Ao salvar: crop via canvas, salvar como data URL ou Object URL
  - Criar aba **Notas do Mestre** (RF-05):
    - Campo Quill (reutilizar Quill config)
    - Visível apenas se `isMasterRole$` = true no estado global
    - Se não é mestre: mostrar "Apenas o Mestre pode ver esta aba"
  - Criar aba **Falas** (RF-06):
    - Lista dinâmica (FormArray):
      - Cada fala: texto + contexto (quando disse) + botão Copiar
    - Botão "Adicionar Fala", "Remover"
    - Botão "Copiar" usa `navigator.clipboard.writeText()`

  **Must NOT do**:
  - Crop é client-side (canvas), não server-side
  - Role "Mestre" é flag booleana, não autenticação real

  **Parallelization**: Can Run In Parallel: YES | Wave 4 (with 18, 19, 20, 21) | Blocked By: Tasks 11, 12

  **Acceptance Criteria**:
  - [ ] Upload de imagem com preview funciona
  - [ ] Crop tool abre e permite recortar
  - [ ] Aba Notas visível apenas se role = mestre
  - [ ] Adicionar fala com texto + contexto
  - [ ] Copiar fala para clipboard funciona

  **QA Scenarios**:
  ```
  Scenario: Upload e crop de avatar
    Tool: Playwright
    Steps:
      1. Upload "aragorn.jpg"
      2. Click "✂️ Recortar"
      3. Ajustar área de crop
      4. Salvar
    Expected Result: Avatar atualizado com imagem cortada
    Evidence: .omo/evidence/task-17-avatar-crop.png

  Scenario: Notas do Mestre visível
    Tool: Playwright
    Steps:
      1. Set isMasterRole = true no store
      2. Navigate to personagem
      3. Click aba "Notas do Mestre"
    Expected Result: Editor Quill visível
    Evidence: .omo/evidence/task-17-master-notes.png

  Scenario: Copiar fala
    Tool: Playwright
    Steps:
      1. Add fala "Um anel para a todos governar"
      2. Click "Copiar"
    Expected Result: Clipboard contém texto da fala
    Evidence: .omo/evidence/task-17-copy-quote.png
  ```

  **Commit**: YES (group with 11, 12)
  - Message: `feat(characters): add image upload/crop, master notes, and quotes`
  - Files: `src/app/features/characters/**`

---

- [x] 18. **Map — Submap Pins**

  **What to do**:
  - Adicionar funcionalidade de **Pins de Submapa** (RF-15):
    - Botão "Adicionar Pin" no toolbar do mapa
    - Ao clicar no mapa: abre dialog para configurar pin:
      - Nome do submapa
      - Link para mapa filho (selecionar de lista de mapas)
      - Ícone/aparência do pin
    - Pins renderizados como markers especiais no OpenLayers
    - Ao clicar no pin: navega para `/mapa/:submapId`
    - Breadcrumbs: Mapa Principal > Mapa da Taverna
    - Botão "Voltar ao mapa pai"

  **Must NOT do**:
  - Não implementar mini-mapas embutidos (links para outros mapas)

  **Parallelization**: Can Run In Parallel: YES | Wave 4 (with 17, 19, 20, 21) | Blocked By: Tasks 15, 16

  **Acceptance Criteria**:
  - [ ] Botão "Adicionar Pin" no toolbar
  - [ ] Clicar no mapa abre config de pin
  - [ ] Pin aparece no mapa
  - [ ] Clicar no pin navega para submapa
  - [ ] Breadcrumbs mostram hierarquia de mapa

  **QA Scenarios**:
  ```
  Scenario: Criar pin de submapa
    Tool: Playwright
    Steps:
      1. Click "Adicionar Pin"
      2. Click no mapa
      3. Type "Taverna do Estalão"
      4. Select mapa "Taverna"
      5. Salvar
    Expected Result: Pin "Taverna do Estalão" aparece no mapa
    Evidence: .omo/evidence/task-18-submap-pin.png

  Scenario: Navegar para submapa
    Tool: Playwright
    Steps:
      1. Click no pin "Taverna do Estalão"
    Expected Result: Navega para /mapa/taverna-id
    Evidence: .omo/evidence/task-18-submap-nav.png
  ```

  **Commit**: YES (group with 15, 16)
  - Message: `feat(map): add submap pins and navigation`
  - Files: `src/app/features/map/**`

---

- [x] 19. **Rules — PDF Reader with Bookmarks and Pagination**

  **What to do**:
  - Atualizar `rules-reader.component.ts` com funcionalidades completas (RF-17, RF-18):
    - **Bookmarks/Índice Lateral**:
      - Sidebar colapsável à esquerda do leitor
      - Extrair outlines do PDF via `pdfjs-dist/getDocument`
      - Renderizar árvore de bookmarks com `<mat-tree>` ou lista aninhada
      - Ao clicar: navega para página correspondente
    - **Paginação**:
      - Input de página: `<mat-form-field>` type="number"
      - Botões: Primeira, Anterior, Próxima, Última
      - Atalhos de teclado: PageUp/PageDown, setas
      - Indicador "Página X de Y"
    - **Zoom**: slider ou botões de zoom in/out
    - Carregar PDF do IndexedDbFileRepository

  **Must NOT do**:
  - Não implementar busca de texto no PDF
  - Não implementar anotações/marcações

  **Parallelization**: Can Run In Parallel: YES | Wave 4 (with 17, 18, 20, 21) | Blocked By: Task 10

  **Acceptance Criteria**:
  - [ ] Sidebar de bookmarks aparece se PDF tem outlines
  - [ ] Clicar em bookmark navega para página correta
  - [ ] Input de página aceita número e navega
  - [ ] PageUp/PageDown mudam página
  - [ ] Zoom in/out funciona

  **QA Scenarios**:
  ```
  Scenario: Bookmarks sidebar
    Tool: Playwright
    Steps:
      1. Open PDF com bookmarks
    Expected Result: Sidebar esquerda mostra índice com tópicos
      2. Click "Capítulo 1"
    Expected Result: PDF navega para página do capítulo
    Evidence: .omo/evidence/task-19-bookmarks.png

  Scenario: Paginação por teclado
    Tool: Playwright
    Steps:
      1. Focus no leitor de PDF
      2. Press PageDown
    Expected Result: Página incrementada, indicador mostra "Página 2 de 10"
    Evidence: .omo/evidence/task-19-pagination.png
  ```

  **Commit**: YES (group with 10)
  - Message: `feat(rules): add PDF reader with bookmarks, pagination, and zoom`
  - Files: `src/app/features/rules/**`

---

- [x] 20. **Session — Split-Screen Layout + Toolbar**

  **What to do**:
  - Criar `src/app/features/session/` como módulo lazy
  - Criar `session-cockpit.component.ts` (RF-19):
    - Layout split-screen com `<mat-sidenav-container>` horizontal:
      - **Painel Esquerdo (entidade em foco)**: Exibe entidade selecionada (personagem, mapa, nota, etc.)
      - **Painel Direito (referência rápida)**: Lista pesquisável de referências
    - Separador arrastável entre painéis (CDK drag-drop ou `<mat-divider>` vertical)
  - Criar `session-toolbar.component.ts`:
    - Toolbar fixa no topo com atalhos configuráveis:
      - 🎲 "Rolar Dados" (abre dialog de simulação de dados)
      - 🗺️ "Abrir Mapa Principal"
      - 🎵 "Tocar Música de Batalha"
      - ⏯️ "Play/Pause Música"
    - Botões configuráveis via estado (store)
  - Criar `dice-roller-dialog.component.ts`:
    - Input: "1d20", "3d6", "2d8+4"
    - Botão "Rolar"
    - Resultado animado
    - Histórico de rolagens

  **Must NOT do**:
  - Não implementar voz/video
  - Não implementar chat em tempo real
  - Atalhos fixos no MVP (não configuráveis pelo usuário)

  **Parallelization**: Can Run In Parallel: YES | Wave 4 (with 17, 18, 19, 21) | Blocked By: Tasks 3, 4

  **Acceptance Criteria**:
  - [ ] `/sessao` carrega lazy
  - [ ] Split-screen com dois painéis visíveis
  - [ ] Separador arrastável entre painéis
  - [ ] Toolbar com botões de atalho
  - [ ] Dice roller abre dialog e rola dados
  - [ ] Histórico de rolagens

  **QA Scenarios**:
  ```
  Scenario: Split-screen layout
    Tool: Playwright
    Steps:
      1. Navigate to /sessao
    Expected Result: Dois painéis lado a lado com separador
    Evidence: .omo/evidence/task-20-split-screen.png

  Scenario: Dice roller
    Tool: Playwright
    Steps:
      1. Click "Rolar Dados" na toolbar
      2. Type "1d20" no input
      3. Click "Rolar"
    Expected Result: Resultado entre 1-20, adicionado ao histórico
    Evidence: .omo/evidence/task-20-dice-roll.png
  ```

  **Commit**: YES
  - Message: `feat(session): add cockpit with split-screen, toolbar, and dice roller`
  - Files: `src/app/features/session/**`

---

- [x] 21. **Session — Quick Search Dialog**

  **What to do**:
  - Criar `session-quick-search.component.ts` (RF-20):
    - Campo de busca em destaque na toolbar da sessão
    - Ao focar ou Ctrl+F: abre modal flutuante `<mat-dialog>`
    - Busca simultânea em:
      - Nomes de personagens (CharacterService)
      - Nomes de regras/livros (RulesService)
      - Falas de personagens (CharacterService)
    - Resultados agrupados por categoria (Personagens, Regras, Falas)
    - Ao clicar no resultado:
      - Abre modal sobreposto com detalhes (sem sair da sessão)
      - MatDialog sobreposto ao cockpit
    - Fechar com Escape ou click outside
    - Atalho de teclado: Ctrl+F dentro da sessão

  **Must NOT do**:
  - Não buscar conteúdo de rich text (só nomes + tags + falas)
  - Modal não substitui a tela atual (fica sobreposto)

  **Parallelization**: Can Run In Parallel: YES | Wave 4 (with 17, 18, 19, 20) | Blocked By: Tasks 3, 4

  **Acceptance Criteria**:
  - [ ] Ctrl+F abre busca rápida
  - [ ] Busca retorna personagens, regras e falas
  - [ ] Resultados agrupados por categoria
  - [ ] Clicar em resultado abre modal de detalhe
  - [ ] Modal sobreposto não fecha sessão

  **QA Scenarios**:
  ```
  Scenario: Quick search results
    Tool: Playwright
    Steps:
      1. Create personagem "Aragorn" com fala "Um anel"
      2. Navigate to /sessao
      3. Press Ctrl+F
      4. Type "anel"
    Expected Result: Resultados mostram "Aragorn" e fala "Um anel para a todos governar"
    Evidence: .omo/evidence/task-21-quick-search.png

  Scenario: Result detail modal
    Tool: Playwright
    Steps:
      1. Search "Aragorn"
      2. Click resultado
    Expected Result: Modal com detalhes do personagem abre sobre a sessão
    Evidence: .omo/evidence/task-21-result-modal.png
  ```

  **Commit**: YES (group with 20)
  - Message: `feat(session): add quick search dialog with cross-module results`
  - Files: `src/app/features/session/**`

---

- [x] 22. **Global Search (Ctrl+K) Integration**

  **What to do**:
  - Completar `search-modal.component.ts` com funcionalidade cross-module:
    - Registrar GlobalSearchService no core (já esboçado na Task 3)
    - Indexar todas as entidades ao criar/atualizar:
      - Personagens (nome, descrição, falas)
      - Pastas de campanha (nome)
      - Itens da galeria (nome)
      - Livros de regras (nome, descrição)
      - Mapas (nome)
    - Modal com debounce search (300ms)
    - Resultados agrupados por tipo de entidade
    - Highlight do termo buscado nos resultados
    - Atalho global Ctrl+K (registrado no ShellComponent)
    - Ao selecionar: navega para entidade (fecha modal, vai pra rota)
    - Fechar com Escape

  **Must NOT do**:
  - Não implementar fuzzy search (exact match + case insensitive no MVP)
  - Não indexar conteúdo de rich text ou PDF

  **Parallelization**: Can Run In Parallel: NO | Blocks: Task 23 | Blocked By: Tasks 17, 18, 20, 21

  **Acceptance Criteria**:
  - [ ] Ctrl+K de qualquer tela abre modal
  - [ ] Digitar "Aragorn" retorna personagem correspondente
  - [ ] Resultados agrupados por tipo
  - [ ] Selecionar resultado navega para rota correta
  - [ ] Modal fecha com Escape

  **QA Scenarios**:
  ```
  Scenario: Global search from any module
    Tool: Playwright
    Steps:
      1. Create personagem "Gandalf" com fala "You shall not pass"
      2. Navigate to /campanha
      3. Press Ctrl+K
      4. Type "Gandalf"
    Expected Result: Gandalf aparece nos resultados
      5. Click resultado
    Expected Result: Navega para /personagens/gandalf
    Evidence: .omo/evidence/task-22-global-search.png
  ```

  **Commit**: YES
  - Message: `feat(search): integrate global search (Ctrl+K) across all modules`
  - Files: `src/app/core/services/search*`, `src/app/shared/search-modal/**`, `src/app/layout/**`

---

- [x] 23. **Polish — Error States, Empty States, Loading States**

  **What to do**:
  - Revisar TODOS os componentes para incluir estados:
    - **Loading**: Spinner enquanto dados carregam (usar LoadingSpinnerComponent)
    - **Empty**: Mensagem amigável quando não há dados (usar EmptyStateComponent)
    - **Error**: Mensagem de erro + botão "Tentar Novamente"
    - **404**: Rota de "personagem não encontrado", "mapa não encontrado"
  - Adicionar `@angular/animations` para transições suaves
  - Registrar `provideAnimations()` em app.config.ts
  - Verificar acessibilidade:
    - `aria-label` em botões de ícone
    - `role` em elementos interativos
    - Focus management em modais
    - Tab order lógico

  **Must NOT do**:
  - Não mudar comportamento dos componentes (só adicionar estados)
  - Não fazer testes de acessibilidade automatizados (fora do escopo)

  **Parallelization**: Can Run In Parallel: NO | Blocked By: Task 22

  **Acceptance Criteria**:
  - [ ] Empty state aparece quando lista está vazia
  - [ ] Loading spinner aparece durante carregamento
  - [ ] 404 tratado para entidades inexistentes
  - [ ] Animações presentes em transições

  **QA Scenarios**:
  ```
  Scenario: Empty state
    Tool: Playwright
    Steps:
      1. Ensure empty database
      2. Navigate to /personagens
    Expected Result: EmptyStateComponent com "Nenhum personagem encontrado" + botão "Criar Personagem"
    Evidence: .omo/evidence/task-23-empty-state.png

  Scenario: 404 personagem
    Tool: Playwright
    Steps:
      1. Navigate to /personagens/id-inexistente
    Expected Result: "Personagem não encontrado" + botão Voltar
    Evidence: .omo/evidence/task-23-404.png
  ```

  **Commit**: YES (group with 22)
  - Message: `feat(ux): add error states, empty states, and loading indicators across all modules`
  - Files: `src/app/features/**/*.ts`, `src/app/shared/**/*.ts`

---

- [x] 24. **npm Scripts + Build Optimization**

  **What to do**:
  - Adicionar scripts úteis em `package.json`:
    ```json
    "test": "vitest",
    "test:watch": "vitest --watch",
    "test:coverage": "vitest --coverage",
    "lint": "ng lint",
    "build:prod": "ng build --configuration production",
    "analyze": "ng build --configuration production --stats-json"
    ```
  - Verificar bundle budgets com `ng build --configuration production`
  - Verificar lazy loading: nenhum chunk de módulo no initial bundle
  - Verificar dynamic imports: Three.js e OpenLayers em chunks separados
  - Adicionar `sourceMap: false` em production (já vem por default do Angular)
  - Verificar se há polyfills desnecessários

  **Must NOT do**:
  - Não adicionar ferramenta de análise de bundle (source-map-explorer fica para depois)
  - Não modificar `tsconfig.json` paths

  **Parallelization**: Can Run In Parallel: NO | Blocks: F1-F4 | Blocked By: Tasks 22, 23

  **Acceptance Criteria**:
  - [ ] `npm test` roda vitest
  - [ ] `npm run build:prod` compila sem erros
  - [ ] Budget initial ≤ 3MB
  - [ ] Nenhum chunk de módulo lazy no initial bundle

  **QA Scenarios**:
  ```
  Scenario: Build production
    Tool: Bash
    Steps:
      1. Run `npx ng build --configuration production`
    Expected Result: Exit code 0, budgets within limits
    Evidence: .omo/evidence/task-24-build-prod.log

  Scenario: Bundle analysis
    Tool: Bash
    Steps:
      1. Run `npx ng build --configuration production --stats-json`
      2. Check initial chunk size
    Expected Result: initial ≤ 3MB, lazy chunks separated
    Evidence: .omo/evidence/task-24-bundle-analysis.log
  ```

  **Commit**: YES
  - Message: `chore(scripts): add npm scripts and verify build optimization`
  - Files: `package.json`

---

## Final Verification Wave (MANDATORY)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .omo/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `ng build --configuration production` + `npx vitest run`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill)
  Start from clean state. Execute EVERY QA scenario from EVERY task. Test cross-task integration. Test edge cases: empty state, invalid input, rapid actions. Save to `.omo/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec. Check "Must NOT do" compliance. Detect cross-task contamination.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

- **1**: `chore(deps): install Angular Material, Quill, OpenLayers, Three.js, ng2-pdf-viewer`
- **2**: `chore(test): configure Vitest + Angular Testing Library` (group with 1)
- **3**: `feat(core): add models, store, repositories, and router with lazy loading` (group with 1,2)
- **4**: `feat(shell): add app layout with sidebar and global search modal`
- **5**: `feat(shared): add shared UI components (dialog, empty-state, upload, audio, crop)` (group with 4)
- **6**: `feat(characters): add list view with type filters (TDD)`
- **7**: `feat(campaign): add tree view with CDK drag-drop (TDD)`
- **8**: `feat(gallery): add dropzone upload and thumbnail grid`
- **9**: `feat(map): add OpenLayers map with dynamic import`
- **10**: `feat(rules): add PDF upload and ng2-pdf-viewer setup`
- **11**: `feat(characters): add detail view with tabs and Quill rich text`
- **12**: `feat(characters): add character sheet with FormArray (attributes, skills, inventory)` (group with 11)
- **13**: `feat(campaign): add entity association dialog and folder content`
- **14**: `feat(gallery): add lightbox and audio player`
- **15**: `feat(map): add layer config panel (grid, fog of war, markers)`
- **16**: `feat(map): add 2.5D Three.js toggle with dynamic import` (group with 15)
- **17**: `feat(characters): add image upload/crop, master notes, and quotes` (group with 11,12)
- **18**: `feat(map): add submap pins and navigation` (group with 15,16)
- **19**: `feat(rules): add PDF reader with bookmarks, pagination, and zoom` (group with 10)
- **20**: `feat(session): add cockpit with split-screen, toolbar, and dice roller`
- **21**: `feat(session): add quick search dialog with cross-module results` (group with 20)
- **22**: `feat(search): integrate global search (Ctrl+K) across all modules`
- **23**: `feat(ux): add error states, empty states, and loading indicators across all modules` (group with 22)
- **24**: `chore(scripts): add npm scripts and verify build optimization`

---

## Success Criteria

### Verification Commands
```bash
ng serve              # App loads with sidebar + home
ng build --configuration production  # Build sem erros, budgets ok
npx vitest run        # Todos os testes passam
```

### Final Checklist
- [ ] 6 módulos lazy-loaded funcionando
- [ ] CRUD Personagens completo (todas as abas)
- [ ] Árvore Campanha com drag-drop e associação
- [ ] Galeria com upload, thumbnails, lightbox, áudio
- [ ] Regras com upload PDF e leitor completo
- [ ] Mapa OpenLayers + toggle 2.5D + camadas + submapas
- [ ] Sessão com split-screen + toolbar + dice roller + busca rápida
- [ ] Global Search (Ctrl+K) funcional
- [ ] Testes passando (Vitest)
- [ ] Build production sem erros
- [ ] Budget initial ≤ 3MB
- [ ] Lazy loading confirmado
- [ ] Dynamic imports para Three.js e OpenLayers
- [ ] Persistência localStorage + IndexedDB





