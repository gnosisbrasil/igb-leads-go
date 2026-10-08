# Virada Node → Go (2026-10-07, executada)

Backend de leads em produção hoje é o Go (`leads-go-backend`).
O container Node (`itc4t27vg7wy1t7uht0w4pco-190805456046`) está parado,
mantido para rollback — não remover sem combinar.

## Mapa de produção (levantado na virada)

- Frontend: Cloudflare Pages (`sistema-leads-frontend.pages.dev`),
  servido em `https://leads.gnosisbrasil.com`. Estático, intocado na virada.
- API pública: `https://itc4t27vg7wy1t7uht0w4pco.145.223.95.103.sslip.io`
  (Traefik → container, porta interna 3001). O bundle React tem essa URL
  fixa — por isso o Go reaproveita a MESMA regra de Host.
- Postgres: container `m7pxh3w063qpaivf2faabzq6`, banco `postgres`
  (mesmo banco, zero migração).
- `EFI_SANDBOX=true`, `EFI_PIX_KEY` vazia (Pix nunca operou de verdade;
  o Node retornava QR **simulado**; o Go falha alto com 502).
- Sem SMTP/WhatsApp configurados (igual ao Node): envios pulados com aviso,
  links `wa.me` manuais funcionando.

## O que foi feito

1. `docker build -t leads-go-backend:prod` em `/root/igb-leads-go-build`.
2. Env em `/root/leads-go-backend.env` (600), copiado do container Node
   menos `COOLIFY_*`, mais `UPLOAD_DIR=/app/uploads`.
3. Volume nomeado `leads-go-uploads` (uploads do Node eram efêmeros).
4. Subido `leads-go-test` (porta 127.0.0.1:3101, sem Traefik) e validado:
   `docs/cutover/virada-test.sh` → **PASS=26 FAIL=0**, incluindo diffs
   Node-vs-Go em dados reais (campanhas, admin report, notificações).
5. Criado `leads-go-backend` com as mesmas regras Traefik do Node,
   verificado pelo IP direto, e parado o Node.
6. Pós-virada pela URL pública: `docs/cutover/pos-virada.sh` →
   **PASS=15 FAIL=0** (frontend 200 + API + ciclo campanha/lead/upload).

## Fix pós-virada (2026-10-07, mesmo dia)

- **Webhook verifica-na-Efí**: antes, qualquer POST com um `txid`
  válido marcava a campanha como paga (herdado do Node). Agora cada
  `txid` é reconsultado (`GET /v2/cob/:txid`) e a baixa só acontece com
  status `CONCLUIDA`; `txid` inexistente na Efí é ignorado (200) e falha
  na consulta responde 502 para retry. Imagem reconstruída e container
  recriado via runbook; verificado em produção (`/webhook` e
  `/webhook/pix` respondendo, Node segue parado).
- Rota nova `POST /api/payment/webhook/pix`: a Efí anexa `/pix` à URL
  nas notificações reais; o Node não tinha essa rota (404 certo quando
  o Pix operasse).

## Release pós-virada 2 (2026-10-07)

- **Meow ligado**: `MEOW_*` da conta do helpdesk; conectividade provada
  (probe auth OK). 2 campanhas com `auto_relationship` ("Meditação",
  "Medita Brasil"): envios automáticos passam a ocorrer nos crons
  (07/08/10h UTC) e em novas inscrições dessas campanhas; nada dispara
  no boot. Teste de envio real pendente (número do operador ou botão
  "gerar link" na UI, que agora envia de verdade).
- **Hostname `api-leads.gnosisbrasil.com`**: DNS CNAME → túnel + ingress
  + label Traefik (HTTP, via túnel). Verificado servindo a API. Frontend
  segue no sslip até seu próprio redeploy (Pages `igb-sistema-leads`,
  repo `gnosisbrasil/igb-sistema-leads`, branch main).
- **Quirks corrigidos** (e2e 71/71, cron 12/12): órfão 400 com mensagem,
  logs com usuário/IP reais, máscara `***1234` no delete, ordenação
  determinística, scheduler 7d/3d/1d funcionando com anti-duplicata.
  Zero campanhas com evento em 7d no deploy → nenhum e-mail disparado.
- OAuth Google: redirect 302 validado em produção (client_id +
  callback corretos); consentimento interativo pendente.

## Rollback (emergência)

```bash
docker stop leads-go-backend
docker start itc4t27vg7wy1t7uht0w4pco-190805456046
```

O Node volta a atender o mesmo domínio (JWT e banco idênticos).

## Recriar o container Go

```bash
/root/leads-go-backend-run.sh   # cópia em docs/cutover/
```

## Migração para gerenciamento Coolify (passo manual na UI)

O container atual é manual (estável: `unless-stopped`, healthcheck,
volume persistente). Para gerenciá-lo pelo dashboard:

1. Suba este repo (`igb-leads-go`) para um remoto git.
2. No Coolify, abra o projeto `sistema-de-leads`, aplicação 25 (a do
   backend, hoje `Exited`): troque o Source para o repo Go, build pack
   Dockerfile, porta 3001, mantenha as envs e adicione storage
   persistente em `/app/uploads` (+ `UPLOAD_DIR=/app/uploads`).
3. Pare o manual (`docker stop leads-go-backend`), clique **Deploy**,
   verifique, depois `docker rm leads-go-backend`.
4. O domínio permanece o mesmo (UUID do recurso não muda) — frontend
   continua sem alteração.

## Pendências pós-virada (não bloqueiam)

- Pix real: definir `EFI_PIX_KEY` (+ `EFI_SANDBOX=false` quando sair do
  sandbox) e o webhook Efí (mTLS — ver `docs/EFI_PIX.md`).
- WhatsApp meow: definir `MEOW_URL`/`MEOW_API_KEY` (chave com o helpdesk)
  quando o operador quiser envio automático.
- SMTP: **ligado em produção em 2026-10-07** via relay Hostinger com a
  conta do app de cursos (`suporte@cursos.gnosisbrasil.com`, remetente
  "Sistema de Leads - Gnosis Brasil"). Reset de senha funcionando
  (verificado com envio real). Troca opcional futura: caixa dedicada
  `noreply@gnosisbrasil.com` (só trocar `SMTP_USER/PASSWORD/FROM`).
- Login interativo (com Turnstile de verdade) validado pelo operador no
  navegador — o gate foi verificado dos dois lados via API.
