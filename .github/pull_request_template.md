## Objetivo

<!-- O problema que isso resolve. Sintoma observável, não a solução. -->

## O que mudou

<!-- As decisões, não o diff. Por que esta abordagem e não a óbvia. -->

## Verificação

<!-- O que você rodou e o que voltou. Cole a saída se couber em três linhas.
     Teste novo: diga qual comportamento ele trava. -->

## Risco e rollback

<!-- O que quebra se isso estiver errado, e como você percebe. Se for "nada",
     escreva o motivo. -->

## Antes do merge

- [ ] Env nova ou alterada → declarada em `apps/agents/values.yaml` e no
      `values-staging.yaml`, ou em `base/configmap.yaml` se for compartilhada
- [ ] Índice ou migração no Mongo → aplicada **antes** do merge; sem ela a
      consulta volta vazia, sem erro
- [ ] Contrato gRPC ou rota HTTP mudou → quem consome aguenta as duas versões,
      ou entra no mesmo deploy
- [ ] Nada acima

---

Merge na `main` publica `ghcr.io/audita-bids/agents:<sha>` e fixa a tag em
`apps/agents/values-production.yaml`, sem aprovação manual. Merge na `staging`
faz o mesmo em `values-staging.yaml`. Rollback é reverter o commit
`ci(agents): deploy …` no repo de infra.
