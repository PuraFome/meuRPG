# Documentação do MeuRPG

Este diretório é a fonte da verdade sobre o produto, as regras e a arquitetura do MeuRPG. Um assunto vive em um arquivo só; os outros apontam para ele por link, sem copiar o conteúdo.

## Índice

| Documento | Para quem | Responde |
| --- | --- | --- |
| [produto/visao.md](produto/visao.md) | Todo o time, principalmente o Samuel | Por que o app existe e o que cada papel faz |
| [produto/glossario.md](produto/glossario.md) | Todo o time | O que cada termo significa |
| [produto/regras.md](produto/regras.md) | Samuel, dev da história | Como o jogo se comporta dentro do app |
| [produto/historias.md](produto/historias.md) | Samuel, dev da história | O que construir e como saber que está pronto |
| [produto/perguntas-em-aberto.md](produto/perguntas-em-aberto.md) | Samuel | O que ainda falta decidir |
| [arquitetura.md](arquitetura.md) | Quem abre o PR, com revisão do Samuel | Como o sistema é montado e onde roda |
| [dados.md](dados.md) | Quem abre o PR, com revisão do Samuel | Tabelas, relações e migrations |
| [privacidade.md](privacidade.md) | Quem abre o PR, com revisão do Samuel e do Vinicius | Que dados pessoais guardamos, por quê e por quanto tempo, e a checklist de privacidade de todo PR |
| [roadmap.md](roadmap.md) | Todo o time | A ordem das etapas até o MVP |
| [operacao.md](operacao.md) | Vinicius e Samuel | Deploy, segredos, custos e alertas |
| [code-quality.md](code-quality.md) | Quem mexe no Angular atual | Relatório de qualidade do frontend atual |
| `adr/` | Samuel e Vinicius | Decisões difíceis de desfazer. Repositório privado separado; a pasta local está no `.gitignore` e não é versionada aqui. |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Todo o time | Como rodar, testar e abrir um PR |
| [../README.md](../README.md) | Quem chega no repositório agora | Visão geral do projeto, com link para o resto |

Contratos de API (`.proto`) e migrations de banco não ficam documentados aqui: o `.proto` em `proto/meurpg/**/v1/*.proto` é a própria referência, gerada pelo CI; as migrations ficam em `backend/migrations/`.

## Como manter a documentação

A documentação muda no mesmo PR que o código. Um PR que muda comportamento sem mudar o documento correspondente volta na revisão. A tabela completa de "o que atualizar quando" e as regras de escrita ficam em [CONTRIBUTING.md → Como manter a documentação](../CONTRIBUTING.md#como-manter-a-documentação).

Convenções rápidas:

- Português, com termos de software em inglês (backend, deploy, token, migration, commit, PR, CI, stream). Código, comentários e nomes no código ficam em inglês.
- Diagramas em Mermaid, porque o GitHub renderiza direto no Markdown.
- A primeira frase de cada seção é a resposta; o contexto vem depois.
