# Perguntas em aberto

Nenhuma delas trava a Etapa 1. Todas são para o Samuel, e cada resposta vira uma regra marcada como "Decidido". Pergunta nova, a partir de agora, entra pelo documento de acompanhamento, não aqui; quando o Samuel responde (ou o Vinicius responde por ele), a resposta fica registrada nesta página.

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
- **Regras como dados e o Expr (ADR-0008).** Aceito: as regras viram dados, o SRD 5.1 vem com o app, o que não está nele a mesa cadastra, e as fórmulas rodam na biblioteca Expr (versão 1.17.7 ou mais nova), com as funções embutidas desligadas e limite de tamanho. Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).
- **Prioridade da MR-023 e da MR-024.** A MR-024 (aprovar o personagem do convite) entra no MVP, na Etapa 4; a MR-023 (passar ou dividir a campanha) fica para depois do MVP, na Etapa 8. Ver [roadmap](../roadmap.md).
- **Senha do jogador sem Google (RN-17).** O jogador entra sem senha. Quando a primeira sessão de 30 dias vence, o app exige uma senha ou o vínculo com o Google para continuar (ADR-0009, opção 3). Ver [RN-17](regras.md).
- **Personagem de quem exclui a conta (RN-16).** Aceito o tratamento proposto: o personagem passa a ser do mestre, sem vínculo com a conta apagada; a tela de exclusão avisa e deixa apagar o personagem junto; o texto livre que fica não é filtrado. A espera de 30 dias do mestre usa a marca "apagar em" na conta, conferida no login. Ver [Privacidade](../privacidade.md#a-tensão-entre-manter-o-personagem-e-apagar-a-identidade).
- **Menores de 18 anos.** O MVP é só para maiores de 18 anos, por autodeclaração. Um menor que já esteja na mesa joga sem conta própria até existir um fluxo com os responsáveis revisado por advogado. Ver [Privacidade](../privacidade.md#menores-de-idade).
- **Controlador, encarregado e canal.** O Samuel é o controlador e o Vinicius é o encarregado. Um e-mail só para isso recebe os pedidos até existir o domínio. Ver [Privacidade](../privacidade.md).
- **Abrir para outras mesas ou cobrar.** Não no MVP: o app é só da nossa mesa. A decisão volta antes de abrir. Ver [Privacidade](../privacidade.md).
- **Plano do CockroachDB e backups.** Fica o plano atual, com os backups em São Paulo, guardados por no máximo 30 dias. Ver [Operação](../operacao.md).
- **Menores na mesa hoje.** Não há (respondida pelo Vinicius em 29/09/2026).
- **Personagem criado depois da primeira sessão (RN-01).** Fica editável até a próxima sessão começar (respondida pelo Vinicius em 29/09/2026). Ver [RN-01](regras.md).
- **Texto descritivo depois da trava (RN-01).** O jogador edita a história do personagem (personalidade, aparência, história, aliados) enquanto a ficha é rascunho. Depois da trava, só quando o mestre libera, personagem por personagem, até a próxima sessão começar ou até o mestre travar de novo. Os números calculados nunca são editáveis (respondida pelo Vinicius em 29/09/2026). Ver [RN-01](regras.md).
- **Critérios de aceite da MR-005.** Aceitos como propostos (respondida pelo Vinicius em 29/09/2026). Ver [MR-005](historias.md#mr-005-criar-npcs).
- **RN-11 (notas do mestre).** Decidida: só o mestre lê e edita as notas do mestre (respondida pelo Vinicius em 29/09/2026). Ver [RN-11](regras.md).
- **Texto do SRD.** Os nomes aparecem em português, numa tradução nossa; as descrições ficam em inglês por enquanto. Depois pesquisamos uma tradução em português com licença adequada (respondida pelo Vinicius em 29/09/2026).
- **Plano do CockroachDB.** É o plano legado Unlimited, contratado antes da mudança de licenças de 2024. Trocar de plano perde o Unlimited; está em avaliação se vale migrar para o Cloud SQL ou outro produto (respondida pelo Vinicius em 29/09/2026). Ver [Operação](../operacao.md).
- **Conteúdo cadastrado pela mesa.** Vale por campanha, porque campanhas diferentes usam materiais diferentes (respondida pelo Vinicius em 29/09/2026). Ver [ADR-0008](../adr/0008-regras-dnd-conteudo-como-dados-motor-puro.md).
- **Banco do app antigo.** É desligado, com os backups, logo depois da importação única dos personagens, na Etapa 8 (respondida pelo Vinicius em 29/09/2026). Ver [roadmap](../roadmap.md).
- **Dados físicos ou do app (RN-18).** Os dois: o mestre escolhe se permite; se permitir, cada jogador escolhe entre o dado do app e o físico (decidido pelo Samuel em 29/09/2026). Ver [RN-18](regras.md).
- **Conteúdo que não vem no SRD.** Ideia do Samuel (29/09/2026): o mestre manda o PDF das regras e o app cadastra sozinho as classes, raças e regras; quando não der, o mestre cadastra, e o jogador pode propor uma raça ou classe nova (com o PDF ou o link) para o mestre aprovar. Virou as histórias [MR-025](historias.md#mr-025-cadastrar-conteúdo-da-mesa), [MR-026](historias.md#mr-026-propor-uma-raça-ou-classe-nova) e [MR-027](historias.md#mr-027-ler-as-regras-de-um-pdf). A prioridade foi decidida em 02/10/2026: Etapa 8, depois do MVP.

## Respondidas em 02/10/2026

Trazidas pelo Vinicius. Os números são os das linhas do documento de acompanhamento.

- **Pergunta 20: prioridade do conteúdo da mesa (MR-025 e MR-026).** As duas ficam para depois do MVP, na Etapa 8: primeiro o cadastro pelo mestre ([MR-025](historias.md#mr-025-cadastrar-conteúdo-da-mesa)), depois a proposta do jogador ([MR-026](historias.md#mr-026-propor-uma-raça-ou-classe-nova)), que só vale depois que o mestre aprova. Ver [roadmap](../roadmap.md).
- **Pergunta 21: ler as regras de um PDF (MR-027).** Sim, mas depois do cadastro pelo mestre (MR-025), e o mestre revisa tudo antes de valer. Fica na Etapa 8. Ver [MR-027](historias.md#mr-027-ler-as-regras-de-um-pdf).
- **Pergunta 22: o app guarda o PDF?** O PDF fica guardado só enquanto é processado e é apagado logo depois; um prazo curto (TTL) no arquivo garante o apagamento mesmo se o processamento falhar. Ver [MR-027](historias.md#mr-027-ler-as-regras-de-um-pdf) e [Privacidade](../privacidade.md).
- **Pergunta 23: o jogador sai quando o mestre recusa o personagem do convite?** Sim: o personagem recusado e a participação pendente são apagados, e o mestre manda um convite novo se quiser. Já implementado. Ver [RN-15](regras.md) e [MR-024](historias.md#mr-024-aprovar-o-personagem-do-convite).
- **Pergunta 24: o jogador pendente que nunca cria o personagem.** O mestre passa a ver quem está pendente sem personagem, com um botão para remover, e a participação pendente é apagada sozinha depois de 30 dias sem personagem. Decidido, ainda a fazer, num PR próprio. Ver [MR-024](historias.md#mr-024-aprovar-o-personagem-do-convite).
- **Pergunta 25: convite comum para quem está pendente.** O jogador pendente vira membro na hora, porque o convite comum não pede aprovação e conta como a aprovação do mestre. Decidido, ainda a fazer (hoje ele continua pendente), num PR próprio. Ver [RN-15](regras.md).
- **Pergunta 27: o jogador vê o documento da campanha (MR-018)?** Não: no MVP, só o mestre. Já implementado. Ver [MR-018](historias.md#mr-018-documento-de-campanha).
- **Pergunta 28: na sessão, o jogador vê o PV dos outros personagens?** Não: cada jogador vê o próprio, e o mestre vê o de todos. Já implementado. Ver [RN-02](regras.md).
- **Pergunta 29: um ponto de batalha ou de cena de RP pode existir antes do combate e das cenas?** Sim, e já é assim: o ponto tem nome e descrição, e os de batalha e de cena abrem o combate e a cena quando essas etapas chegarem. Ver [MR-008](historias.md#mr-008-pontos-de-interesse).
- **Pergunta 30: limites da galeria (MR-019).** JPEG, PNG ou WebP, até 10 MB por imagem, até 300 imagens e 500 MB por campanha; os metadados da foto (EXIF, localização) saem no envio. Já implementado. Ver [MR-019](historias.md#mr-019-galeria-de-imagens).
- **Pergunta 31: NPC no mapa sem os jogadores verem?** Sim: o token do NPC nasce escondido, e o mestre revela quando quiser. Já implementado. Ver [MR-009](historias.md#mr-009-mapa-sem-spoiler).
- **Pergunta 32: o jogador mantém acesso à imagem depois que o mestre para de mostrar (MR-028)?** O padrão fica: a imagem some da tela dos jogadores e eles perdem o acesso quando o mestre para de mostrar. Além disso, o mestre ganha um controle para manter a imagem à mostra quando precisar. O controle ainda não existe: o desenho vem com as telas da Etapa 6. Ver [MR-028](historias.md#mr-028-mostrar-uma-imagem-aos-jogadores).

## Em aberto

Nenhuma aqui. As perguntas novas ficam no documento de acompanhamento.

## Ver também

- [Regras de negócio](regras.md)
- [Histórias e critérios de aceite](historias.md)
