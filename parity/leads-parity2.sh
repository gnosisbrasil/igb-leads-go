#!/bin/bash
# Paridade Fase 2: mesmos fluxos no Node e no Go, diff normalizado.
set -u
PG=/usr/local/bin
NORM="python3 -c \"import json,sys,re; d=json.load(sys.stdin); s=json.dumps(d, sort_keys=True); s=re.sub(r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}', 'UUID', s); s=re.sub(r'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+', 'JWT', s); s=re.sub(r'20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z?', 'TS', s); s=re.sub(r'[0-9]{2}/[0-9]{2}/[0-9]{4}, [0-9]{2}:[0-9]{2}:[0-9]{2}', 'DTBR', s); s=re.sub(r'\b[0-9a-f]{16}\b', 'TOKEN16', s); s=re.sub(r'\b[A-Z0-9]{12}\b', 'CHECKIN', s); s=re.sub(r'campaign_[0-9]+', 'campaign_N', s); s=re.sub(r'[a-z-]+-[0-9]{13}', 'SLUGN', s); print(s)\""
HNORM="python3 -c \"import sys,re; s=sys.stdin.read(); s=re.sub(r'/form/[0-9a-f-]{36}', '/form/UUID', s); print(s)\""

reset_db() {
  $PG/psql -h /tmp -p 5544 -U postgres -c "DROP DATABASE IF EXISTS leads_parity;" -c "CREATE DATABASE leads_parity;" >/dev/null
  $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -q -f /tmp/leads-schema.sql >/dev/null 2>&1
  H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'
  $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -q <<SQL >/dev/null
INSERT INTO regions (id, name, code, country, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111','São Paulo','SP','Brasil',true,now(),now());
INSERT INTO users (id, email, password_hash, first_name, last_name, whatsapp, role, status, region_id, created_at, updated_at) VALUES
('22222222-2222-2222-2222-222222222222','admin@teste.com','$H','Ada','Admin','11999999999','admin','active','11111111-1111-1111-1111-111111111111',now(),now()),
('33333333-3333-3333-3333-333333333333','exec@teste.com','$H','Ex','Ecutivo','11988888888','executive','active','11111111-1111-1111-1111-111111111111',now(),now()),
('44444444-4444-4444-4444-444444444444','user@teste.com','$H','Comum','User','11977777777','user','active','11111111-1111-1111-1111-111111111111',now(),now());
SQL
}
db() { $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -tAc "$1"; }

capture() { # <base> <outdir>
  local B=$1 O=$2; mkdir -p $O
  local AT=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  local UT=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"user@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  local ET=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"exec@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  local PAYLOAD='{"title":"Palestra Pública Centro","description":"desc aqui","budget":1500.50,"objectives":"camara_publica","address_city":"Campinas","address_state":"SP","event_date":"2026-11-20T19:00:00Z","event_time":"19:00","weekdays":["seg","qua"],"event_dates":[{"date":"2026-11-20","time":"19:00"}],"details":{"call_to_action":"Inscreva-se"}}'
  curl -s -X POST $B/api/campaigns -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d "$PAYLOAD" | eval "$NORM" > $O/c-create.json
  local CID=$(db "SELECT id FROM campaigns LIMIT 1")
  local FTOKEN=$(db "SELECT public_token FROM forms LIMIT 1")
  curl -s $B/api/campaigns/$CID -H "Authorization: Bearer $UT" | eval "$NORM" > $O/c-get.json
  curl -s "$B/api/campaigns?page=1&limit=20" -H "Authorization: Bearer $AT" | eval "$NORM" > $O/c-list.json
  curl -s -X PUT $B/api/campaigns/$CID -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d '{"location":"Centro","budget":"2000.00"}' | eval "$NORM" > $O/c-update.json
  curl -s -X POST $B/api/campaigns/$CID/submit -H "Authorization: Bearer $UT" | eval "$NORM" > $O/c-submit1.json
  curl -s -X POST $B/api/campaigns/$CID/to-draft -H "Authorization: Bearer $UT" | eval "$NORM" > $O/c-todraft.json
  curl -s -X POST $B/api/campaigns/$CID/submit -H "Authorization: Bearer $UT" | eval "$NORM" > $O/c-submit2.json
  curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/campaigns/$CID/submit -H "Authorization: Bearer $UT" > $O/c-submit3.code
  curl -s -X POST $B/api/campaigns/$CID/mark-paid -H "Authorization: Bearer $AT" | eval "$NORM" > $O/c-markpaid.json
  curl -s "$B/api/campaigns/available" -H "Authorization: Bearer $ET" | eval "$NORM" > $O/c-available.json
  curl -s -X POST $B/api/campaigns/$CID/pick-up -H "Authorization: Bearer $ET" | eval "$NORM" > $O/c-pickup.json
  curl -s -X POST $B/api/campaigns/$CID/start -H "Authorization: Bearer $ET" | eval "$NORM" > $O/c-start.json
  curl -s $B/api/campaigns/$CID/health -H "Authorization: Bearer $AT" | eval "$NORM" > $O/c-health.json
  curl -s $B/api/forms/token/$FTOKEN | eval "$NORM" > $O/f-token.json
  curl -s -X POST $B/api/leads/public/$FTOKEN -H 'Content-Type: application/json' -d '{"first_name":"Maria","last_name":"Silva","whatsapp":"(11) 98888-7777","email":"maria@teste.com"}' | eval "$NORM" > $O/l-create.json
  local LID=$(db "SELECT id FROM leads LIMIT 1"); local LCODE=$(db "SELECT checkin_code FROM leads LIMIT 1")
  curl -s $B/api/leads/$LID -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-get.json
  curl -s "$B/api/leads?page=1&limit=100" -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-list.json
  curl -s "$B/api/leads/search?q=maria" -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-search.json
  curl -s "$B/api/leads/history" -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-history.json
  curl -s "$B/api/leads/states" -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-states.json
  curl -s "$B/api/leads/export/$CID" -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-export.json
  curl -s $B/api/leads/confirm/$LCODE | eval "$NORM" > $O/l-validate.json
  curl -s -X POST $B/api/leads/confirm/$LCODE | eval "$NORM" > $O/l-confirm.json
  curl -s -X POST $B/api/leads/checkin/$LCODE | eval "$NORM" > $O/l-checkin.json
  curl -s -X POST $B/api/leads/$LID/mark-confirmation-sent -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-mark.json
  curl -s -X POST $B/api/campaigns/$CID/public-link -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d '{}' | eval "$NORM" > $O/p-genlink.json
  local PTOKEN=$(db "SELECT public_link_token FROM campaigns LIMIT 1")
  curl -s -X POST $B/api/campaigns/public/$PTOKEN/leads -H 'Content-Type: application/json' -d '{}' | eval "$NORM" > $O/p-leads.json
  local TID=$(db "SELECT id FROM message_templates WHERE campaign_id='$CID' ORDER BY sort_order LIMIT 1")
  curl -s -X POST $B/api/campaigns/public/$PTOKEN/leads/$LID/whatsapp/$TID -H 'Content-Type: application/json' -d '{}' | eval "$NORM" > $O/p-wa.json
  curl -s -X POST $B/api/campaigns/public/$PTOKEN/templates -H 'Content-Type: application/json' -d '{}' | eval "$NORM" > $O/p-tpls.json
  curl -s $B/api/campaigns/$CID/public-status -H "Authorization: Bearer $UT" | eval "$NORM" > $O/p-status.json
  curl -s -X POST $B/api/campaigns -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d '{"title":"Segunda","objectives":"workshop"}' -o /dev/null
  local CID2=$(db "SELECT id FROM campaigns WHERE title='Segunda'")
  curl -s -X POST $B/api/campaigns/$CID2/mark-paid -H "Authorization: Bearer $AT" -o /dev/null
  curl -s -X POST $B/api/campaigns/$CID2/reject -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"reason":"motivo x"}' | eval "$NORM" > $O/c-reject.json
  curl -s $B/api/og/$FTOKEN | eval "$HNORM" > $O/og.html
  curl -s $B/api/qr/$LCODE -o $O/qr.png; file -b $O/qr.png > $O/qr.file
  echo "capturado em $O"
}

