# Plano de Correção — Responsividade, Botões Quebrados & Warnings (MVP Angular 21)

## TL;DR

> **Quick Summary**: Corrigir todos os problemas críticos de responsividade, botões sem ação, warnings TypeScript e falta de persistência de dados no MVP Angular 21 do meuRPG.
>
> **Deliverables**:
> - Dados persistidos via localStorage (PersistenceService.init() no bootstrap)
> - Botões da Home, CharacterList empty state e CharacterDetail com navegação funcional
> - Diálogos com gerenciamento correto de afterClosed e ImageCrop com emissão única
> - Layout responsivo em mobile para QuickSearch, Cockpit split, CharacterSheet e RulesReader
> - 7 warnings NG8107 resolvidos, 18 tipos `any` tipados (map-three + rules-reader)
> - Código morto removido (SidebarService, map.component.ts)
>
> **Estimated Effort**: Medium (~25 tarefas)
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Task 1 (PersistenceService.init) → Task 5 → Task 7 → Task 15 → Tasks F1-F4

---

## Context

### Original Request
Avaliar TODOS os erros de responsividade, botões quebrados e warnings do MVP frontend Angular 21 e gerar um plano de correção completo.

### Interview Summary
**4 agentes background analisaram em paralelo**:
1. **Responsividade** — 33 arquivos de estilo, apenas 1 com media queries (3%), 74 larguras fixas em px
2. **Botões Quebrados** — PersistenceService.init() nunca chamado (perda total de dados), 2 botões Home sem handler, empty state quebrado, diálogos sem afterClosed
3. **TypeScript Warnings** — 0 erros de compilação, 7 NG8107 warnings, 19 tipos `any`, 4 StoreService sem genérico
4. **Estado e Persistência** — StoreService completo, Vitest configurado (8 spec files), PersistenceService.init() nunca invocado

### Metis Review
**Identified Gaps** (addressed):
- Test infrastructure EXISTS (Vitest) — draft corrigido para tests-after
- Guardrails definidos: NÃO refatorar estilos inline para SCSS externo, NÃO modificar SessionComponent "em construção", NÃO adicionar features novas
- Prioridade: Persistência > Botões > Responsividade > TS Warnings > Código morto

---

## Work Objectives

### Core Objective
Corrigir todos os bugs de runtime, responsividade e warnings identificados no MVP Angular 21, priorizando perda de dados e funcionalidades quebradas.

### Concrete Deliverables
- `app.config.ts` — APP_INITIALIZER com PersistenceService.init()
- 4 componentes com botões/handlers corrigidos (Home, CharacterList, CharacterDetail, GalleryLightbox)
- 4 componentes com responsividade crítica corrigida (QuickSearch, Cockpit, CharacterSheet, RulesReader)
- 3 arquivos com NG8107 resolvidos (file-upload, page-header, gallery-lightbox)
- 2 arquivos com `any` tipados (map-three.service, rules-reader)
- Código morto removido (SidebarService, map.component.ts)

### Definition of Done
- [ ] `ng build` passa sem erros (apenas 0 NG8107 warnings desejável)
- [ ] `ng test` passa (8+ spec files)
- [ ] Dados persistem após reload completo
- [ ] Todos os botões listados navegam para rota correta
- [ ] 4 problemas críticos de responsividade corrigidos em viewport 375px

### Must Have
- PersistenceService.init() integrado no bootstrap (APP_INITIALIZER)
- HomeComponent botões com navegação funcional
- CharacterList empty state action funcional
- SessionQuickSearch com width responsivo em mobile
- SessionCockpit split com fallback vertical em mobile
- CharacterSheet flex-wrap: nowrap → wrap
- RulesReader sidebar com fallback mobile
- NG8107 file-upload e page-header corrigidos
- ImageCrop com emissão única

### Must NOT Have (Guardrails)
- NÃO refatorar estilos inline para arquivos SCSS externos — media queries permanecem inline
- NÃO modificar SessionComponent além da correção do diálogo
- NÃO adicionar novas features ou completar partes "em construção"
- NÃO mudar a lógica de carregamento dinâmico do Three.js
- NÃO adicionar cobertura completa de testes para código existente não modificado
- NÃO criar módulos service (CharacterService, etc.) — fora de escopo

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed.

### Test Decision
- **Infrastructure exists**: YES (vitest)
- **Automated tests**: Tests-after (testes unitários para correções principais)
- **Framework**: vitest ^4.1.10 (jsdom)

### QA Policy
Every task MUST include agent-executed QA scenarios.
- **Frontend/UI**: Use Playwright — Navigate, interact, assert DOM, screenshot (.omo/evidence/task-{N}-{scenario}.png)
- **API/Backend**: Use Bash (curl) — Send requests, assert status + response fields
- **CLI**: Use Bash — Run commands, check exit codes and output

---

## Execution Strategy

