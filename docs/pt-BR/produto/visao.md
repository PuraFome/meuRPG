# Visão do produto

> A versão em inglês ([vision.md](../../product/vision.md)) é a canônica.

O MeuRPG é onde uma mesa de D&D 5e prepara e joga suas campanhas. O mestre prepara o mundo e conduz a sessão ao vivo. O jogador entra por convite, acompanha a ficha e age no RP e no combate com o que as regras permitem.

O MVP tem uma meta só: **a mesa joga a primeira sessão inteira pelo app**, do convite ao XP no fim da noite.

| Papel | O que faz no app |
| --- | --- |
| Mestre | Cria campanhas, NPCs e mapas. Convida os jogadores, inicia a sessão, controla o mapa e os inimigos, e dá XP. |
| Jogador | Entra pelo convite, cria o personagem e acompanha a ficha e o mapa. Na sua vez, vê as ações possíveis; no RP, vê o que pode rolar. |
| Sistema | Aplica as regras durante a sessão: PV, espaços de magia, turnos e XP. O mestre tem a palavra final. |

## Princípios

Estes princípios guiam as decisões de produto e de arquitetura.

1. **O servidor é a autoridade.** Toda regra (trava da ficha, PV, XP, quem vê o quê) é conferida no servidor, nunca só na tela.
2. **Sem spoiler.** O jogador só vê o que o grupo já descobriu. As notas do mestre nunca saem do servidor para um jogador.
3. **O jogador joga pelo celular.** As telas do jogador são pensadas primeiro para a tela pequena.
4. **Custo perto de zero.** Sem sessão ativa, o servidor fica parado e nada fica conectado.
5. **Requisitos primeiro.** Cada história tem critérios de aceite que viram testes automáticos; uma funcionalidade só está pronta quando os testes dela passam. O backend novo é construído do zero, história por história, sem migração gradual a partir do app antigo.

## Ver também

- [Glossário](glossario.md): o significado de cada termo usado aqui.
- [Regras de negócio](regras.md): como esses princípios viram regra no sistema.
- [Histórias e critérios de aceite](historias.md): o que construir para chegar no MVP.
- [Roadmap](../../roadmap.md): a ordem das etapas até o MVP.