echo "== postgres =="
$PG/pg_ctl -D /tmp/pgscratch -l /tmp/pgscratch.log -o "-p 5544 -k /tmp" start >/dev/null 2>&1
command -v npm >/dev/null && (cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-sistema-leads/apps/backend" && [ -d node_modules ] || npm ci --no-audit --no-fund >/dev/null 2>&1)

echo "===== NODE ====="
reset_db
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-sistema-leads/apps/backend"
TZ=UTC PORT=3552 NODE_ENV=production DB_HOST=127.0.0.1 DB_PORT=5544 DB_USER=postgres DB_PASSWORD=x DB_NAME=leads_parity \
  JWT_SECRET="segredo-de-teste-com-32-chars-ok!" FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3552 \
  node src/app.js >/tmp/leads-node.log 2>&1 &
NODEPID=$!
sleep 4
capture http://localhost:3552 /tmp/parity2-node
kill $NODEPID 2>/dev/null; sleep 1

echo "===== GO ====="
reset_db
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-leads-go" && go build -o /tmp/leads-go . || exit 1
DATABASE_URL="postgres://postgres@/leads_parity?host=/tmp&port=5544&sslmode=disable" \
  JWT_SECRET="segredo-de-teste-com-32-chars-ok!" PORT=3551 NODE_ENV=production \
  FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3551 \
  /tmp/leads-go >/tmp/leads-go.log 2>&1 &
GOPID=$!
sleep 2
capture http://localhost:3551 /tmp/parity2-go
kill $GOPID 2>/dev/null

echo ""
echo "===== DIFFS ====="
for f in c-create.json c-get.json c-list.json c-update.json c-submit1.json c-todraft.json c-reject.json c-submit2.json c-markpaid.json c-available.json c-pickup.json c-start.json c-health.json f-token.json l-create.json l-get.json l-list.json l-search.json l-history.json l-states.json l-export.json l-validate.json l-confirm.json l-checkin.json l-mark.json p-genlink.json p-leads.json p-wa.json p-tpls.json p-status.json; do
  if diff -q /tmp/parity2-node/$f /tmp/parity2-go/$f >/dev/null 2>&1; then echo "IGUAL: $f"
  else echo "--- DIFF: $f"; diff /tmp/parity2-node/$f /tmp/parity2-go/$f | head -n 8; fi
done
a=$(cat /tmp/parity2-node/c-submit3.code); b=$(cat /tmp/parity2-go/c-submit3.code)
[ "$a" == "$b" ] && echo "IGUAL: c-submit3.code ($a)" || echo "DIFF: c-submit3.code node=$a go=$b"
if diff -q /tmp/parity2-node/og.html /tmp/parity2-go/og.html >/dev/null 2>&1; then echo "IGUAL: og.html"; else echo "--- DIFF: og.html"; diff /tmp/parity2-node/og.html /tmp/parity2-go/og.html | head -n 8; fi
echo "qr node: $(cat /tmp/parity2-node/qr.file)"; echo "qr go: $(cat /tmp/parity2-go/qr.file)"
