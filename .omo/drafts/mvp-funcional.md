# Draft: MVP Funcional — Correção de Bugs Críticos + Conformidade com SRS

## Contexto da Sessão
- Plano anterior `responsividade-correcao`: 20/20 tarefas `[x]`, onda final F1-F4 em aberto
  - F1 (Plan Compliance): APPROVE (2026-07-29)
  - F2 (Code Quality): APPROVE (2026-07-29, 2026-07-30)
  - F3 (Real Manual QA): ABORTED — nunca completou (usuário fez QA manual ele mesmo)
  - F4 (Scope Fidelity): APPROVE após correção de atribuição (2026-07-31)
- Último commit: `c296f3a` (2026-07-28) — TODO o trabalho dos dois planos está UNCOMMITTED no working tree
- **F3 abortado + usuário encontrou bugs reais no QA manual** → necessidade de novo plano de correção

## Bugs Reportados pelo Usuário (QA manual)
1. **Erro ao carregar mapa** — navegar para /mapa falha (causa a investigar — agente bg_d50284d6 em andamento)
2. **Sem ícones para as páginas** — ÍCONES: ROOT CAUSE CONFIRMADA (bg_e2212323)
3. **Sem cadastro de personagens** — CADASTRO: ROOT CAUSE CONFIRMADA (bg_9a79acbd)
4. **"etc"** — outros problemas encontrados no QA manual (cobertura via auditoria SRS bg_99deb0e8 em andamento)

## DIAGNÓSTICO CONFIRMADO: Ícones (bg_e2212323)
- **Root cause**: `src/index.html:10` carrega Material Icons **Outlined**, mas MatIcon usa a classe default `material-icons` (regular) — `_icon-registry-chunk.mjs:43`. A classe `.material-icons` (única que define `font-family: 'Material Icons'`) NUNCA é carregada. Angular Material NÃO define font-family nos ícones (só token `--mat-icon-color`). Resultado: ligaturas ("home", "menu") renderizam como texto literal/Roboto.
- **Escopo**: 27 componentes, 113 usos de `<mat-icon>`, ~65 ligaturas únicas (todas padrão).
- **FIX (escolha do usuário = Material Icons font)**:
  - Opção A (1 linha, recomendada): `index.html:10` trocar `family=Material+Icons+Outlined` → `family=Material+Icons`
  - Opção B: manter Outlined + `MatIconRegistry.setDefaultFontSetClass('material-icons-outlined')`
  - Opção C (offline-proof): self-host woff2 em `public/fonts/` + `@font-face` + `.material-icons` em styles.scss

## DIAGNÓSTICO CONFIRMADO: Cadastro de Personagens (bg_9a79acbd)
- **Root cause**: rota `/personagens/novo` existe (`characters.routes.ts:10-13`) mas carrega `CharactersComponent` = placeholder de 8 linhas `<p>Personagens — em construção</p>` (`characters.component.ts:6`). NÃO EXISTE nenhum componente de formulário de criação no repo (grep 0 matches para character-form/new/edit).
- **Cadeia quebrada**: botões (list:43-46, list:326, home:14) → rota OK → **CharactersComponent (DEAD END)** → nenhum form → `store.set('characters', ...)` NUNCA chamado (só `patch`/`update` em detail:464/478/572/643 e sheet:540, que exigem id existente).
- **O que JÁ funciona**: navegação, registro de rota (novo antes de :id), `store.set()` (store.service.ts:40), persistência (`meurpg_characters`, persistence.service.ts:48-55), reatividade da lista (list:274).
- **FIX**: criar `character-form.component.ts` (nome required, type select npc/player/boss, description opcional; submit → `crypto.randomUUID()` id + `store.set('characters', char)` + navigate `['/personagens', id]`), apontar rota `novo` para ele, adicionar spec.
- **Nota latente**: Dates (`createdAt`/`updatedAt`) viram string após round-trip JSON do localStorage — considerar parse no load (persistence.service.ts:37-39).

## Requisitos do Usuário
- Documento SRS completo fornecido (RF-01 a RF-22) — base para "deixar o MVP funcional"
- Stack: Angular, HTML5, SCSS, Angular Material
- "revise tudo com base nos seguintes requisitos e faça um plano para deixar esse MVP funcional"

## SRS Resumido (RF-01 a RF-22)
- 2.1 Personagens: RF-01 tipos dinâmicos (NPC/Jogador/Boss), RF-02 cabeçalho com upload+crop, RF-03 História (Quill), RF-04 Ficha (FormArray), RF-05 Notas do Mestre (role), RF-06 Falas (CRUD+copiar)
- 2.2 Campanha: RF-07 pastas infinitamente aninhadas (drag-drop), RF-08 associação de entidades
- 2.3 Galeria: RF-09 organização em pastas, RF-10 Lightbox, RF-11 Audio Player (play/pause/volume/loop)
- 2.4 Mapa: RF-12 Leaflet/OpenLayers + Three.js, RF-13 alternância 2D/3D, RF-14 camadas (grid, fog, markers mestre), RF-15 submapas via pins
- 2.5 Regras: RF-16 upload .pdf, RF-17 ng2-pdf-viewer/pdf.js, RF-18 índice lateral + paginação
- 2.6 Sessão: RF-19 toolbar atalhos (Rolar Dados, Abrir Mapa, Tocar Música), RF-20 busca rápida com modal sobreposto
- 2.7 Dashboard: RF-21 cards últimas campanhas/personagens recentes, RF-22 compêndio com busca de PDFs

## Arquitetura (do SRS)
- Lazy Loading por módulo
- Estado: RxJS BehaviorSubjects ou NgRx
- Global Search: Ctrl+K modal
- Smart vs Dumb Components (dica de implementação)

## Decisões do Usuário (2026-07-31)
- [x] **Escopo**: Auditoria SRS COMPLETA — corrigir TODOS os RFs quebrados/faltantes (RF-01 a RF-22), não só os 3 bugs
- [x] **Dashboard**: INCLUIR RF-21 (cards últimas campanhas/personagens recentes) + RF-22 (widget compêndio com busca de livros)
- [x] **Ícones**: Material Icons (font) — Google Fonts + MatIconRegistry, padrão Angular Material
- [ ] Re-verificação final (onda F1-F4) deve ser incluída no novo plano? → (default: SIM, obrigatório — onda final sempre incluída)
- [ ] Commits: trabalhar nos commits não commitados existentes ou novo branch? → (default: commit final agrupado por wave, como plano anterior)

## Estado Técnico Conhecido
- `ng build` PASS (0 erros, 0 NG8107), `ng test` 33/33 PASS (verificado F2 em 2026-07-30)
- Working tree: 36 modificados + 12 untracked (união dos 2 planos)
- `@types/three` instalado; Three.js/OpenLayers com dynamic import
