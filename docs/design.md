# Design: o visual do MeuRPG

O MeuRPG tem o visual da **ficha de papel**: a ficha oficial de D&D 5e, com a mesma ordem e as mesmas formas (medalhões de atributo, o escudo da CA, os pontos de proficiência), limpa e legível na tela, com um tema claro e um escuro de verdade. É a direção A das três exploradas em 29/09/2026; o Vinicius escolheu esta.

Este arquivo é a referência para quem mexe em qualquer tela do `web/`. O mesmo sistema existe no Claude Design, com os componentes desenhados, para desenhar tela nova antes do código:

- [MeuRPG — sistema visual](https://claude.ai/artifact/3wg7GmD4uuDEXhNP57ESdu) (Claude Design System);
- [MeuRPG — direções visuais](https://claude.ai/artifact/VDbSC4Y9H1z5vviHHL5nQm) (as três direções, com a ficha e a campanha de cada uma).

Os dois links são privados até o Vinicius compartilhar.

## Princípios

- **O número de jogo vem primeiro.** Modificadores, CA, PV e CD são a coisa mais visível da ficha. Todo o resto é mais quieto.
- **A ficha fica onde o jogador está acostumado** (MR-004): atributos à esquerda, depois salvaguardas e perícias, combate e magias no meio, características e história à direita. No celular, a mesma ordem numa coluna só.
- **Uma ousadia só.** O grená aparece uma vez por tela como preenchimento (o botão principal) e, fora disso, como contorno (CA, PV) ou texto (links). Nada de gradiente, sombra decorativa ou cartão com faixa colorida na lateral.
- **Estrutura é informação.** Borda, painel e divisória separam coisas diferentes de verdade. Numeração só onde há sequência (os passos do editor).
- **Os dois temas valem o mesmo.** O escuro é para a sessão à noite: cada cor tem par nos dois temas, com contraste AA. O tema segue o do sistema operacional.

## Como uma tela é feita

Toda tela nova, ou mudança visível numa tela, passa por quatro passos:

1. **Brief:** a única tarefa da tela, quem usa (mestre ou jogador), onde (celular na mesa ou notebook), os dados reais e todos os estados (vazio, carregando, erro, travado, pendente, morto, sem permissão).
2. **Desenho no Claude Design,** com o sistema acima, a 390px e a 1280px, com dados reais. O Vinicius aprova antes do código. Correção pequena dentro do sistema não precisa de desenho.
3. **Código com os tokens** abaixo, nunca uma cor escrita à mão.
4. **Revisão pela tela:** prints a 390, 768 e 1280px, claro e escuro, cada estado, conferidos com o desenho e com a checklist do PR (hierarquia, sobreposição, alvos de toque, foco, contraste, texto). O PR leva os prints de antes e depois.

## Tokens

Os tokens são propriedades CSS em `web/src/styles.scss`, com o prefixo `--mr-`. Cada cor usa `light-dark()`, então o mesmo token vale nos dois temas.

### Cor

| Token | Claro | Escuro | Uso |
| --- | --- | --- | --- |
| `--mr-ground` | `#eef0f3` | `#11151c` | Fundo da página |
| `--mr-surface` | `#ffffff` | `#1a1f29` | Painéis, barra do app, campos |
| `--mr-line` | `#d5dae1` | `#2e3542` | Borda dos painéis (decorativa) |
| `--mr-rule` | `#e3e7ec` | `#262c38` | Divisória entre linhas de um painel |
| `--mr-ink` | `#1b2230` | `#e6e9ef` | Texto; borda dos medalhões; ponto de proficiência cheio |
| `--mr-ink-muted` | `#525b6b` | `#a3abb9` | Rótulos e texto de apoio (6,8:1 e 7,1:1) |
| `--mr-control-line` | `#8a93a3` | `#6b7485` | Borda de campo, ponto vazio, espaço de magia (3:1) |
| `--mr-accent` | `#9e2b3b` | `#e88593` | Grená: botão principal, contorno da CA e dos PV, aba ativa, foco |
| `--mr-accent-strong` | `#7a1f2d` | `#f2a9b3` | Hover e pressed do principal |
| `--mr-on-accent` | `#ffffff` | `#2a0a10` | Texto sobre o grená |
| `--mr-accent-text` | `#9e2b3b` | `#f0a0ab` | Links e botões de texto |
| `--mr-accent-soft` | `#f6e3e6` | `#3a1e24` | Item selecionado |
| `--mr-focus` | = accent | = accent | Anel de foco de 2px |
| `--mr-warning-surface`, `-line`, `-ink` | `#fbf3e2`, `#e6cf9f`, `#6e4700` | `#2e2616`, `#6b5320`, `#f0d08a` | Pendências de regra e o estado Pendente |
| `--mr-danger-surface`, `-ink` | `#fce8e6`, `#8c1d18` | `#3b1a1a`, `#f4b4ae` | Erro e o estado Morto |
| `--mr-success-surface`, `-ink` | `#e3f2ea`, `#1f6b45` | `#16301f`, `#9ed8b4` | Confirmação e o estado Aprovado |

Estado nunca é só cor: toda etiqueta tem a palavra, e todo aviso tem ícone e texto.

### Tipografia

Duas famílias com licença OFL, servidas pelo próprio app (pacotes `@fontsource/alegreya` e `@fontsource/alegreya-sans`, só o subconjunto latino, que cobre o português). Nunca Google Fonts: o CSP (`font-src 'self'`) e a [privacidade](privacidade.md) proíbem pedir fonte a terceiros.

| Estilo | Família | Tamanho / altura / peso | Uso |
| --- | --- | --- | --- |
| `display-xl` | Alegreya (`--mr-font-display`) | 48 / 48 / 800 | Nome do personagem (38 no celular) |
| `display-l` | Alegreya | 38 / 40 / 800 | Título da página, `.mr-page-title` (34 no celular) |
| `title` | Alegreya | 21 / 26 / 700 | Título de painel, `.mr-panel__title` |
| `stat-xl` | Alegreya | 40 / 40 / 800 | CA e PV máximos |
| `stat` | Alegreya | 38 / 42 / 700 | Modificadores, iniciativa, deslocamento |
| `body` | Alegreya Sans (`--mr-font-sans`) | 16 / 22 / 400 | Texto e linhas de lista |
| `field-value` | Alegreya Sans | 19 / 24 / 500 | Campos do cabeçalho da ficha |
| `small` | Alegreya Sans | 14 / 19 / 400 | Apoio |
| `label` | Alegreya Sans | 13 / 16 / 500 | Rótulos, `.mr-label`, nunca em caixa alta |
| `button` | Alegreya Sans | 16 / 20 / 700 | Botões |

Números usam `font-variant-numeric: lining-nums tabular-nums` (já vem do `body`), para as colunas de bônus alinharem.

### Espaço e forma

- Espaços: `--mr-space-1` 4, `-2` 8, `-3` 12, `-4` 16, `-5` 20, `-6` 24, `-8` 48 (px). `--mr-gutter` é a margem lateral: 16 no celular, 48 a partir de 768px.
- Raios: `--mr-radius-sm` 8 (botões, campos), `-md` 10 (avisos), `-lg` 12 (painéis), `-xl` 14 (medalhões, cabeçalho), `-pill` (etiquetas).
- Bordas, não sombras. A única sombra é a de um diálogo aberto.
- Layout: largura máxima de 1280px. A ficha tem quatro colunas no desktop, duas no tablet e uma no celular. Alvos de toque de pelo menos 44px (48 no botão principal do celular).

## Componentes

As peças comuns a várias telas são classes globais em `web/src/styles/_ui.scss`; o que é de uma tela só fica no componente dela.

| Peça | Como usar |
| --- | --- |
| Título da página | `<h1 class="mr-page-title">` e, embaixo, `<p class="mr-page-lead">` |
| Painel | `<section class="mr-panel">` com `<h2 class="mr-panel__title">` |
| Lista de linhas | `<ul class="mr-list">`; cada item com `<a class="mr-list__row">`, `.mr-list__text`, `.mr-list__name`, `.mr-list__sub` |
| Etiqueta de estado | `<span class="mr-tag">`, com `--pending`, `--success` ou `--danger`; Travada leva o ícone `lock` |
| Aviso | `<div class="mr-notice mr-notice--warning">` (ou `--danger`, `--success`, `--neutral`), com um `mat-icon` e um `<p>` que começa com `<strong>` |
| Botões | Material: `mat-flat-button` para o principal (um por tela), `mat-stroked-button` para os de apoio, `mat-button` para os de texto. Todos com canto de 8px |
| Texto só para leitor de tela | `.mr-visually-hidden` |
| "Ao vivo" | `<app-live-pill>` (`shared/live-pill`): um ponto `accent` de 8px e a palavra, numa pílula com borda e texto `accent-text`. Na barra do app é um link para a sessão aberta mais nova, com `aria-label` "Ao vivo: sessão 4 de Mirathel" (ao lado do menu no celular, antes da conta no desktop; some nas páginas de sessão). Em "Minhas campanhas" vira a etiqueta "Sessão ao vivo" (`size="small"`, 26px), em cima da etiqueta do papel. Na página da sessão abre a linha de estado. Não pisca |
| Aviso de sessão | `<app-live-notice>` (`shell/live-notice`): variante de aviso com fundo `surface`, borda de 1,5px `accent` (a mesma do painel "Sessão": moldura grená quer dizer sessão), o ícone `sensors`, uma frase em negrito ("A sessão 4 de Mirathel começou."), "Entrar na sessão" com contorno e o botão de fechar de 44px ("Fechar aviso"). Fica embaixo da barra do app, com `role="status"`, para quem joga na campanha, fora da página dela e das páginas de sessão; fechar vale só para a aba. Com mais de uma sessão aberta, um aviso por sessão, o mais antigo em cima: o que começa depois entra embaixo, e nenhum aviso muda sob o dedo de quem vai tocar em "Entrar na sessão". O botão não é cheio porque o aviso cai em telas que já têm o seu |
| PV da sessão | O bloco de combate da ficha com os valores atuais (`pages/live-session/player-vitals`): a caixa de PV com borda de 2px `accent`, o atual em 52px, "de 23" e uma barra de 8px (`ink` sobre `line`, `aria-hidden`: os números dizem tudo); o escudo da CA, "PV temporários" e "Dados de vida"; e "Espaços de magia", um anel vazio para o espaço livre e um disco cheio riscado para o usado, sempre com as palavras ("2 de 4 usados"). Na linha do mestre ("Grupo"), o atual em 30px, uma barra de 6px e a palavra quando precisa: "Abaixo da metade" (atual × 2 < máximo) ou "Inconsciente" (0) |
| Ajuste de números | O mesmo conteúdo numa folha de baixo no celular e num diálogo de 440px no desktop (`adjust-vitals`). Cada número tem os botões de passo em volta de um campo onde digitar já define o valor: "−5 −1 [campo] +1 +5" para os PV, "− [campo] +" para o resto. No limite, o botão fica desabilitado (40% de opacidade) e as palavras ao lado dizem por quê ("máximo 24", "0 de 3 usados"). Um valor digitado fora do limite não é corrigido escondido: o campo diz "Use um número de 0 a 24." e "Salvar ajuste", o único botão cheio, não envia |

Regras que valem em toda tela:

- **Etiqueta de estado é uma palavra:** Rascunho, Pendente, Travada, Aprovado, Morto; convites Ativo, Usado, Expirado, Revogado. O aviso ao lado explica o resto ("Esperando a aprovação do mestre").
- **Pendência não é lembrete.** Na ficha, o que o motor de regras aponta como problema (armadura sem proficiência, magia fora do grimório) vira um aviso só, logo abaixo do cabeçalho, cada item começando pelo problema em negrito. O que é só lembrete (vantagem contra magia, por exemplo) fica, quieto, em "Características e traços".
- **Ação que não se desfaz pede confirmação na própria tela:** "Marcar como morto" vira "Confirmar morte", e "Recusar personagem" vira "Confirmar recusa", com "Cancelar" ao lado e o foco no botão novo.
- **Formulário em passos** (o editor da ficha): cada passo é uma aba, com o nome no cabeçalho, e o conteúdo começa com "Passo N de M" e o título. No celular, as abas viram números de 44px e o título do passo aparece no conteúdo. O botão de salvar fica embaixo do passo aberto, em qualquer passo; um passo com campo inválido ganha "(com erro)" na aba, e o envio leva ao primeiro deles.
- **Um botão cheio por tela.** Quando a tela tem duas ações importantes (o mestre olhando um personagem pendente), a principal ("Aprovar personagem") é a cheia e a outra fica com contorno. A página da sessão do mestre não tem nenhum: as ações dela se repetem por linha ("Ajustar"), e o cheio fica onde se decide, o "Salvar ajuste". "Confirmar encerramento" usa contorno com a cor de perigo, para não virar o segundo cheio da página da campanha.
- **Contagem diz "usados":** "1 de 3 usados", nos espaços de magia, nos dados de vida e no ajuste.

Na ficha: o medalhão de atributo, o escudo da CA, as caixas de número, as linhas de proficiência e os campos do cabeçalho, desenhados no sistema visual do Claude Design.

### Galeria e imagens

As peças da galeria (MR-019, desenhos E5-20 a E5-22 e E5-31). As que outras telas reusam ficam em `web/src/app/shared/gallery-picker/`.

| Peça | Como é |
| --- | --- |
| Cartão da galeria (`pages/gallery/gallery-card`) | A miniatura de 480 px, cortada para preencher, o nome (17/700), "2000 × 1400 px, 1,5 MB" e onde a imagem é usada ("Usada em Mirathel e arredores", com o ícone de mapa, ou "Ainda não usada", a partir de `used_in_maps`). Com mouse, "Ver / Renomear / Apagar" cobrem os 48 px de baixo da miniatura no hover ou no foco do teclado, sem mexer no resto (num cartão estreito, de tablet, fica "Renomear / Apagar"); no toque (`hover: none`), "Renomear / Apagar" ficam sempre embaixo do cartão, e tocar na miniatura é "Ver". Renomear troca o nome por um campo, com "Salvar nome" e "Cancelar" |
| Envio em andamento (`shared/gallery-picker/upload-progress`) | O cartão do arquivo no começo da grade, com "Enviando ruinas.jpg… 60%" numa linha só (um nome longo termina em "…", a porcentagem fica), uma barra determinada (`role="progressbar"`, trilho `rule`, preenchimento `ink`) e "Cancelar envio". Sem prévia do arquivo: o CSP só aceita imagem da própria origem. Quando chega, vira o cartão da imagem, e um `role="status"` diz "Imagem enviada" |
| Erro de envio | Um aviso `--danger` por arquivo, no topo: "**Não deu para enviar mapa-antigo.gif.** Esse arquivo não é uma imagem JPEG, PNG ou WebP." O texto vem do `reason` do servidor, nunca da mensagem dele |
| Janela da imagem | Um `<dialog>` nativo: a imagem inteira (`/images/<id>`) sobre `ground`, o nome, "Renomear", "Apagar", "Fechar", e "Imagem anterior", "2 de 5", "Próxima imagem" (← e → também). É a única sombra do app (0 24px 64px), sobre um véu `ink` a 55% no claro e preto a 60% no escuro, onde uma borda `line` desenha a beirada. Enquanto a imagem inteira chega, a miniatura (já no cache) segura o lugar. No celular, ocupa a tela, os botões dizem "Anterior" e "Próxima", e o dedo passa a imagem |
| Seletor de imagem (`shared/gallery-picker`) | Um `radiogroup` das miniaturas (as setas andam e escolhem, Tab entra na escolhida). A escolhida tem borda de 2 px `accent`, rodapé `accent-soft` e um check: nunca só a cor. O último bloco, tracejado, é "Enviar imagem", que envia e já escolhe a imagem nova. Uso: `<app-gallery-picker [campaignId] [(selectedId)] label (picked)>`; o formulário em volta dá o rótulo visível e o erro (`describedBy`) |
| Painel "Galeria" da campanha (`pages/campaign-detail/gallery-panel`) | Só para o mestre (E5-09): as 5 imagens mais novas numa linha (3 no celular), em molduras 4:3 com o nome embaixo, "5 imagens, 5,8 MB de 500 MB" e "Abrir galeria" contornado |
| Lembrete de privacidade (`ImagePrivacyNote`) | Ao lado de todo "Enviar imagem", sempre com as mesmas palavras: "Use imagens do jogo. Não envie fotos de pessoas sem a autorização delas." ([Privacidade](privacidade.md)). Uma linha quieta, com o ícone `info`, como o aviso de ficção dos campos de texto |

Regras da galeria:

- **Apagar uma imagem que é o fundo de um mapa** o servidor recusa, e o cartão diz quais mapas, pelo detalhe `ImageInUse`: "Essa imagem é o fundo de Mirathel e arredores. Troque a imagem do mapa antes de apagá-la."
- **O botão cheio é "Enviar imagem".** Por isso a confirmação de apagar é contornada em `danger-ink` ("Apagar imagem"), com o foco nela, e "Cancelar" ao lado.
- **A cota** ("5 imagens · 5,8 MB de 500 MB") vira um aviso `--warning` a partir de 90% de qualquer um dos dois limites.
- **Galeria vazia** é um painel que convida ao primeiro envio ("Nenhuma imagem ainda"), no lugar da área de arrastar; do tablet para cima, ele também é tracejado e diz "Ou arraste as imagens para cá".
- **Cada cartão tem a própria altura:** renomear ou confirmar a exclusão num cartão não estica os vizinhos.
- **Arrastar arquivos** funciona em qualquer ponto da página; a área de envio ganha a borda tracejada `accent` enquanto o arquivo está em cima.

### Documento da campanha

As peças do documento (MR-018, desenhos E5-27 a E5-30), em `web/src/app/pages/campaign-document/`. O texto é Markdown, lido e desenhado por um parser nosso (`web/src/app/shared/markdown/`), sem dependência nova e sem `innerHTML`.

| Peça | Como é |
| --- | --- |
| Leitura (`document-read`) | O texto em 17/27 px, numa coluna de uns 70 caracteres (80 px de margem de cada lado no desktop, 16 px no celular), dentro de um painel. Títulos em Alegreya 25/31 (23/29 no celular). A imagem sai 56 px da coluna no desktop e vai de ponta a ponta no celular, com a legenda em 14 px `ink-muted` (`<figure>`/`<figcaption>`). "Editar documento" é o botão cheio da tela |
| Sumário (`document-toc`) | Os títulos `##`/`#`. Coluna fixa à esquerda no desktop, com a seção lida em `accent-soft` e negrito (`aria-current="location"`); no celular, um painel fechado com linhas de 44 px. Escolher uma linha leva ao título e põe o foco nele |
| Links do app | `[texto](mapa:<id>)` e `[texto](ficha:<id>)` são botões com cara de link (`accent-text`, sublinhado, ícone de mapa ou de ficha, `aria-haspopup="dialog"`) que abrem uma janela. Se o alvo foi apagado, vira texto com "(mapa apagado)" ou "(ficha apagada)"; uma imagem apagada vira a caixa neutra "Imagem apagada". Link `https://` abre em outra aba; qualquer outra coisa (HTML, `javascript:`, imagem de fora) fica como texto |
| Janelas (`doc-dialog`) | Um `<dialog>` nativo com ícone, título (Alegreya 24), uma linha embaixo, "Fechar" e rodapé; mesma sombra e mesmo véu da janela da imagem. O mapa mostra o nome, a imagem e "Abrir mapa"; a ficha, o nome, classe e raça e "Abrir ficha". Ao fechar, o foco volta ao link |
| Edição (`document-editor`) | Um `<textarea>` monoespaçado (a pilha do sistema) ao lado da prévia viva (uns 150 ms depois de digitar); no celular, as abas "Texto" e "Prévia". Em cima: "Rascunho não salvo" (`warning-ink`) ou "Sem mudanças", "Descartar mudanças" (confirma na própria tela) e "Salvar documento" (cheio). Um contador ("184 KB de 200 KB") aparece a partir de 90% do limite de 204.800 bytes. O rascunho só existe na memória; sair com ele pergunta antes ("Sair sem salvar?"), nunca por `beforeunload` |
| Barra de ferramentas (`document-toolbar`) | `role="toolbar"`: Tab entra num botão só, as setas (e Home/End) andam. Título, Negrito, Itálico, Lista escrevem no cursor; "Imagem da galeria" (o seletor compartilhado), "Link para mapa" e "Link para ficha" abrem uma janela com a lista, e ninguém digita um ID |
| Conflito | Se outra aba salvou antes, um aviso `--danger` ("Este documento mudou em outra aba ou em outro aparelho.") com "Recarregar"; o rascunho continua na tela até a pessoa escolher |
| Painel "Documento da campanha" (`pages/campaign-detail/document-panel`) | Só para o mestre: "Mirathel — preparação", "Editado ontem às 22:10. Só você vê este documento." e "Abrir documento" contornado |

### Mapas e imagem mostrada

As peças dos mapas (MR-008, MR-009, MR-012, desenhos E5-02 a E5-06 e E5-23 a E5-26) e de "mostrar uma imagem" (MR-028, E5-10 a E5-13). O mapa é um só componente, `shared/map-view`, que o editor, o visualizador do jogador e a página da sessão usam.

| Peça | Como é |
| --- | --- |
| Mapa (`shared/map-view`) | A imagem do mapa numa caixa com a proporção dela (`image.width/height`), então nada mexe enquanto ela carrega; a caixa nunca passa de uns 78% da altura da tela, e uma planta alta fica no meio, sem esticar a página. Pontos e tokens ficam em pontos-base (0 a 10000) como porcentagem. Zoom de 100% a 400% sem biblioteca: roda do mouse em volta do ponteiro, pinça com dois dedos (um dedo ainda rola a página), arrastar um lugar vazio para andar, e os botões de 44 px "Reduzir", "Ampliar" e "Ajustar à tela". Quatro modos: `preview` (imagem parada, a prévia da sessão), `view` (anda e dá zoom, os pontos abrem), `tokens` (também arrasta os tokens, a sessão do mestre) e `edit` (também arrasta os pontos, o editor). Respeita `prefers-reduced-motion` (não há movimento nele) |
| Marcador de ponto | Forma pelo tipo: losango com espadas cruzadas (Batalha), quadrado arredondado com escada (Submapa), círculo com balão (Cena de RP). 34 px, `surface` com borda de 2 px `ink`; a área de toque é de 44 px. Escondido: borda tracejada, ícone `ink-muted` e o selo `visibility_off` (só o mestre vê). Escolhido: borda de 3 px `accent`. É um botão (o nome é "Taverna do Javali, Cena de RP, escondido"); na prévia é só desenho, sem leitor de tela |
| Rótulo | Uma pílula `surface` com borda `line`, 13/700, 24 px, embaixo do marcador (ao lado quando faltaria espaço na borda). Escondido: tracejada, "Nome | Escondido" com o ícone. Escolhido: borda de 2 px `accent` |
| Token | Disco `ink` de 26 px com a inicial (Alegreya 700) e um halo de 2 px `surface`; duas letras quando dois tokens do mapa começam igual. Escondido (só o mestre): vazado, tracejado, com o selo. O do próprio jogador tem halo `accent` e a legenda diz "(você)". Só é botão onde o mestre pode arrastar; para os outros, não pega o clique do marcador que está embaixo |
| Pilha | Imagem, pontos, tokens, rótulos, o item escolhido ou sob o ponteiro, os controles. Passar o mouse ou focar traz um item sobreposto para cima; as listas são o caminho sem ponteiro |
| Legenda | Embaixo de todo mapa do mestre: as três formas, "Revelado" (sólido) e "Escondido" (tracejado com selo), e os tokens com o nome, "escondido" no que está escondido. Para o jogador, só os tokens, com "(você)" |
| Editor (`pages/maps/map-editor`) | Só no computador (E5-23). Uma barra: "Adicionar ponto" (Batalha, Submapa, Cena de RP, em `aria-pressed`), o menu "Adicionar token" (quem ainda não está no mapa) e o zoom com a porcentagem. Escolher o tipo e clicar no mapa cria o ponto ali, escondido e escolhido, e o foco vai para "Nome". Arrastar um ponto ou token, ou as setas no escolhido (0,5%; Shift 5%), e a posição vai ao soltar ou ao largar a tecla, um movimento de cada item por vez (enquanto um vai, só o último espera); se o servidor recusar, o item volta para a última posição salva e uma linha diz por quê |
| Painel do ponto (`pages/maps/point-panel`) | Ao lado do mapa: Tipo (três botões em `radiogroup`, o escolhido em `accent-soft`, negrito e com check), Nome, "Leva para" (só no Submapa), "Descrição para os jogadores" (até 2.000 caracteres), o interruptor "Revelado aos jogadores" (trilho `ink` com check quando ligado, contorno quando desligado: o grená é só do botão principal) e "Salvar ponto", o único botão cheio da tela. "Apagar ponto" confirma na própria tela ("Apagar Taverna do Javali? Não dá para desfazer.", com o foco no botão novo). Escolher outro ponto com mudança não salva pergunta ali mesmo: "Salvar e continuar", "Descartar mudanças" ou "Continuar editando" |
| Mapa do mestre no celular (`pages/maps/map-manage`) | O mapa só anda e dá zoom (a legenda diz "Use dois dedos…"), e revelar ou esconder fica nas listas "Pontos do mapa" e "Tokens no mapa" (E5-24) |
| Listas "Pontos do mapa" e "Tokens no mapa" (`shared/map-lists`) | Uma linha por item: o glifo, o nome, o tipo ("Submapa: Torre de Mirathel", "Mago 3, de Vinicius", "NPC, inimigo"), o estado em palavras com ícone ("Revelado", "Visível", "Escondido") e um botão contornado "Revelar aos jogadores" ou "Esconder" (o nome acessível leva o do item). A linha se arruma pela largura que tem (container query): numa coluna estreita o botão fica embaixo |
| Ficha do ponto (`shared/point-sheet`) | O que o jogador lê ao abrir um ponto (E5-25, E5-26): o glifo, o nome (Alegreya 22), o tipo e a descrição. No celular é uma folha de baixo que não bloqueia (o mapa continua andando); do tablet para cima é o primeiro bloco da coluna lateral. O foco vai para o título; "Fechar", Esc ou tocar no mapa vazio fecham, e o foco volta ao marcador. Um Submapa cujo destino o jogador vê oferece "Abrir <mapa>": a ficha vem primeiro, nunca um salto direto |
| Mapa do jogador (`pages/maps/player-map`) | O mapa com só o que está revelado, a trilha "Mirathel e arredores › Torre de Mirathel" dentro de um submapa (`parent_maps`), "Mapas revelados" (com `aria-current` no aberto) e "Pontos deste mapa", a alternativa sem ponteiro. Aberto pela sessão, o link de volta diz "Voltar para a sessão" |
| Painel "Mapas" da campanha (`pages/campaign-detail/maps-panel`) | Uma linha por mapa, com "Mapa principal, 5 pontos" ou "Submapa de Mirathel e arredores", e as etiquetas "Mapa atual" (pino), "Revelado" (olho) e "Escondido" (tracejada, olho cortado); "Novo mapa" contornado. O jogador vê só os mapas que pode abrir, sem etiquetas de estado, e nada enquanto não há nenhum |
| "Novo mapa" (`pages/maps/map-new`) | Nome e o seletor de imagem; o aviso neutro "O mapa nasce escondido. Revele quando o grupo chegar lá."; "Criar mapa", o único botão cheio, abre o editor no mapa novo. Erros: "Dê um nome ao mapa." e "Escolha uma imagem para o mapa." |
| Mapa da sessão (`pages/live-session/session-map`) | Para o jogador, a prévia parada (no celular abre com zoom de 2× no grupo; no computador mostra o mapa todo) com os tokens na legenda e "Ver mapa"; a prévia inteira também abre o mapa. O mestre tem o seletor "Mapa atual" (escolher um mapa escondido o revela aos jogadores, e a tela avisa), tokens que se arrastam no computador (um movimento por vez, como no editor), "Abrir mapa" e "Pontos do mapa". Sem mapa: "O mestre ainda não escolheu um mapa." (e, se o jogador perde a vista do mapa, o servidor responde `not_found` e a tela volta a esse aviso) |
| Painel "Imagem para os jogadores" (`pages/live-session/shown-image-panel`) | O painel do mestre (E5-10, E5-11): vazio, "Nenhuma imagem à mostra." e "Mostrar imagem" contornado, com o ícone `cast`; mostrando, moldura de 1,5 px `accent`, a miniatura de 112 × 84, a etiqueta "Mostrando agora", o nome (Alegreya 22/700) e "Parar de mostrar" e "Trocar imagem". "Parar de mostrar" age na hora, sem confirmar (mostrar de novo desfaz) e leva o foco a "Mostrar imagem"; mostrar leva o foco a "Parar de mostrar". Cada mudança é dita num `role="status"`. Sem botão cheio na página |
| Seletor "Mostrar uma imagem aos jogadores" (`shared/gallery-picker/image-picker-dialog`) | Um `MatDialog` de 720 px (tela cheia no celular): o seletor de imagem em 3 colunas (2 no celular), só com o nome embaixo (é a legenda que os jogadores verão). A imagem já à mostra leva "À mostra agora"; a que é o fundo de um mapa escondido leva o ícone `visibility_off` e "Fundo de mapa escondido", e escolhida, a linha diz "Mostrar essa imagem não revela o mapa Covil dos goblins." "Mostrar aos jogadores" é o botão cheio do diálogo (desabilitado com a imagem já à mostra, e a linha diz por quê); sem escolha, ele diz "Escolha uma imagem para mostrar." O mesmo diálogo serve a "Trocar imagem" do mapa |
| Bloco "O mestre está mostrando" (`pages/live-session/shown-image-block`) | O que o jogador vê (E5-12, E5-13): moldura de 1,5 px `accent`, o título com o ícone `cast`, a imagem, a legenda (o nome da imagem), "Fica aqui enquanto o mestre mostrar." e "Ver em tela cheia" (que leva o nome da imagem). A moldura da imagem tem tamanho fixo (160 px de altura no celular, 320 px no computador, até 440 px de largura), com a imagem inteira sobre `ground`: trocar um retrato por uma paisagem não mexe no resto. Aparece crescendo e clareando (200 ms) e some clareando e fechando; com movimento reduzido, aparece e some na hora. Nunca toma o foco nem a rolagem; um `role="status"` diz "O mestre está mostrando Capitão Goblin." e "O mestre parou de mostrar a imagem." A tela cheia acompanha o estado: trocar atualiza, parar fecha |

Regras dos mapas:

- **O jogador nunca recebe o que está escondido.** Os pontos escondidos e os tokens escondidos não existem na tela do jogador, nem desenhados em cinza: o servidor não manda, e o componente do mapa filtra de novo.
- **Estado é linha, ícone e palavra, nunca só cor:** tracejado + selo + "Escondido".
- **Imagem do jogador só enquanto é mostrada ou é de um mapa que ele vê** (RN-10): a tela nunca guarda a imagem para depois.

## Texto na tela

- Português do Brasil, com os termos da tradução oficial de D&D ("Classe de Armadura", "Salvaguardas", "Pontos de vida").
- Caixa normal ("Minhas campanhas"); rótulos nunca em caixa alta.
- O botão diz o que acontece ("Iniciar sessão", "Gerar convite"), e a confirmação repete o verbo ("Convite gerado").
- Erro diz o que houve e como resolver, sem pedir desculpas. Tela vazia convida à próxima ação.
- Número com unidade e vírgula decimal: "7,5 m (25 pés)".
- Nada interno na tela: sem IDs, versão de conteúdo, nome de enum ou "Sem nome".
- Sem emoji.

## Movimento

Só em resposta a uma ação (abrir uma característica, trocar de passo, confirmar), e nada anima com `prefers-reduced-motion`.

## Ícones

Material Symbols Outlined, servidos pelo app (`@material-symbols/font-400`), de 16 a 24px, na cor do texto ao lado. Botão só com ícone tem `aria-label`.