```
Wave 1 (Foundation — quick, independent fixes):
├── Task 1: PersistenceService.init() via APP_INITIALIZER [quick]
├── Task 2: HomeComponent — adicionar (click) handlers [quick]
├── Task 13: NG8107 — file-upload + page-header [quick]
└── Task 18: Padronizar *ngIf → @if no dice-roller-dialog [quick]

Wave 2 (Core fixes — moderate, majority parallel):
├── Task 3: CharacterList onNewCharacter() — navegação funcional [quick]
├── Task 4: Diálogos com afterClosed() [quick]
├── Task 5: ImageCrop — emissão única [quick]
├── Task 10: Diálogos min-width fixo → responsivo [unspecified-high]
├── Task 17: character-detail error()! → error() ?? [quick]
├── Task 11: FileUpload preview max-width: 100% [quick]
└── Task 14: NG8107 gallery-lightbox (falso positivo) [quick]

Wave 3 (Complex fixes — CSS + tipagem):
├── Task 6: SessionQuickSearch width 560px → responsivo [visual-engineering]
├── Task 7: SessionCockpit split → fallback mobile stack [visual-engineering]
├── Task 8: CharacterSheet nowrap → wrap [visual-engineering]
├── Task 9: RulesReader sidebar 240px → responsivo [visual-engineering]
├── Task 12: Fontes px → rem (map-config-panel, map-view, submap-pin) [quick]
├── Task 15: Tipar map-three.service.ts (14 any) [unspecified-high]
└── Task 16: Tipar rules-reader.component.ts (4 any) [unspecified-high]

Wave 4 (Cleanup — dead code removal):
├── Task 19: Remover SidebarService não utilizado [unspecified-high]
└── Task 20: Remover map.component.ts (arquivo morto) [quick]

Wave FINAL (4 parallel reviews):
├── F1: Plan compliance audit [oracle]
├── F2: Code quality review [unspecified-high]
├── F3: Real manual QA [unspecified-high]
└── F4: Scope fidelity check [deep]
-> Present results -> Get explicit user okay
```

### Dependency Matrix
- **1**: - → 3
- **2, 13, 18**: - → 6-9, 12, 15, 16
- **3-5, 10, 11, 14, 17**: - → 6-9, 12, 15, 16
- **6-9, 12, 15, 16**: - → 19, 20
- **19, 20**: - → F1-F4
- **F1-F4**: - → user okay

---

## TODOs

- [x] 1. **Integrar PersistenceService.init() via APP_INITIALIZER**

  **What to do**:
  - Criar um provider `APP_INITIALIZER` em `src/app/app.config.ts` que injete `PersistenceService` e chame `init()`
  - Garantir que `init()` seja chamado ANTES da renderização do primeiro componente
  - Verificar que `store.events$` subscription (dentro de `init()`) passa a persistir mudanças automaticamente
  - Verificar que `init()` carrega dados do localStorage para o store na inicialização
  - Tratar erro de JSON corrompido no localStorage silenciosamente (já implementado no PersistenceService)

  **Must NOT do**:
  - NÃO chamar `init()` no constructor do AppComponent ou ShellComponent
  - NÃO modificar a assinatura do método `init()`
  - NÃO adicionar dependências circulares

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Config change, single file, well-understood Angular pattern
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 13, 18)
  - **Blocks**: Tasks 3-20 (all downstream fixes need persistence working)
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/app.config.ts` — File to modify, currently has `provideRouter` and `provideAnimations`
  - `src/app/core/services/persistence.service.ts` — Service whose `init()` method needs to be called

  **Acceptance Criteria**:
  - [ ] `app.config.ts` has APP_INITIALIZER calling PersistenceService.init()
  - [ ] `ng build` passes (0 errors)
  - [ ] After calling `store.set('characters', {...})` and reloading, data persists (verified via Playwright)

  **QA Scenarios**:
  ```
  Scenario: Dados persistem após reload
    Tool: Bash + Playwright
    Preconditions: App running on localhost
    Steps:
      1. Navigate to /personagens
      2. Inject character via store: evaluate `window.__store.set('characters', { id: 'test-1', name: 'Test', type: 'player', attributes: {} })`
      3. Assert character appears in list
      4. Reload page (Playwright page.reload)
      5. Assert character still appears in list
    Expected Result: Character persists across page reload
    Evidence: .omo/evidence/task-1-persistence.png

  Scenario: Dados corrompidos no localStorage não quebram o app
    Tool: Bash
    Preconditions: App running, localStorage has invalid JSON for key 'meurpg_characters'
    Steps:
      1. Set corrupt data: `localStorage.setItem('meurpg_characters', '{corrupt')`
      2. Reload page
      3. Assert app renders without errors (no console errors)
    Expected Result: App initializes with empty state, no crash
    Evidence: .omo/evidence/task-1-corrupt-data.txt
  ```

  **Commit**: NO (groups with 2-5, 13, 18)
  - Message: `fix(core): ativa persistência de dados e corrige interações quebradas`

- [x] 2. **HomeComponent — adicionar (click) handlers nos botões CTA**

  **What to do**:
  - No template do `src/app/home.component.ts`, adicionar binding `(click)` nos botões "Começar Jornada" e "Explorar"
  - "Começar Jornada" deve navegar para `/personagens/novo`
  - "Explorar" deve navegar para `/personagens`
  - Injetar `Router` via `inject(Router)` e usar `this.router.navigate([...])`

  **Must NOT do**:
  - NÃO mudar o layout ou estilo da Home
  - NÃO adicionar animações ou transições

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple event handler addition, 2 lines of code
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 13, 18)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/home.component.ts:13-14` — The two `<button>` elements without (click)

  **Acceptance Criteria**:
  - [ ] "Começar Jornada" button has (click) handler
  - [ ] "Explorar" button has (click) handler
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: "Começar Jornada" navigates to /personagens/novo
    Tool: Playwright
    Preconditions: App running, on home page
    Steps:
      1. page.goto('/')
      2. page.click('text=Começar Jornada')
      3. Assert URL contains '/personagens/novo'
    Expected Result: Navigation to new character page
    Evidence: .omo/evidence/task-2-comecar-jornada.png

  Scenario: "Explorar" navigates to /personagens
    Tool: Playwright
    Preconditions: App running, on home page
    Steps:
      1. page.goto('/')
      2. page.click('text=Explorar')
      3. Assert URL contains '/personagens'
    Expected Result: Navigation to character list
    Evidence: .omo/evidence/task-2-explorar.png
  ```

  **Commit**: NO (groups with 1, 3-5, 13, 18)

- [x] 3. **CharacterListComponent — implementar onNewCharacter()**

  **What to do**:
  - Implementar o método `onNewCharacter()` em `src/app/features/characters/character-list.component.ts`
  - Deve navegar para `/personagens/novo` via `this.router.navigate(['/personagens', 'novo'])`
  - O método existe mas está vazio (apenas um comentário)

  **Must NOT do**:
  - NÃO mudar o EmptyStateComponent ou seu template
  - NÃO adicionar lógica extra além da navegação

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Trivial method implementation, 2 lines
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5, 10, 11, 14, 17)
  - **Blocks**: None
  - **Blocked By**: Task 1 (PersistenceService.init)

  **References**:
  - `src/app/features/characters/character-list.component.ts:324-326` — Empty method stub

  **Acceptance Criteria**:
  - [ ] `onNewCharacter()` navigates to `/personagens/novo`
  - [ ] Empty state "Criar Personagem" button triggers navigation
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: Empty state action navigates to new character
    Tool: Playwright
    Preconditions: App running, clean state (no characters)
    Steps:
      1. page.goto('/personagens')
      2. Assert empty state is visible
      3. Click the "Criar Personagem" or action button in empty state
      4. Assert URL contains '/personagens/novo'
    Expected Result: Navigation to new character page
    Evidence: .omo/evidence/task-3-empty-state-action.png

  Scenario: Empty state shows button
    Tool: Playwright
    Preconditions: App running, no characters in store
    Steps:
      1. page.goto('/personagens')
      2. Assert empty state app-empty-state element is visible
      3. Assert action button exists in empty state
    Expected Result: Empty state rendering correctly
    Evidence: .omo/evidence/task-3-empty-state-render.png
  ```

  **Commit**: NO (groups with 1, 2, 4, 5, 13, 18)

