# Paridade e E2E

Probes reutilizáveis da reescrita. Todos sobem um Postgres scratch local
(porta 5544), carregam o schema real (`/tmp/leads-schema.sql`, extraído
somente-leitura da produção) e exercitam a API.

Requisitos: Postgres local (`initdb`, `psql`), Go, Node e o backend
original em `../igb-sistema-leads` (para os scripts de paridade).

| Script | O quê |
|---|---|
| `leads-e2e.sh` | Fase 1 no Go: auth, usuários, regiões (37 checagens) |
| `leads-e2e2.sh` | Fase 2 no Go: campanhas, formulários, leads, links públicos, OG/QR (65 checagens) |
| `leads-parity.sh` | Fase 1: mesmas requisições no Node e no Go, diff normalizado (11 pontos) |
| `leads-parity2.sh` | Fase 2: idem (31 pontos + OG + QR decodificado) |
| `leads-e2e3.sh` | Fase 3 no Go: Pix (mock Efí), webhook real, topup, autorelacionamento, SMTP (33 checagens) |
| `cron/run.sh` | Métodos dos crons + scheduler de e-mails (12 checagens) |
| `mocks/` | Servidores falsos de Efí, meow e SMTP usados pelos scripts |
| `leads-e2e4.sh` | Fase 4 no Go: relatórios, notificações, upload, templates de mensagem (71 checagens) |
| `leads-parity4.sh` | Fase 4: idem Node vs Go (23 pontos; `r-sup` difere só nos segredos — ver abaixo) |

Divergências intencionais (correções de segurança, não bugs):
- `GET /api/reports/supervisor`: o Node vaza `password_hash`,
  `reset_password_token` e `reset_password_expires` dentro de cada
  `campaigns[].user`. O Go omite as três chaves (o modelo `User` já as
  esconde em todas as outras rotas). Todo o resto do relatório é idêntico.
Melhorias pós-virada (divergem do Node de propósito):
- Lead órfão em links WhatsApp: 400 `Lead não está vinculado a uma
  campanha` (era 500 herdado do crash no Node).
- `system_logs` grava `user_id` e `ip_address` reais; `LEAD_DELETED`
  mascara o WhatsApp (`***1234`) em vez do `-` literal do typo Node.
- Templates ordenados por `sort_order, key` (empates determinísticos).
- Scheduler 7d/3d/1d corrigido: o Node nunca agendava (guarda
  impossível); agora agenda devido-já, sem duplicar (anti-spam).

A normalização troca UUIDs, JWTs, timestamps, tokens e check-in codes
por placeholders antes do diff, então só diferenças reais aparecem.

Uso:

```bash
./parity/leads-e2e2.sh
./parity/leads-parity2.sh
```
