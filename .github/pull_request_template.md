## O que muda e por quê

<!-- Duas ou três frases. O diff já mostra o quê; escreva o porquê. -->

## Como testei

<!-- Comando, endpoint, cenário. "Testei local" não diz nada a quem revisa. -->

## Impacto no deploy

<!-- Marque o que se aplica e apague o resto. Os caminhos são do repo de infra. -->

- [ ] **Variável de ambiente** nova ou alterada → está em `env:` ou
      `secretEnv:` de `apps/agents/values.yaml`, ou em `base/configmap.yaml`
      se for compartilhada. Sem isso o pod sobe sem ela.
- [ ] **Índice ou migração no Mongo** → versionado em `atlas/` e aplicado
      **antes** do merge, senão a consulta volta vazia sem erro nenhum.
- [ ] **Contrato gRPC ou rota HTTP** mudou → quem consome já aguenta as duas
      versões, ou entra no mesmo deploy.
- [ ] Nada acima.

---

O merge na `main` **vai para produção sem aprovação manual**: publica
`ghcr.io/audita-bids/agents:<sha>` e commita a tag em
`apps/agents/values-production.yaml` no repo de infra; o ArgoCD sincroniza a
seguir. Rollback é reverter o commit `ci(agents): deploy <sha>` lá.