- [x] 4. **Diálogos — adicionar afterClosed() e armazenar dialogRef**

  **What to do**:
  - Em `src/app/features/session/session.component.ts:100-106` — `openQuickSearch()`: armazenar `MatDialogRef` e chamar `afterClosed().subscribe()`
  - Em `src/app/features/session/session-cockpit.component.ts` — `openDiceRoller()`: armazenar `MatDialogRef` e chamar `afterClosed().subscribe()`
  - Adicionar `takeUntil(this.destroy$)` no afterClosed para evitar vazamento se o componente for destruído enquanto o diálogo está aberto

  **Must NOT do**:
  - NÃO modificar o conteúdo dos componentes de diálogo (QuickSearch, DiceRoller)
  - NÃO adicionar lógica no afterClosed além da assinatura mínima

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Standard Angular pattern, well-understood
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 5, 10, 11, 14, 17)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately, independent of Task 1)

  **References**:
  - `src/app/features/session/session.component.ts:100-106` — openQuickSearch without afterClosed
  - `src/app/features/session/session-cockpit.component.ts` — openDiceRoller without afterClosed
  - `src/app/layout/shell.component.ts:176-178` — pattern to follow for takeUntil(destroy$)

  **Acceptance Criteria**:
  - [ ] `session.component.ts` — dialogRef stored and afterClosed subscribed
  - [ ] `session-cockpit.component.ts` — dialogRef stored and afterClosed subscribed
  - [ ] `ng build` passes
  - [ ] `ng test` passes

  **QA Scenarios**:
  ```
  Scenario: QuickSearch dialog opens and closes without leaks
    Tool: Playwright
    Preconditions: App running, on session page
    Steps:
      1. page.goto('/sessao')
      2. Open QuickSearch (Ctrl+K or button)
      3. Close dialog (Esc key or click outside)
      4. Open QuickSearch again
      5. Assert only one dialog overlay exists
      6. Assert no console errors
    Expected Result: Dialog opens/closes cleanly, no accumulation
    Evidence: .omo/evidence/task-4-quicksearch-dialog.png

  Scenario: DiceRoller dialog opens and closes without leaks
    Tool: Playwright
    Preconditions: App running, on session page
    Steps:
      1. page.goto('/sessao')
      2. Click dice roller button in toolbar
      3. Close dialog
      4. Open dice roller again
      5. Assert only one dialog overlay exists
    Expected Result: Dialog opens/closes cleanly
    Evidence: .omo/evidence/task-4-diceroller-dialog.png
  ```

  **Commit**: NO (groups with 1-3, 5, 13, 18)

- [x] 5. **ImageCropComponent — emitir cropComplete apenas UMA vez**

  **What to do**:
  - Em `src/app/shared/components/image-crop.component.ts`, modificar `applyCrop()` para emitir `cropComplete` UMA ÚNICA vez
  - Decisão: emitir apenas `dataUrl` (string) para manter compatibilidade com `AvatarCropDialogComponent.onCropComplete()` que só trata `string`
  - Remover o segundo emit (Blob) ou comentar com explicação

  **Must NOT do**:
  - NÃO mudar a assinatura do Output `cropComplete`
  - NÃO modificar a lógica de crop existente

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple removal of redundant emission
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 10, 11, 14, 17)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/shared/components/image-crop.component.ts:224-279` — applyCrop with double emission
  - `src/app/features/characters/avatar-crop-dialog.component.ts:47-51` — Consumer that only handles string

  **Acceptance Criteria**:
  - [ ] `applyCrop()` emits `cropComplete` exactly once (string dataUrl)
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: Image crop emits once
    Tool: Bash (node REPL simulation)
    Preconditions: ImageCrop component loaded
    Steps:
      1. Inspect applyCrop() method
      2. Assert cropComplete.emit() called only once
      3. Assert first emission is dataUrl string
      4. Assert no second emission
    Expected Result: Single emission
    Evidence: .omo/evidence/task-5-single-emission.txt
  ```

  **Commit**: NO (groups with 1-4, 13, 18)

