# Security policy

MeuRPG runs one service in production, at https://meurpg.app, built from the
`main` branch. Only that version is supported: a fix lands on `main` and ships
in the next release.

## Reporting a vulnerability

Please report it privately, through GitHub's private vulnerability reporting:
the **Security** tab of this repository, then **Report a vulnerability**. Do
not open a public issue, pull request or discussion about it.

Say what you found, how to reproduce it and what an attacker could do with
it. You will get an answer within 7 days. Please give us a reasonable time to
fix it before you disclose it.

Do not test against https://meurpg.app with real accounts other than your
own, and do not read, change or delete data that is not yours. The local
environment (`make up`, see [CONTRIBUTING.md](CONTRIBUTING.md)) runs the whole
service on your machine for testing.

## What is in scope

The Go backend, the Angular app, the API contracts in `proto/`, the build and
deploy files, and the CI workflows. The privacy rules the service follows are
in [docs/privacy.md](docs/privacy.md).
