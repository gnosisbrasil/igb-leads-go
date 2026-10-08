#!/bin/bash
# Paridade Fase 4: mesmos fluxos no Node e no Go, diff normalizado.
set -u
PG=/usr/local/bin
NORM="python3 -c \"import json,sys,re; d=json.load(sys.stdin); s=json.dumps(d, sort_keys=True); s=re.sub(r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}', 'UUID', s); s=re.sub(r'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+', 'JWT', s); s=re.sub(r'20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z?', 'TS', s); s=re.sub(r'[0-9]{2}/[0-9]{2}/[0-9]{4}, [0-9]{2}:[0-9]{2}:[0-9]{2}', 'DTBR', s); s=re.sub(r'\b[0-9a-f]{16}\b', 'TOKEN16', s); s=re.sub(r'\b[A-Z0-9]{12}\b', 'CHECKIN', s); s=re.sub(r'%2F[0-9A-F]{12}', '%2FCHECKIN', s); print(s)\""

reset_db() {
  $PG/psql -h /tmp -p 5544 -U postgres -c "DROP DATABASE IF EXISTS leads_parity;" -c "CREATE DATABASE leads_parity;" >/dev/null
  $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -q -f /tmp/leads-schema.sql >/dev/null 2>&1
  H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'
  $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -q <<SQL >/dev/null
INSERT INTO regions (id, name, code, country, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111','São Paulo','SP','Brasil',true,now(),now());
INSERT INTO users (id, email, password_hash, first_name, last_name, whatsapp, role, status, region_id, created_at, updated_at) VALUES
('22222222-2222-2222-2222-222222222222','admin@teste.com','$H','Ada','Admin','11999999999','admin','active','11111111-1111-1111-1111-111111111111',now(),now()),
('33333333-3333-3333-3333-333333333333','sup@teste.com','$H','Su','Pervisor','11966666666','supervisor','active','11111111-1111-1111-1111-111111111111',now(),now()),
('55555555-5555-5555-5555-555555555555','sup2@teste.com','$H','Su','SemRegiao','11955555555','supervisor','active',NULL,now(),now()),
('66666666-6666-6666-6666-666666666666','exec@teste.com','$H','Ex','Ecutivo','11944444444','executive','active',NULL,now(),now()),
('44444444-4444-4444-4444-444444444444','user@teste.com','$H','Comum','User','11977777777','user','active','11111111-1111-1111-1111-111111111111',now(),now());
SQL
}
db() { $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -tAc "$1"; }

capture() { # <base> <outdir>
  local B=$1 O=$2; mkdir -p $O
  local AT=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  local UT=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"user@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  local ST=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"sup@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  local S2T=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"sup2@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  local ET=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"exec@teste.com","password":"SenhaForte123"}' | python3 -c "import json,sys;print(json.load(sys.stdin)['token'])")
  curl -s -X POST $B/api/campaigns -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d '{"title":"Camp A","budget":100,"objectives":"camara_publica","address_city":"Campinas","event_date":"2026-11-01T19:00:00Z","event_time":"19:00"}' -o /dev/null
  curl -s -X POST $B/api/campaigns -H "Authorization: Bearer $UT" -H 'Content-Type: application/json' -d '{"title":"Camp B","budget":200}' -o /dev/null
  local CIDA=$(db "SELECT id FROM campaigns WHERE title='Camp A'")
  db "UPDATE campaigns SET region_id='11111111-1111-1111-1111-111111111111', status='in_progress', traffic_manager_id='66666666-6666-6666-6666-666666666666' WHERE title='Camp A'" >/dev/null
  db "UPDATE campaigns SET status='pending_approval' WHERE title='Camp B'" >/dev/null
  db "UPDATE forms SET is_active=true" >/dev/null
  local FTOKA=$(db "SELECT public_token FROM forms WHERE campaign_id='$CIDA'")
  curl -s -X POST $B/api/leads/public/$FTOKA -H 'Content-Type: application/json' -d '{"first_name":"Maria","last_name":"Silva","whatsapp":"(11) 98888-7777","email":"maria@teste.com"}' -o /dev/null
  curl -s -X POST $B/api/leads/public/$FTOKA -H 'Content-Type: application/json' -d '{"first_name":"João","last_name":"Souza","whatsapp":"11966665555","email":"joao@teste.com"}' -o /dev/null
  local LID=$(db "SELECT id FROM leads WHERE email='maria@teste.com'")
  db "UPDATE leads SET status='converted' WHERE email='joao@teste.com'" >/dev/null
  db "INSERT INTO notifications (id, user_id, title, message, type, is_read, created_at, updated_at) VALUES ('a1111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','N1','m1','info',false,now(),now()),('a2222222-2222-2222-2222-222222222222','22222222-2222-2222-2222-222222222222','N2','m2','info',true,now(),now())" >/dev/null
  db "INSERT INTO system_logs (id, user_id, action, description, created_at, updated_at) VALUES ('c1111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','login','Admin entrou',now(),now()),('c2222222-2222-2222-2222-222222222222',NULL,'webhook_pix','Pagamento recebido',now(),now())" >/dev/null
  # reports
  curl -s $B/api/reports/admin -H "Authorization: Bearer $AT" | eval "$NORM" > $O/r-admin.json
  curl -s -o /dev/null -w "%{http_code}" $B/api/reports/admin -H "Authorization: Bearer $UT" > $O/r-admin403.code
  curl -s $B/api/reports/system-logs -H "Authorization: Bearer $AT" | eval "$NORM" > $O/r-logs.json
  curl -s "$B/api/reports/system-logs?search=PAGAMENTO" -H "Authorization: Bearer $AT" | eval "$NORM" > $O/r-logs-search.json
  curl -s "$B/api/reports/system-logs?page=2&limit=1" -H "Authorization: Bearer $AT" | eval "$NORM" > $O/r-logs-page.json
  curl -s -o $O/r-logs-abc.json -w "%{http_code}" "$B/api/reports/system-logs?limit=abc" -H "Authorization: Bearer $AT" > $O/r-logs-abc.code; echo "" >> $O/r-logs-abc.code
  curl -s $B/api/reports/supervisor -H "Authorization: Bearer $ST" | eval "$NORM" > $O/r-sup.json
  curl -s -o /dev/null -w "%{http_code}" $B/api/reports/supervisor -H "Authorization: Bearer $S2T" > $O/r-sup400.code
  curl -s $B/api/reports/executive -H "Authorization: Bearer $ET" | eval "$NORM" > $O/r-exec.json
  # notifications
  curl -s $B/api/notifications -H "Authorization: Bearer $AT" | eval "$NORM" > $O/n-list.json
  curl -s "$B/api/notifications?unread_only=true" -H "Authorization: Bearer $AT" | eval "$NORM" > $O/n-unread.json
  curl -s $B/api/notifications/unread-count -H "Authorization: Bearer $AT" | eval "$NORM" > $O/n-count.json
  curl -s -X PATCH $B/api/notifications/a1111111-1111-1111-1111-111111111111/read -H "Authorization: Bearer $AT" | eval "$NORM" > $O/n-read.json
  curl -s -o /dev/null -w "%{http_code}" -X PATCH $B/api/notifications/00000000-0000-0000-0000-000000000000/read -H "Authorization: Bearer $AT" > $O/n-read404.code
  curl -s -X POST $B/api/notifications/mark-all-read -H "Authorization: Bearer $AT" | eval "$NORM" > $O/n-all.json
  curl -s -o $O/n-page-abc.json -w "%{http_code}" "$B/api/notifications?page=abc" -H "Authorization: Bearer $AT" > $O/n-page-abc.code; echo "" >> $O/n-page-abc.code
  # upload
  curl -s -X POST $B/api/upload/image -H "Authorization: Bearer $AT" -F "image=@/tmp/e2e4-img.png;type=image/png" | eval "$NORM" > $O/u-upload.json
  local FID=$(db "SELECT id FROM uploads LIMIT 1")
  curl -s -X POST $B/api/upload/image -H "Authorization: Bearer $AT" -F "image=@/tmp/e2e4-txt.txt;type=text/plain" | eval "$NORM" > $O/u-txt.json
  curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/upload/image -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{}' > $O/u-nofile.code
  curl -s -X DELETE $B/api/upload/$FID -H "Authorization: Bearer $AT" | eval "$NORM" > $O/u-delete.json
  curl -s -o /dev/null -w "%{http_code}" -X DELETE $B/api/upload/00000000-0000-0000-0000-000000000000 -H "Authorization: Bearer $AT" > $O/u-delete404.code
  # templates
  curl -s "$B/api/message-templates?campaign_id=$CIDA" -H "Authorization: Bearer $AT" | eval "$NORM" > $O/t-list.json
  curl -s -o /dev/null -w "%{http_code}" $B/api/message-templates -H "Authorization: Bearer $AT" > $O/t-list400.code
  curl -s -X POST $B/api/message-templates -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d "{\"campaign_id\":\"$CIDA\",\"key\":\"custom\",\"label\":\"Custom\",\"content\":\"Olá {{inscrito_nome}} (teste)\"}" | eval "$NORM" > $O/t-create.json
  local TPLC=$(db "SELECT id FROM message_templates WHERE campaign_id='$CIDA' AND key='custom'")
  curl -s $B/api/message-templates/$TPLC -H "Authorization: Bearer $AT" | eval "$NORM" > $O/t-get.json
  curl -s -X PUT $B/api/message-templates/$TPLC -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"label":"Custom 2","is_active":false}' | eval "$NORM" > $O/t-update.json
  curl -s -X DELETE $B/api/message-templates/$TPLC -H "Authorization: Bearer $AT" | eval "$NORM" > $O/t-delete.json
  local TPLD=$(db "SELECT id FROM message_templates WHERE campaign_id='$CIDA' AND key='pedido_confirmacao'")
  curl -s $B/api/message-templates/$TPLD/lead/$LID/link -H "Authorization: Bearer $AT" | eval "$NORM" > $O/t-link.json
  curl -s $B/api/message-templates/lead/$LID/links -H "Authorization: Bearer $AT" | eval "$NORM" > $O/t-links.json
  curl -s -o /dev/null -w "%{http_code}" $B/api/message-templates/$TPLD/lead/00000000-0000-0000-0000-000000000000/link -H "Authorization: Bearer $AT" > $O/t-link404.code
  db "DELETE FROM message_templates WHERE campaign_id='$CIDA'" >/dev/null
  curl -s -X POST $B/api/message-templates/campaign/$CIDA/seed-defaults -H "Authorization: Bearer $AT" | eval "$NORM" > $O/t-seed.json
  curl -s -X POST $B/api/message-templates/campaign/$CIDA/seed-defaults -H "Authorization: Bearer $AT" | eval "$NORM" > $O/t-seed0.json
  local TPLD2=$(db "SELECT id FROM message_templates WHERE campaign_id='$CIDA' AND key='pedido_confirmacao'")
  curl -s -o /dev/null -w "%{http_code}" -X PUT $B/api/message-templates/$TPLD2 -H "Authorization: Bearer $AT" -H 'Content-Type: application/json' -d '{"label":"X"}' > $O/t-update403.code
  curl -s -o /dev/null -w "%{http_code}" -X DELETE $B/api/message-templates/$TPLD2 -H "Authorization: Bearer $AT" > $O/t-delete403.code
  local JID=$(db "SELECT id FROM leads WHERE email='joao@teste.com'")
  curl -s -X DELETE $B/api/leads/$JID -H "Authorization: Bearer $UT" | eval "$NORM" > $O/l-delete.json
  curl -s -o /dev/null -w "%{http_code}" -X DELETE $B/api/leads/00000000-0000-0000-0000-000000000000 -H "Authorization: Bearer $UT" > $O/l-delete404.code
  echo "capturado em $O"
}