- [x] 6. **SessionQuickSearch — width 560px fixo → responsivo**

  **What to do**:
  - Em `src/app/features/session/session-quick-search.component.ts`, alterar `.qs-container { width: 560px }` para:
    ```css
    width: min(560px, 95vw);
    max-height: min(620px, 90vh);
    ```
    Isso faz o container ocupar 95% da viewport em telas <589px
  - Verificar que todos os elementos internos com overflow funcionam com o novo width

  **Must NOT do**:
  - NÃO mudar o layout interno (linhas, avatar, detalhes)
  - NÃO adicionar scroll horizontal ao container

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: CSS/layout change that affects visual rendering
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8, 9, 12, 15, 16)
  - **Blocks**: None
  - **Blocked By**: Wave 2 tasks

  **References**:
  - `src/app/features/session/session-quick-search.component.ts:320` — `.qs-container { width: 560px }`
  - `src/app/features/session/session-quick-search.component.ts:321` — `max-height: 620px`

  **Acceptance Criteria**:
  - [ ] `.qs-container` width changed to `min(560px, 95vw)`
  - [ ] `ng build` passes
  - [ ] In viewport 375px, container width ≤ 356px (95vw)

  **QA Scenarios**:
  ```
  Scenario: QuickSearch fits mobile viewport
    Tool: Playwright
    Preconditions: App running, on session page
    Steps:
      1. Set viewport to 375x812
      2. Open QuickSearch (Ctrl+K)
      3. Assert container width ≤ viewport width
      4. Assert no horizontal scrollbar
      5. Screenshot
    Expected Result: Dialog fits within viewport
    Evidence: .omo/evidence/task-6-quicksearch-mobile.png

  Scenario: QuickSearch still renders correctly on desktop
    Tool: Playwright
    Preconditions: App running, on session page
    Steps:
      1. Set viewport to 1440x900
      2. Open QuickSearch
      3. Assert container width ≈ 560px
      4. Assert layout matches screenshot
    Expected Result: Desktop layout unchanged
    Evidence: .omo/evidence/task-6-quicksearch-desktop.png
  ```

  **Commit**: NO (groups with 7-12)

- [x] 7. **SessionCockpit — split layout com fallback mobile (stack vertical)**

  **What to do**:
  - Em `src/app/features/session/session-cockpit.component.ts`, ADICIONAR media queries nos estilos inline
  - `@media (max-width: 768px)` para:
    - Mudar `.split-container` de `flex-direction: row` para `flex-direction: column`
    - Painéis passam a ocupar 100% da largura
    - O split divider fica oculto (ou vira um separador horizontal)
    - `overflow: hidden` nos painéis vira `overflow: auto` para permitir scroll
  - Garantir que a lógica de resize (drag divider) não quebre — ela pode continuar ativa, mas em mobile os painéis empilham independentemente do ratio

  **Must NOT do**:
  - NÃO remover a lógica de drag divider
  - NÃO mudar o layout desktop existente (>768px)
  - NÃO adicionar dependências CSS externas

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Layout-critical CSS changes affecting split panel behavior
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 8, 9, 12, 15, 16)
  - **Blocks**: None
  - **Blocked By**: Wave 2 tasks

  **References**:
  - `src/app/features/session/session-cockpit.component.ts:161-175` — split-container with overflow hidden
  - `src/app/features/session/session-cockpit.component.ts:364-369` — panel width percentages (60%/40%)
  - `src/app/features/campaign/campaign.component.ts:76` — existing media query pattern (@media max-width: 768px)

  **Acceptance Criteria**:
  - [ ] `@media (max-width: 768px)` added to split container
  - [ ] Panels stack vertically in mobile
  - [ ] No horizontal overflow in mobile
  - [ ] Desktop layout unchanged

  **QA Scenarios**:
  ```
  Scenario: Cockpit stacks vertically on mobile
    Tool: Playwright
    Preconditions: App running, on session page
    Steps:
      1. Set viewport to 375x812
      2. Assert split-container has flex-direction: column (or equivalent stacking)
      3. Assert each panel takes full width
      4. Assert no horizontal scroll
      5. Screenshot
    Expected Result: Panels stack vertically, no overflow
    Evidence: .omo/evidence/task-7-cockpit-mobile.png

  Scenario: Cockpit split layout unchanged on desktop
    Tool: Playwright
    Preconditions: App running, on session page
    Steps:
      1. Set viewport to 1440x900
      2. Assert split-container has flex-direction: row
      3. Assert panels side by side
      4. Screenshot compare with baseline
    Expected Result: Desktop layout identical to before
    Evidence: .omo/evidence/task-7-cockpit-desktop.png
  ```

  **Commit**: NO (groups with 6, 8-12)

- [x] 8. **CharacterSheet — flex-wrap: nowrap → wrap nas linhas dinâmicas**

  **What to do**:
  - Em `src/app/features/characters/character-sheet.component.ts:319`, alterar `.dynamic-row { flex-wrap: nowrap }` para `flex-wrap: wrap`
  - Ajustar `gap` e `min-width` nos itens internos para que quebrem de forma legível
  - `.flex-1 { flex: 1 1 80px; min-width: 70px }` — considerar reduzir `min-width` para `60px` em mobile

  **Must NOT do**:
  - NÃO mudar o layout de outras seções do componente
  - NÃO quebrar o grid de atributos (repeat(3, 1fr))

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: CSS layout change affecting form rendering
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 7, 9, 12, 15, 16)
  - **Blocks**: None
  - **Blocked By**: Wave 2 tasks

  **References**:
  - `src/app/features/characters/character-sheet.component.ts:315-319` — .dynamic-row with nowrap
  - `src/app/features/characters/character-sheet.component.ts:326-328` — .flex-1 with min-width: 70px

  **Acceptance Criteria**:
  - [ ] `.dynamic-row` changed to `flex-wrap: wrap`
  - [ ] Skill rows wrap correctly in mobile
  - [ ] Inventory rows wrap correctly in mobile
  - [ ] Desktop layout unchanged

  **QA Scenarios**:
  ```
  Scenario: Skill rows wrap on mobile
    Tool: Playwright
    Preconditions: App running, on character detail page, sheet tab
    Steps:
      1. Set viewport to 375x812
      2. Add a skill (if not present)
      3. Assert skill row items wrap to next line
      4. Assert no horizontal overflow
      5. Screenshot
    Expected Result: Skill/inventory rows wrap instead of compressing
    Evidence: .omo/evidence/task-8-sheet-mobile.png
  ```

  **Commit**: NO (groups with 6, 7, 9-12)

