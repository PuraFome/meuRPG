# Perguntas em aberto

Nenhuma delas trava a Etapa 1. Todas são para o Samuel, e cada resposta vira uma regra marcada como "Decidido". Pergunta nova, a partir de agora, entra pelo documento de acompanhamento, não aqui.

## Respondidas em 29/09/2026

- **Nome do produto e domínio.** "our pure rpg" é nome provisório; o domínio sai só quando o nome for definitivo. Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).
- **RN-02 (correção de PV na mão).** Sim, o mestre pode corrigir PV e espaços de magia a qualquer momento da sessão; o mestre tem a palavra final. Ver [RN-02](regras.md).
- **RN-05 (mestre também jogador em outra campanha).** Sim, decidido; o backend já suportava. Ver [RN-05](regras.md).
- **MR-015 (quem escolhe as ações da cena).** Na cena de RP, o mestre escolhe as ações possíveis; no combate, quem decide e mostra as ações é o sistema, pelas regras de D&D. Ver [MR-015](historias.md#mr-015-ações-da-cena-de-rp) e [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).
- **RN-09 (XP por ouro).** 1 XP por 1 peça de ouro (PO), como nas edições antigas. Ver [RN-09](regras.md).
- **RN-03 (um personagem por campanha, ou outro quando o primeiro morre).** O jogador só cria um personagem novo na mesma campanha quando o atual morre; o personagem morto não é apagado — fica no sistema. Ver [RN-03](regras.md).
- **RN-07 (usos e validade do convite).** Padrão de 1 uso e 7 dias; o mestre escolhe de 1 a 20 usos e de 5 minutos a 30 dias, e pode revogar. O comportamento já implementado vira regra. Ver [RN-07](regras.md) e [MR-002](historias.md#mr-002-gerar-convite).
- **RN-06 (a notificação em tela basta, ou precisa de push?).** Basta a notificação em tela, para quem está com o app aberto; sem notificação push do navegador no MVP. Ver [RN-06](regras.md).
- **RN-08 (DOCX continua? qual o formato de exibição?).** Sem DOCX: só PDF editável, no formato do D&D Beyond ou da ficha em português. A importação em si fica para depois do MVP. Ver [RN-08](regras.md) e [MR-007](historias.md#mr-007-importar-ficha-em-pdf).
- **MR-018 e MR-019 (MVP ou Depois?).** MVP, na Etapa 5 do roadmap, ao lado dos mapas. Ver [roadmap](../roadmap.md) e [historias.md](historias.md#mr-018-documento-de-campanha).
- **Algum dado do app antigo precisa vir para o sistema novo?** O banco antigo pode ser apagado. Só os personagens são importados para o banco novo, numa importação única. Ver [Modelo de dados](../dados.md) e [roadmap](../roadmap.md).
- **Classes e raças que a mesa usa hoje (ADR-0008).** Todas as 12 classes e as 9 raças base do D&D 5e, com planos de acrescentar as expansões oficiais e algum conteúdo feito pela comunidade. O SRD 5.1 cobre as 12 classes e as 9 raças base, mas só uma subclasse por classe e um conjunto limitado de subraças e antecedentes; o resto (outras subclasses, o antecedente Sábio, expansões e conteúdo da comunidade) entra como conteúdo cadastrado pela mesa. Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).
- **Personagens de quem exclui a conta.** Os personagens de um jogador que exclui a conta ficam vinculados ao mestre, não são apagados. O mestre que exclui a conta tem 30 dias para voltar com a mesma conta antes de tudo ser apagado. Ver [RN-16](regras.md) e [Privacidade](../privacidade.md#excluir-a-conta).
- **Por quanto tempo guardamos uma conta sem uso?** Cada pessoa escolhe no próprio perfil; padrão de 1 ano. Ver [RN-16](regras.md).
- **Mais de um mestre.** Sim: uma campanha pode ter mais de um mestre, e um mestre pode passar a campanha para outro. Ver [RN-13](regras.md) e [MR-023](historias.md#mr-023-passar-ou-dividir-a-campanha).
- **Criar campanha exige conta com Google?** Confirmado para o MVP: só uma conta de mestre completa cria campanha, e hoje só a conta com Google é completa. Ver [RN-14](regras.md).
- **Convite com aprovação.** Sim, com um adicional: pelo convite, o jogador já cria o personagem, e o mestre aprova ou recusa esse personagem para a campanha. Ver [RN-15](regras.md) e [MR-024](historias.md#mr-024-aprovar-o-personagem-do-convite).
- **MR-002, critérios propostos.** Aceitos pelo Samuel, junto com os padrões do convite (1 uso, 7 dias) já implementados. Ver [MR-002](historias.md#mr-002-gerar-convite).

## Em aberto

### Privacidade (LGPD e GDPR)

- [ ] Quem é o controlador e quem é o encarregado (DPO)? Qual e-mail recebe os pedidos?
- [ ] Há planos de abrir o app para outras mesas ou de cobrar?
- [ ] Qual é o plano do CockroachDB (e onde estão os backups)?
- [ ] **Menores de 18 anos.** O Samuel perguntou o que precisamos fazer se jogadores ou mestres forem menores de 18 anos. Respondemos em 29/09/2026, no documento de acompanhamento: o MVP fica só para maiores de 18 anos, por autodeclaração, e um menor que já esteja na mesa joga sem conta própria até existir um fluxo com os responsáveis revisado por advogado (ver [Privacidade](../privacidade.md#menores-de-idade)). Falta o aceite do Samuel, e saber se há algum menor na mesa hoje.
- [ ] O tratamento proposto para o personagem de quem exclui a conta (fica com o mestre, sem vínculo com a conta apagada, ou é apagado junto, se a pessoa preferir) e a marca "apagar em" para os 30 dias do mestre estão aceitos? Ver [Privacidade](../privacidade.md#a-tensão-entre-manter-o-personagem-e-apagar-a-identidade).

### Login do jogador sem Google

RN-17 já decide o login anônimo por handle de mesa (apelido do mestre junto do apelido do jogador). Falta:

- [ ] Uma senha passa a ser exigida antes de acabarem os primeiros 30 dias? Sugestão (ADR-0009, opção 3): o jogador entra sem senha, e só precisa definir uma senha ou vincular o Google quando a primeira sessão vencer. Ver [ADR-0009](../adr/0009-login-do-jogador-sem-google.md).

### Prioridade das histórias novas

- [ ] [MR-023](historias.md#mr-023-passar-ou-dividir-a-campanha) (passar ou dividir a campanha, RN-13) e [MR-024](historias.md#mr-024-aprovar-o-personagem-do-convite) (aprovar o personagem do convite, RN-15): qual a prioridade de cada uma? Sugestão: a MR-024 no MVP, na Etapa 4, que já faz o personagem do convite; a MR-023 depois do MVP, porque o banco já guarda o papel de cada membro.

## Em discussão desde 28/09/2026

### Regras por classe e raça

Quais classes e raças a mesa usa já foi respondido acima (todas as base, mais expansões e conteúdo da comunidade depois). Ainda em aberto: o motor de fórmulas.

Proposta: as regras viram **dados**, e o módulo `rules` só calcula. O SRD 5.1 é aberto (CC-BY-4.0) e vem junto com o app; o que não está nele (outras subclasses, o antecedente Sábio, expansões e conteúdo da comunidade) é cadastrado pela própria mesa.

- **Decidido pelo Vinicius (29/09):** fórmulas de regra (CD de magia, bônus) usam a biblioteca **Expr**, com as funções embutidas desligadas e limite de tamanho, porque o conteúdo cadastrado pela mesa é entrada não confiável.
- **Pergunta:** falta o aceite do Samuel para a escolha do Expr. Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).
- **Afeta:** [MR-004](historias.md#mr-004-ficha-no-formato-do-pdf), [MR-013](historias.md#mr-013-ordem-dos-turnos), [MR-014](historias.md#mr-014-sua-vez), [MR-015](historias.md#mr-015-ações-da-cena-de-rp), [MR-016](historias.md#mr-016-dar-xp), [MR-017](historias.md#mr-017-subir-de-nível) e [RN-02](regras.md).

## Ver também

- [Regras de negócio](regras.md)
- [Histórias e critérios de aceite](historias.md)