echo "== postgres =="
$PG/pg_ctl -D /tmp/pgscratch -l /tmp/pgscratch.log -o "-p 5544 -k /tmp" start >/dev/null 2>&1
mkdir -p "/Users/thiago/Documents/Trabalhos/Gnosis/igb-sistema-leads/apps/backend/uploads/campaigns"
printf '\x89PNG\r\n\x1a\nfakepng' > /tmp/e2e4-img.png
echo texto > /tmp/e2e4-txt.txt

echo "===== NODE ====="
reset_db
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-sistema-leads/apps/backend"
TZ=UTC PORT=3552 NODE_ENV=production DB_HOST=127.0.0.1 DB_PORT=5544 DB_USER=postgres DB_PASSWORD=x DB_NAME=leads_parity \
  JWT_SECRET="segredo-de-teste-com-32-chars-ok!" FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3999 \
  node src/app.js >/tmp/leads-node.log 2>&1 &
NODEPID=$!
sleep 4
capture http://localhost:3552 /tmp/parity4-node
kill $NODEPID 2>/dev/null; sleep 1

echo "===== GO ====="
reset_db
rm -rf /tmp/parity4-uploads
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-leads-go" && go build -o /tmp/leads-go . || exit 1
DATABASE_URL="postgres://postgres@/leads_parity?host=/tmp&port=5544&sslmode=disable" \
  JWT_SECRET="segredo-de-teste-com-32-chars-ok!" PORT=3551 NODE_ENV=production \
  FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3999 UPLOAD_DIR=/tmp/parity4-uploads \
  /tmp/leads-go >/tmp/leads-go.log 2>&1 &