- [x] 9. **RulesReader — sidebar 240px fixa com fallback mobile**

  **What to do**:
  - Em `src/app/features/rules/rules-reader.component.ts:286-287`, adicionar media query `@media (max-width: 768px)`:
    - Sidebar (`.bookmarks-sidebar`) muda para `width: 100%; min-width: unset` ou fica oculta com toggle
    - Conteúdo do PDF (`.pdf-container`) ocupa 100% da largura
    - Considerar adicionar um botão toggle para mostrar/esconder a sidebar em mobile

  **Must NOT do**:
  - NÃO modificar a lógica de carregamento de PDF (pdf.js)
  - NÃO mudar o layout do rules-list.component.ts

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: CSS layout change with mobile toggle UX consideration
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 7, 8, 12, 15, 16)
  - **Blocks**: None
  - **Blocked By**: Wave 2 tasks

  **References**:
  - `src/app/features/rules/rules-reader.component.ts:286-287` — .bookmarks-sidebar width: 240px
  - `src/app/features/rules/rules-reader.component.ts:346` — .pdf-container

  **Acceptance Criteria**:
  - [ ] Sidebar adapts in viewport <768px (stacks below or hides)
  - [ ] PDF content fills width in mobile
  - [ ] Desktop layout unchanged

  **QA Scenarios**:
  ```
  Scenario: RulesReader mobile layout
    Tool: Playwright
    Preconditions: App running, PDF document open in reader
    Steps:
      1. Set viewport to 375x812
      2. Navigate to /regras and open a PDF
      3. Assert sidebar is either hidden or stacked below
      4. Assert PDF container fills viewport width
      5. Screenshot
    Expected Result: Content readable on mobile
    Evidence: .omo/evidence/task-9-rules-mobile.png

  Scenario: RulesReader desktop unchanged
    Tool: Playwright
    Preconditions: App running, PDF open
    Steps:
      1. Set viewport to 1440x900
      2. Assert sidebar visible at 240px
      3. Assert PDF container beside sidebar
    Expected Result: Desktop layout identical
    Evidence: .omo/evidence/task-9-rules-desktop.png
  ```

  **Commit**: NO (groups with 6-8, 10-12)

- [x] 10. **Diálogos — corrigir min-width fixo para responsivo**

  **What to do**:
  - Corrigir os diálogos que têm `min-width` em px para usarem unidades relativas:
    - `entity-selector-dialog.component.ts:124` — `min-width: 420px` → `min-width: min(420px, 90vw)`
    - `avatar-crop-dialog.component.ts:28-29` — `min-width: 360px; min-height: 200px` → `min-width: min(360px, 90vw); min-height: min(200px, 50vh)`
    - `confirm-dialog.component.ts:39` — `min-width: 320px` → `min-width: min(320px, 90vw)`
    - `dice-roller-dialog.component.ts:158` — `min-width: 320px` → `min-width: min(320px, 90vw)`
  - Usar `min()` do CSS para garantir que o tamanho nunca exceda a viewport

  **Must NOT do**:
  - NÃO mudar o conteúdo ou layout interno dos diálogos
  - NÃO mexer no `max-width` dos diálogos (já configurado pelo Angular Material)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple files, but simple CSS changes
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 5, 11, 14, 17)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/features/campaign/entity-selector-dialog.component.ts:124`
  - `src/app/features/characters/avatar-crop-dialog.component.ts:28-29`
  - `src/app/shared/components/confirm-dialog.component.ts:39`
  - `src/app/features/session/dice-roller-dialog.component.ts:158`

  **Acceptance Criteria**:
  - [ ] All 4 dialogs have `min-width` using `min()` with viewport-aware fallback
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: Dialogs fit mobile viewport
    Tool: Playwright
    Preconditions: App running
    Steps:
      1. Set viewport to 375x812
      2. Open entity-selector dialog
      3. Assert dialog width fits viewport
      4. Close, open avatar-crop dialog
      5. Assert dialog fits viewport
    Expected Result: All dialogs fit within viewport
    Evidence: .omo/evidence/task-10-dialogs-mobile.png
  ```

  **Commit**: NO (groups with 6-9, 11-12)

