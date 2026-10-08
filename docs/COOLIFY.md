# Backend Leads Go no Coolify

App gerenciado desde 2026-10-08 (migração do container manual `leads-go-backend`).

- **App:** `API Leads Go` — uuid `aw3xsjdibipjcxd5hs697bxf`
- **Projeto:** Sistema de Leads (`ack0okocks88ww4okos4wg4w`), env `production`, servidor `localhost`
- **Repo:** `gnosisbrasil/igb-leads-go`, branch `main`, build `dockerfile`, porta `3001`
- **Domínios (fqdn):** `https://itc4t27vg7wy1t7uht0w4pco.145.223.95.103.sslip.io,http://api-leads.gnosisbrasil.com`
- **Volume:** `aw3xsjdibipjcxd5hs697bxf-leads-go-uploads` → `/app/uploads` (o antigo `leads-go-uploads` estava vazio e foi removido)
- **Env:** 28 vars runtime importadas de `/root/leads-go-backend.env` (arquivo mantido no VPS como backup)
- **Healthcheck:** `GET /health` (Dockerfile + Coolify)

## Por que `http://` no api-leads

O `api-leads.gnosisbrasil.com` chega ao Traefik via túnel Cloudflare em HTTP puro
(`http://localhost:80`, TLS termina no edge). Com scheme `https://` o Coolify gera
middleware `redirect-to-https` no router HTTP e o túnel entra em loop 302.

Com scheme `http://`, o Coolify gera só o router HTTP sem redirect — igual ao
setup manual anterior. O edge já tem Always Use HTTPS, então o cliente sempre usa TLS.

## Deploy manual via API (sem webhook configurado)

Push no GitHub **não** dispara deploy (0 webhooks). Para subir nova versão:

```
POST /api/v1/deploy?uuid=aw3xsjdibipjcxd5hs697bxf   (Bearer token, ability deploy/root)
GET  /api/v1/deployments/{deployment_uuid}          (status)
```

Evite viradas sobre o minuto `:00` (cron horário roda in-process; dois containers
dividindo o banco disparam crons duplicados se o overlap cruzar o boundary).

## Aposentados em 2026-10-08

- Container/imagem manual `leads-go-backend` / `leads-go-backend:prod` (removidos)
- `/root/leads-go-backend-run.sh` → `.retired` (só referência; NÃO executar — conflita com o app)
- Volume `leads-go-uploads` (removido; backup vazio em `/tmp/leads-uploads-20261008.tgz`)
- App Coolify 25 `API Leads` (Node, removido via API após virada Go)
