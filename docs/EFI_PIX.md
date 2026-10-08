# Efí Pix — integração (somente Pix)

Fonte: [dev.efipay.com.br/docs/api-pix](https://dev.efipay.com.br/docs/api-pix/credenciais)
(credenciais, cobranças imediatas, webhooks).

## Credenciais

- Conta Digital Efí com aplicação criada; cada aplicação tem um par
  `Client_Id`/`Client_Secret` para produção e outro para homologação.
- Escopos necessários: `cob.write`, `cob.read`, `webhook.write`, `webhook.read`.
- Autenticação: OAuth2 client_credentials (`POST /oauth/token`, Basic).
- **Produção exige certificado mTLS** (.p12) em todas as chamadas à API Pix
  (norma BCB). Homologação (`pix-h.api.efipay.com.br`) dispensa mTLS.
- Env: `EFI_CLIENT_ID`, `EFI_CLIENT_SECRET`, `EFI_CERTIFICATE_BASE64`,
  `EFI_PIX_KEY`, `EFI_SANDBOX`.

## Cobrança imediata (copia-e-cola + QR)

- `PUT /v2/cob/:txid` com `{calendario.expiracao, devedor, valor.original,
  chave, solicitacaoPagador}`. O `txid` é nosso (26–35 chars, único).
- Resposta traz `pixCopiaECola` (payload BR Code pronto) e `location`.
- O QR Code imagem é renderizado **localmente** a partir do
  `pixCopiaECola` (qualquer lib QR) — nenhuma chamada extra.
- Status: `GET /v2/cob/:txid` → `ATIVA`, `CONCLUIDA`,
  `REMOVIDA_PELO_USUARIO_RECEBEDOR`, `REMOVIDA_PELO_PSP`.
- Homologação: cobrança R$ 0,01–10,00 confirma sozinha (com webhook);
  acima disso fica ativa sem confirmar.

## Webhook

- Registro: `PUT /v2/webhook/:chave` com `{"webhookUrl": "..."}`.
  Uma URL pode servir várias chaves. Consulta: `GET /v2/webhook/:chave`.
- A Efí **acrescenta `/pix`** ao final da URL nas notificações reais
  (no cadastro de teste, não). Truque oficial: cadastrar a URL com
  `?ignorar=` no final para ela não acrescentar o sufixo.
- Payload real: `{pix: [{endToEndId, txid, valor, horario, chave, ...}]}`.
  A confirmação chega com o `txid` da cobrança — é por ele que damos
  baixa (`payment_txid` → `paid`).
- **Verificação antes da baixa**: o handler reconsulta
  `GET /v2/cob/:txid` e só marca `paid` com status `CONCLUIDA`.
  `txid` inexistente na Efí é ignorado (200); falha na consulta
  responde 502 para a Efí retentar mais tarde. Rotas: o Node só tinha
  `POST /api/payment/webhook`; o Go atende também
  `POST /api/payment/webhook/pix` (a Efí anexa `/pix` de verdade).
- **mTLS no nosso servidor é obrigatório**: a Efí apresenta o certificado
  cliente dela e nosso TLS precisa exigi-lo (2 requests de validação:
  sem cert recusa, com cert faz handshake; TLS 1.2+).
- Implicação de deploy: o webhook precisa chegar com TLS nosso até o
  backend. Atrás do proxy Cloudflare o mTLS quebra (a Cloudflare termina
  o TLS). Opções: subdomínio do webhook em modo DNS-only apontando
  direto à VPS + Traefik exigindo client-cert só nessa rota; ou
  Cloudflare com mTLS configurado repassando.
- Defesa em profundidade: além do webhook, manter reconciliação por
  polling (`GET /v2/cob/:txid` nas campanhas `pending`) — cobre webhook
  perdido sem depender só dele.

## Registro do webhook (setup único por chave)

```bash
# 1. token
TOKEN=$(curl -su "$EFI_CLIENT_ID:$EFI_CLIENT_SECRET" \
  --cert-type p12 --cert <(echo "$EFI_CERTIFICATE_BASE64" | base64 -d) \
  https://pix.api.efipay.com.br/oauth/token \
  -d '{"grant_type":"client_credentials"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['access_token'])")

# 2. registra (a Efí acrescenta /pix; o truque ?ignorar= evita isso)
curl -s -X PUT "https://pix.api.efipay.com.br/v2/webhook/$EFI_PIX_KEY" \
  --cert-type p12 --cert <(echo "$EFI_CERTIFICATE_BASE64" | base64 -d) \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"webhookUrl":"https://leads.gnosisbrasil.com/api/payment/webhook?ignorar="}'

# 3. confere
curl -s "https://pix.api.efipay.com.br/v2/webhook/$EFI_PIX_KEY" \
  --cert-type p12 --cert <(echo "$EFI_CERTIFICATE_BASE64" | base64 -d) \
  -H "Authorization: Bearer $TOKEN"
```

O handler (`POST /api/payment/webhook`) confirma pelo `txid`,
avança campanhas `payment_pending` e notifica o dono. Txids
desconhecidos são registrados e confirmados (evita retry infinito).

## Bugs do Node que esta versão corrige

1. O webhook Node espera HMAC (`x-efi-signature`) e campos `type/status`
   — formato que **não existe** no webhook Pix real (`{pix: [...]}`).
   Na prática a confirmação automática nunca funcionava; só o polling
   do `getPaymentStatus` dava baixa.
2. Se a Efí falhava (ou nem estava configurada), o Node gerava um QR de
   **simulação** e seguia como pendente — cobrança falsa. Aqui: sem
   credencial ou com erro, a API retorna erro explícito.
3. O caminho `credit_card` chamava `createCardCharge`, que não existe no
   service — sempre caía em simulação. Removido: Pix only.