- [x] 11. **FileUpload — adicionar max-width: 100% na preview**

  **What to do**:
  - Em `src/app/shared/components/file-upload.component.ts:113-114`, alterar `.preview` de `max-width: 200px; max-height: 200px` para:
    ```css
    max-width: min(200px, 100%);
    max-height: min(200px, 50vh);
    object-fit: contain;
    ```
  - Adicionar `object-fit: contain` para que a imagem se ajuste sem distorcer

  **Must NOT do**:
  - NÃO mudar o layout do dropzone

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Tiny CSS change, single property addition
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 5, 10, 14, 17)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/shared/components/file-upload.component.ts:113-114` — .preview styles

  **Acceptance Criteria**:
  - [ ] `.preview` has `max-width: min(200px, 100%)` and `object-fit: contain`
  - [ ] `ng build` passes

  **Commit**: NO (groups with 6-10, 12)

- [x] 12. **Fontes px → rem nos componentes map-config-panel, map-view, submap-pin**

  **What to do**:
  - Converter `font-size` em px para rem nos seguintes componentes e linhas:
    - `src/app/features/map/map-config-panel.component.ts:73` — `font-size: 14px` → `0.875rem`
    - `src/app/features/map/map-config-panel.component.ts:97` — `font-size: 12px` → `0.75rem`
    - `src/app/features/map/map-view.component.ts:139` — `font-size: 13px` → `0.8125rem`
    - `src/app/features/map/map-view.component.ts:177` — `font-size: 13px` → `0.8125rem`
    - `src/app/features/map/submap-pin-dialog.component.ts:113` — `font-size: 13px` → `0.8125rem`

  **Must NOT do**:
  - NÃO mudar `font-size` de ícones Material (que usam tamanhos em px intencionalmente)
  - NÃO alterar outras propriedades CSS

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Mechanical conversion, 5 lines total
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 7, 8, 9, 15, 16)
  - **Blocks**: None
  - **Blocked By**: Wave 2 tasks

  **References**:
  - Same files listed above

  **Acceptance Criteria**:
  - [ ] All 5 `font-size` values converted to rem
  - [ ] Visual appearance unchanged (1rem = 16px default)
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: Font sizes converted correctly
    Tool: Bash
    Preconditions: Source files
    Steps:
      1. grep for 'font-size: 1[2-4]px' in map-config-panel, map-view, submap-pin
      2. Assert no matches found
      3. grep for 'font-size: 0.' in same files
      4. Assert matches found for each converted value
    Expected Result: No px font sizes remain in these files
    Evidence: .omo/evidence/task-12-font-conversion.txt
  ```

  **Commit**: NO (groups with 6-11)

- [x] 13. **NG8107 — corrigir file-upload e page-header**

  **What to do**:
  - `src/app/shared/components/file-upload.component.ts:41`:
    - `acceptedTypes?.join(', ')` → `acceptedTypes.join(', ')`
    - `acceptedTypes` é `string[]` com default `[]`, nunca é null/undefined
  - `src/app/shared/components/page-header.component.ts:24`:
    - `breadcrumbs?.length` → `breadcrumbs.length`
    - `breadcrumbs` é `BreadcrumbItem[]` com default `[]`, nunca é null/undefined

  **Must NOT do**:
  - NÃO mudar a assinatura dos @Input() — apenas remover o `?.` desnecessário

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Trivial fix, 2 characters removed each
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 18)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/shared/components/file-upload.component.ts:41`
  - `src/app/shared/components/page-header.component.ts:24`

  **Acceptance Criteria**:
  - [ ] `acceptedTypes?.join` → `acceptedTypes.join` in file-upload
  - [ ] `breadcrumbs?.length` → `breadcrumbs.length` in page-header
  - [ ] `ng build` shows 2 fewer NG8107 warnings

  **QA Scenarios**:
  ```
  Scenario: No more NG8107 on these two files
    Tool: Bash
    Preconditions: Source files
    Steps:
      1. Check file-upload template for acceptedTypes?.join
      2. Assert it uses acceptedTypes.join (no ?.)
      3. Check page-header template for breadcrumbs?.length
      4. Assert it uses breadcrumbs.length (no ?.)
    Expected Result: Both expressions use direct property access
    Evidence: .omo/evidence/task-13-ng8107-check.txt
  ```

  **Commit**: NO (groups with 1-5, 18)

- [x] 14. **NG8107 — corrigir gallery-lightbox (falso positivo Angular v21)**

  **What to do**:
  - Em `src/app/features/gallery/gallery-lightbox.component.ts`, `currentItem` é um `computed(() => ... ?? null)` que retorna `GalleryItem | null`
  - O Angular v21 template checker interpreta o retorno como não-nulável (falso positivo conhecido)
  - **Solução**: Adicionar cast explícito no retorno do computed:
    ```typescript
    readonly currentItem = computed((): GalleryItem | null => {
      const all = this.items();
      const idx = this.currentIndex();
      return (all[idx] ?? null) as GalleryItem | null;
    });
    ```
  - Isso força o template checker a reconhecer o tipo como `GalleryItem | null`, validando o `?.`

  **Must NOT do**:
  - NÃO remover o `?.` das expressões no template — `currentItem()` PODE ser null em runtime
  - NÃO usar `@ts-ignore` para suprimir o warning

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single line change, typed casting
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 5, 10, 11, 17)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/features/gallery/gallery-lightbox.component.ts:37,61,64,70,107` — 5 NG8107 occurrences
  - The `currentItem` computed signal definition

  **Acceptance Criteria**:
  - [ ] `currentItem` computed has explicit `GalleryItem | null` return type
  - [ ] `as GalleryItem | null` cast added to return value
  - [ ] `ng build` shows 5 fewer NG8107 warnings (or 0 from this file)

  **QA Scenarios**:
  ```
  Scenario: Template expressions still use optional chaining
    Tool: Bash (grep)
    Preconditions: Source file
    Steps:
      1. grep for 'currentItem()\?\.' in gallery-lightbox template
      2. Assert 5 matches (optional chaining preserved)
      3. Assert computed has 'as GalleryItem | null'
    Expected Result: Template safety preserved, warning resolved
    Evidence: .omo/evidence/task-14-gallery-ng8107.txt
  ```

  **Commit**: NO (groups with 15-17)

