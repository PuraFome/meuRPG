## O que muda
Uma ou duas frases.

## História e regras
MR-xxx · RN-xx

## Como testei
- [ ] Testes automáticos novos ou atualizados
- [ ] Testei na tela (com prints, se algo visível mudou)

## Telas (se algo visível mudou)
- [ ] Segue o `docs/design.md` (tokens e peças comuns, nenhuma cor escrita à mão)
- [ ] Prints de antes e depois a 390px e 1280px, no tema claro e no escuro
- [ ] Todo estado da tela conferido: vazio, carregando, erro e os de permissão
- [ ] Olhei cada peça de perto (2x): ícones na altura das palavras, nada grudado na borda, nada por cima de outra coisa
- [ ] Alvos de toque de pelo menos 44px, foco visível, nada vaza para o lado a 320px
- [ ] `npx playwright test --grep @a11y` passa: sem violação séria ou crítica, e sem falha de alinhamento (`e2e/tests/layout.ts`)
- [ ] Texto em português, em caixa normal, botões dizendo o que fazem

## Documentação
- [ ] Atualizei os documentos afetados, ou não há mudança de comportamento
- [ ] Decisão difícil de desfazer? Abri um ADR no repositório privado
- [ ] Mexe com dado pessoal, logs, imagens ou fornecedor? Segui o checklist de privacidade (`docs/privacidade.md`)
