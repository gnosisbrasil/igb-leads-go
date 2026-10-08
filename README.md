# igb-leads-go

Backend do Sistema de Leads da Gnosis Brasil em Go — porte do
`../igb-sistema-leads/apps/backend` (Express + Sequelize), mantendo as
mesmas rotas, lógica e JSON (snake_case) sobre o **mesmo schema
PostgreSQL**. O frontend React não precisa de alteração.

Diferenças intencionais em relação ao Node:

- **Pix only via Efí** — sem cartão, boleto ou modo simulação
  (ver [docs/EFI_PIX.md](docs/EFI_PIX.md))
- **WhatsApp via API meow** (não-oficial) em vez da Meta Cloud API
- Falha alta: sem credencial ou com provedor fora, erro explícito —
  nunca QR/cobrança falsos

## Estrutura

```
igb-leads-go/
├── main.go            # wiring, rotas, shutdown gracioso
├── config/            # env com fail-fast
├── model/             # structs das 13 tabelas (JSON = Sequelize)
├── repository/        # pgx (pool + agregados)
├── service/           # regras de negócio e integrações
├── handler/           # HTTP, um arquivo por domínio
├── middleware/        # CORS, JSON, log, auth
└── docs/              # EFI_PIX.md e demais integrações
```

## Desenvolvimento

```bash
cp .env.example .env   # ajuste DATABASE_URL e JWT_SECRET
go run .
```

## Deploy (Coolify, mesmo padrão do busca-go)

1. Novo recurso → build pack **Dockerfile**
2. Porta interna `3000`, healthcheck `/health`
3. Volume: `./uploads` (via `UPLOAD_DIR`)
4. Variáveis conforme `.env.example`; `DATABASE_URL` do Postgres existente

## Fases

- [x] Fundação: config, models, banco, health
- [x] Fase 1: auth, usuários, regiões
- [x] Fase 2: campanhas, formulários, leads (+ QR e OG antecipados)
- [x] Fase 3: pagamento Pix + WhatsApp meow
- [x] Fase 4: relatórios, notificações, upload, templates de mensagem
- [x] Paridade + cutover (Go assume, Node desliga — ver docs/CUTOVER.md)

## Notas de paridade

- `GET /api/reports/supervisor` omite `password_hash`,
  `reset_password_token` e `reset_password_expires` de
  `campaigns[].user` — o Node vaza essas chaves; todo o resto é idêntico
  (ver `parity/README.md`).
- Rotas portadas na Fase 4: `GET /api/reports/{admin,system-logs,supervisor,executive}`,
  `GET/PATCH/POST /api/notifications*`, `POST/DELETE /api/upload/*`,
  CRUD + seed + links `GET /api/message-templates*`, estáticos `/uploads/*`
  e a rota `DELETE /api/leads/{id}` (faltava no Go).