- [x] 15. **Tipar map-three.service.ts — substituir 14 `any` por tipos Three.js**

  **What to do**:
  - Em `src/app/features/map/map-three.service.ts`, substituir os 14 `any` por tipos adequados:
    - `_THREE: any` → `_THREE: typeof import('three') | null`
    - `_OrbitControls: any` → `_OrbitControls: typeof import('three/examples/jsm/controls/OrbitControls').OrbitControls | null`
    - `scene: any` → `scene: import('three').Scene | null`
    - `camera: any` → `camera: import('three').PerspectiveCamera | null`
    - `renderer: any` → `renderer: import('three').WebGLRenderer | null`
    - `controls: any` → `controls: import('three/examples/jsm/controls/OrbitControls').OrbitControls | null`
    - `grid: any` → `grid: import('three').GridHelper | null`
    - `mapPlane: any` → `mapPlane: import('three').Mesh | null`
    - `markerMeshes: any[]` → `markerMeshes: import('three').Mesh[]`
    - Parâmetros de callbacks: `(child: any)` → `(child: import('three').Object3D)`, etc.
  - Instalar `@types/three` se não estiver presente: `npm install --save-dev @types/three`

  **Must NOT do**:
  - NÃO mudar a lógica de carregamento dinâmico (import() dentro de init)
  - NÃO refatorar a estrutura do serviço
  - NÃO alterar métodos além da tipagem

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multiple type declarations, requires Three.js types understanding
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 7, 8, 9, 12, 16)
  - **Blocks**: None
  - **Blocked By**: Wave 2 tasks

  **References**:
  - `src/app/features/map/map-three.service.ts:24-33` — Main any-type properties
  - `src/app/features/map/map-three.service.ts:51` — Return type any
  - `src/app/features/map/map-three.service.ts:261,265,327,341` — Callback any params

  **Acceptance Criteria**:
  - [ ] All 14 `any` replaced with proper Three.js types
  - [ ] `ng build` passes
  - [ ] `@types/three` installed (if needed)
  - [ ] No runtime behavior changes

  **QA Scenarios**:
  ```
  Scenario: No more any in map-three.service.ts
    Tool: Bash (grep)
    Preconditions: Source file
    Steps:
      1. grep for ': any' in map-three.service.ts
      2. Assert 0 matches
      3. grep for 'import('three')' in same file
      4. Assert at least 5 type imports
    Expected Result: Zero any, proper Three.js types
    Evidence: .omo/evidence/task-15-map-three-types.txt
  ```

  **Commit**: NO (groups with 14, 16, 17)

- [x] 16. **Tipar rules-reader.component.ts — substituir 4 `any` por interfaces**

  **What to do**:
  - Em `src/app/features/rules/rules-reader.component.ts`, substituir os 4 `any`:
    - `items: any[]` (linha 31) → `items: PdfOutlineItem[]` (criar interface)
    - `private pdfDocument: any` (linha 379) → `private pdfDocument: PdfDocument | null` (criar interface)
    - `pdf: any` (linha 439) → `pdf: PdfDocument`
    - `outline: any` (linha 445) → `outline: PdfOutlineItem[]`
  - Criar interfaces locais no mesmo arquivo:
    ```typescript
    interface PdfDocument { numPages: number; getPage(pageNum: number): Promise<any>; getOutline(): Promise<PdfOutlineItem[]>; getDestination(dest: string): Promise<any>; getPageIndex(ref: any): Promise<number>; }
    interface PdfOutlineItem { title: string; dest: string | any[] | null; url: string | null; bold: boolean; italic: boolean; items?: PdfOutlineItem[]; }
    ```
  - Substituir `(window as any)['pdfWorkerSrc']` (linha 384) por declaração global:
    ```typescript
    declare global { interface Window { pdfWorkerSrc?: string; } }
    window.pdfWorkerSrc = 'pdfjs-dist/build/pdf.worker.min.mjs';
    ```

  **Must NOT do**:
  - NÃO modificar a lógica de carregamento do PDF.js
  - NÃO mover o side effect do constructor para APP_INITIALIZER (escopo reduzido)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Requires understanding pdf.js types and creating appropriate interfaces
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 6, 7, 8, 9, 12, 15)
  - **Blocks**: None
  - **Blocked By**: Wave 2 tasks

  **References**:
  - `src/app/features/rules/rules-reader.component.ts` — all lines listed above

  **Acceptance Criteria**:
  - [ ] 0 `any` remaining in rules-reader.component.ts
  - [ ] `(window as any)` replaced with `window.pdfWorkerSrc` (typed)
  - [ ] `ng build` passes
  - [ ] PDF reader still works

  **QA Scenarios**:
  ```
  Scenario: No more any in rules-reader
    Tool: Bash (grep)
    Preconditions: Source file
    Steps:
      1. grep for ': any' in rules-reader.component.ts
      2. Assert 0 matches
      3. grep for 'as any' in same file
      4. Assert 0 matches
      5. grep for 'interface PdfDocument' in same file
      6. Assert 1 match
    Expected Result: Clean types
    Evidence: .omo/evidence/task-16-rules-types.txt
  ```

  **Commit**: NO (groups with 14, 15, 17)

- [x] 17. **CharacterDetail — substituir non-null assertion error()! por error() ??**

  **What to do**:
  - Em `src/app/features/characters/character-detail.component.ts:61`, no template:
    - `[message]="error()!"` → `[message]="error() ?? 'Erro desconhecido'"`
  - Isso evita o potencial runtime crash se `error()` for null no momento da renderização

  **Must NOT do**:
  - NÃO mudar lógica de erro do componente
  - NÃO alterar outras partes do template

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single expression change
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 5, 10, 11, 14)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/features/characters/character-detail.component.ts:61`

  **Acceptance Criteria**:
  - [ ] `error()!` replaced with `error() ?? 'Erro desconhecido'`
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: No non-null assertion in template
    Tool: Bash (grep)
    Preconditions: Source file
    Steps:
      1. grep for 'error()!' in character-detail template
      2. Assert 0 matches
      3. grep for 'error() ??' in character-detail template
      4. Assert 1 match
    Expected Result: Safe null-coalescing operator used
    Evidence: .omo/evidence/task-17-nonnull-fix.txt
  ```

  **Commit**: NO (groups with 14, 15, 16)