GOPID=$!
sleep 2
capture http://localhost:3551 /tmp/parity4-go
kill $GOPID 2>/dev/null

echo ""
echo "===== DIFFS ====="
for f in r-admin.json r-logs.json r-logs-search.json r-logs-page.json r-sup.json r-exec.json n-list.json n-unread.json n-count.json n-read.json n-all.json u-upload.json u-txt.json u-delete.json t-list.json t-create.json t-get.json t-update.json t-delete.json t-seed.json t-seed0.json t-link.json t-links.json l-delete.json; do
  if diff -q /tmp/parity4-node/$f /tmp/parity4-go/$f >/dev/null 2>&1; then echo "IGUAL: $f"
  else echo "--- DIFF: $f"; diff /tmp/parity4-node/$f /tmp/parity4-go/$f | head -n 8; fi
done
for f in r-admin403.code r-sup400.code n-read404.code u-nofile.code u-delete404.code t-list400.code t-update403.code t-delete403.code t-link404.code l-delete404.code; do
  a=$(cat /tmp/parity4-node/$f); b=$(cat /tmp/parity4-go/$f)
  [ "$a" == "$b" ] && echo "IGUAL: $f ($a)" || echo "DIFF: $f node=$a go=$b"
done
echo "--- sondas NaN (alinhar depois se divergir) ---"
for f in r-logs-abc.code n-page-abc.code; do
  echo "$f node=$(cat /tmp/parity4-node/$f | tr -d '\n') go=$(cat /tmp/parity4-go/$f | tr -d '\n')"
done
echo "r-logs-abc body node: $(cat /tmp/parity4-node/r-logs-abc.json 2>/dev/null | head -c 200)"
echo "r-logs-abc body go: $(cat /tmp/parity4-go/r-logs-abc.json 2>/dev/null | head -c 200)"
echo "n-page-abc body node: $(cat /tmp/parity4-node/n-page-abc.json 2>/dev/null | head -c 200)"
echo "n-page-abc body go: $(cat /tmp/parity4-go/n-page-abc.json 2>/dev/null | head -c 200)"
