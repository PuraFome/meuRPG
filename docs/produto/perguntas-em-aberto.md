# Perguntas em aberto

Nenhuma delas trava a Etapa 1. Todas são para o Samuel, e cada resposta vira uma regra marcada como "Decidido".

- [ ] **Nome do produto e domínio.** O domínio sai do Student Pack e só é necessário no primeiro deploy.
- [ ] **RN-02:** o mestre pode corrigir PV e espaços de magia na mão durante a sessão? O guia assume que sim ("o mestre tem a palavra final").
- [ ] **RN-05:** um mestre pode ser jogador em outra campanha? A resposta de 28/09 tratou dos personagens; o guia assume que sim.
- [ ] **MR-015:** a lista de ações da cena sai sozinha da ficha, ou o mestre escolhe as ações de cada cena?
- [ ] **RN-09:** no modo por ouro, quanto XP vale cada peça de ouro? As edições antigas do D&D usavam 1 XP por 1 PO.
- [ ] **RN-03:** o jogador tem um personagem por campanha, ou pode ter outro, por exemplo quando o primeiro morre?
- [ ] **RN-07:** o convite serve para vários jogadores ou para um só? E vale por quanto tempo? O link de hoje pode ser reutilizado.
- [ ] **RN-06:** a notificação para quem está com o app aberto basta no MVP, ou precisa de notificação do navegador (push) com o app fechado?
- [ ] **RN-08:** a importação de DOCX continua? Se sim, de qual modelo? E a escolha entre D&D Beyond e ficha em português vale também para o jeito de mostrar a ficha?

## Em discussão desde 28/09/2026

Estes três temas ainda **não viraram regra**: RN-05, RN-06, MR-003 e as histórias de combate continuam valendo como estão até o Samuel aceitar. O que o Vinicius já decidiu em 29/09/2026 está marcado assim, e falta o aceite do Samuel.

### Login do jogador sem Google

Proposta: só o mestre entra com Google. O jogador entra pelo convite com um **handle da mesa** (único entre as campanhas do mesmo mestre, não por campanha, para a cópia de personagem da RN-03 continuar funcionando), sem e-mail nem nome real. Menos dados pessoais, o que ajuda com a LGPD (ver [Privacidade](../privacidade.md)).

- **Decidido pelo Vinicius (29/09):** as sessões seguem o NIST SP 800-63B-4: login de novo a cada **30 dias**, no máximo, para todo mundo. Senha nova é checada no Pwned Passwords, e só os 5 primeiros caracteres do hash saem do servidor.
- **Pergunta:** jogador sem senha perde o acesso a cada 30 dias e depende de um link novo do mestre. A recomendação é exigir uma senha antes de acabarem os primeiros 30 dias (passkey no lugar da senha quando o domínio existir). Vale assim?
- **Afeta:** [RN-05](regras.md), [RN-06](regras.md), [RN-07](regras.md), [MR-003](historias.md#mr-003-entrar-pelo-convite) e o módulo `identity` (ver [Arquitetura](../arquitetura.md)).

### Regras por classe e raça

MR-014 (e também MR-004, MR-013, MR-015, MR-016, MR-017 e RN-02) precisa que o sistema conheça as regras da classe e da raça do personagem: ações, ação bônus, reações e recursos como espaços de magia e deslocamento.

Proposta: as regras viram **dados**, e o módulo `rules` só calcula. O SRD 5.1 é aberto (CC-BY-4.0) e vem junto com o app. O PHB e o Xanathar não podem ser copiados, então esse conteúdo é cadastrado pela própria mesa. Exemplo: o Pensantus é gnomo das rochas e mago de evocação (estão no SRD), mas o antecedente Sábio não está.

- **Decidido pelo Vinicius (29/09):** fórmulas de regra (CD de magia, bônus) usam a biblioteca **Expr**, com as funções embutidas desligadas e limite de tamanho, porque o conteúdo cadastrado pela mesa é entrada não confiável.
- **Pergunta:** quais classes e raças a mesa usa hoje? Isso define o que entra primeiro para o MVP.
- **Afeta:** [MR-004](historias.md#mr-004-ficha-no-formato-do-pdf), [MR-013](historias.md#mr-013-ordem-dos-turnos), [MR-014](historias.md#mr-014-sua-vez), [MR-015](historias.md#mr-015-ações-da-cena-de-rp), [MR-016](historias.md#mr-016-dar-xp), [MR-017](historias.md#mr-017-subir-de-nível) e [RN-02](regras.md).

### Privacidade (LGPD e GDPR)

**Decidido pelo Vinicius (29/09):** a régua é a LGPD e o GDPR, sempre a regra mais rigorosa das duas. O que isso muda está em [Privacidade](../privacidade.md). Perguntas para o Samuel:

- [ ] Quem é o controlador e quem é o encarregado (DPO)? Qual e-mail recebe os pedidos?
- [ ] Algum jogador da mesa tem menos de 18 anos? A proposta é o MVP ser 18+.
- [ ] Quando alguém exclui a conta, o que acontece com os personagens dele numa campanha de outro mestre?
- [ ] Por quanto tempo guardamos uma conta sem uso?
- [ ] Há planos de abrir o app para outras mesas ou de cobrar?
- [ ] Qual é o plano do CockroachDB (e onde estão os backups)?

## Ver também

- [Regras de negócio](regras.md)
- [Histórias e critérios de aceite](historias.md)