- [x] 18. **DiceRollerDialog — padronizar *ngIf para @if**

  **What to do**:
  - Em `src/app/features/session/dice-roller-dialog.component.ts:56`, mudar:
    - `<button *ngIf="notation" matSuffix ...>` → `<button @if (notation) { <button matSuffix ...> }`
  - Garantir que todo o template use o novo control flow (`@if`, `@for`) de forma consistente

  **Must NOT do**:
  - NÃO alterar a lógica do componente
  - NÃO remover imports do CommonModule (pode ser usado em outros lugares)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple syntax change, no logic modification
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 13)
  - **Blocks**: None
  - **Blocked By**: None (can start immediately)

  **References**:
  - `src/app/features/session/dice-roller-dialog.component.ts:56` — *ngIf="notation"
  - Lines 77, 86, 113 — Already using @if (reference pattern)

  **Acceptance Criteria**:
  - [ ] No `*ngIf` in dice-roller-dialog template (only `@if`)
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: Template uses @if consistently
    Tool: Bash (grep)
    Preconditions: Source file
    Steps:
      1. grep for '\*ngIf' in dice-roller-dialog template
      2. Assert 0 matches
      3. grep for '@if' in template
      4. Assert at least 1 match
    Expected Result: Consistent control flow syntax
    Evidence: .omo/evidence/task-18-control-flow.txt
  ```

  **Commit**: NO (groups with 1-5, 13)

- [x] 19. **Remover SidebarService não utilizado**

  **What to do**:
  - Remover `src/app/core/services/sidebar.service.ts` (serviço nunca injetado em nenhum componente)
  - Remover a exportação do SidebarService no barrel `src/app/core/index.ts`
  - Verificar que nenhum arquivo importa de SidebarService (já confirmado: 0 referências além do barrel)

  **Must NOT do**:
  - NÃO remover outros serviços do barrel
  - NÃO modificar o ShellComponent (usa navItems hardcoded)
  - NÃO criar funcionalidade substituta

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: File deletion and barrel cleanup — simple but needs careful verification
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 20)
  - **Blocks**: None
  - **Blocked By**: All Wave 3 tasks

  **References**:
  - `src/app/core/services/sidebar.service.ts`
  - `src/app/core/index.ts` — barrel export

  **Acceptance Criteria**:
  - [ ] `sidebar.service.ts` deleted
  - [ ] No references to SidebarService in barrel
  - [ ] `ng build` passes
  - [ ] `ng test` passes

  **QA Scenarios**:
  ```
  Scenario: SidebarService fully removed
    Tool: Bash
    Preconditions: Source files
    Steps:
      1. Test-Path sidebar.service.ts → Assert False
      2. grep for 'SidebarService' across src/ → Assert 0 matches
      3. ng build → Assert passes
    Expected Result: No trace of SidebarService remains
    Evidence: .omo/evidence/task-19-sidebar-removed.txt
  ```

  **Commit**: NO (groups with 20)

- [x] 20. **Remover map.component.ts (arquivo morto)**

  **What to do**:
  - Remover `src/app/features/map/map.component.ts` (só exporta `MAP_COMPONENT_DEPRECATED = true`)
  - Verificar que nenhum arquivo importa `MAP_COMPONENT_DEPRECATED` ou `map.component`
  - Se houver imports, atualizá-los ou removê-los

  **Must NOT do**:
  - NÃO remover `map-view.component.ts` (o componente real de mapa)
  - NÃO modificar as rotas de mapa

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single file removal, constant export
  - **Skills**: (none needed)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 19)
  - **Blocks**: None
  - **Blocked By**: All Wave 3 tasks

  **References**:
  - `src/app/features/map/map.component.ts` — dead file

  **Acceptance Criteria**:
  - [ ] `map.component.ts` deleted
  - [ ] No references to `MAP_COMPONENT_DEPRECATED` remain
  - [ ] `ng build` passes

  **QA Scenarios**:
  ```
  Scenario: Dead map file removed
    Tool: Bash
    Preconditions: Source files
    Steps:
      1. Test-Path map.component.ts → Assert False
      2. grep for 'MAP_COMPONENT_DEPRECATED' across src/ → Assert 0 matches
      3. ng build → Assert passes
    Expected Result: Dead code removed
    Evidence: .omo/evidence/task-20-map-removed.txt
  ```

  **Commit**: NO (groups with 19)

---

## Final Verification Wave (MANDATORY)

> 4 review agents run in PARALLEL. ALL must APPROVE. Wait for user's explicit "okay" before completing.

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, curl endpoint, run command). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .omo/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `ng build` + `ng test`. Review all changed files for: lint issues, `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [x] F3. **Real Manual QA** — `unspecified-high` (+ `playwright` skill)
  Start from clean state. Execute EVERY QA scenario from EVERY task — follow exact steps, capture evidence. Test cross-task integration and edge cases. Save to `.omo/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff (git log/diff). Verify 1:1 — everything in spec was built (no missing), nothing beyond spec was built (no creep). Check "Must NOT do" compliance.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy
- **1-5, 13, 18**: `fix(core): ativa persistência de dados e corrige interações quebradas`
- **6, 7, 8, 9, 10, 11, 12**: `fix(ui): corrige responsividade crítica para mobile`
- **14, 15, 16, 17**: `fix(types): resolve warnings NG8107 e tipa any`
- **19, 20**: `chore(cleanup): remove código morto`
- **F1-F4**: `chore(qa): verificação final e ajustes`

---

## Success Criteria

### Verification Commands
```bash
ng build                      # Expected: 0 errors (NG8107 warnings tolerated)
ng test                       # Expected: all tests pass
```

### Final Checklist
- [ ] Dados persistem após reload completo (Playwright)
- [ ] HomeComponent botões navegam corretamente
- [ ] CharacterList empty state action navega para /personagens/novo
- [ ] SessionQuickSearch cabe em viewport 375px
- [ ] SessionCockpit empilha painéis em viewport <768px
- [ ] CharacterSheet quebra linhas em mobile
- [ ] RulesReader sidebar colapsa em mobile
- [ ] 0 NG8107 warnings residuais
- [ ] Código morto removido
