#!/bin/bash
# E2E Fase 4: relatórios, notificações, upload, templates de mensagem.
set -u
PG=/usr/local/bin
DIR="$(cd "$(dirname "$0")" && pwd)"
BASE=http://localhost:3551
PASS=0; FAIL=0
check() { if [ "$2" == "$3" ]; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); echo "FALHOU: $1 -- esperado [$2] obtido [$3]"; fi }
J() { python3 -c "import json;print(json.load(open('$1'))$2)"; }

$PG/pg_ctl -D /tmp/pgscratch -l /tmp/pgscratch.log -o "-p 5544 -k /tmp" start >/dev/null 2>&1
$PG/psql -h /tmp -p 5544 -U postgres -c "DROP DATABASE IF EXISTS leads;" -c "CREATE DATABASE leads;" >/dev/null
$PG/psql -h /tmp -p 5544 -U postgres -d leads -q -f /tmp/leads-schema.sql >/dev/null 2>&1
H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'
$PG/psql -h /tmp -p 5544 -U postgres -d leads -q <<SQL >/dev/null
INSERT INTO regions (id, name, code, country, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111','São Paulo','SP','Brasil',true,now(),now());
INSERT INTO users (id, email, password_hash, first_name, last_name, whatsapp, role, status, region_id, created_at, updated_at) VALUES
('22222222-2222-2222-2222-222222222222','admin@teste.com','$H','Ada','Admin','11999999999','admin','active','11111111-1111-1111-1111-111111111111',now(),now()),
('33333333-3333-3333-3333-333333333333','sup@teste.com','$H','Su','Pervisor','11966666666','supervisor','active','11111111-1111-1111-1111-111111111111',now(),now()),
('55555555-5555-5555-5555-555555555555','sup2@teste.com','$H','Su','SemRegiao','11955555555','supervisor','active',NULL,now(),now()),
('66666666-6666-6666-6666-666666666666','exec@teste.com','$H','Ex','Ecutivo','11944444444','executive','active',NULL,now(),now()),
('44444444-4444-4444-4444-444444444444','user@teste.com','$H','Comum','User','11977777777','user','active','11111111-1111-1111-1111-111111111111',now(),now());
SQL
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-leads-go" && go build -o /tmp/leads-go . || exit 1
export DATABASE_URL="postgres://postgres@/leads?host=/tmp&port=5544&sslmode=disable"
export JWT_SECRET="segredo-de-teste-com-32-chars-ok!"
export PORT=3551 NODE_ENV=production FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3551
export UPLOAD_DIR=/tmp/leads-uploads4
export MEOW_URL=http://127.0.0.1:3592 MEOW_API_KEY=test-key MEOW_SESSION=default
rm -rf /tmp/leads-uploads4 /tmp/mock-meow.log
python3 "$DIR/mocks/mock-meow-smtp.py" >/dev/null 2>&1 &
MOCKPID=$!
sleep 1
/tmp/leads-go >/tmp/leads-go.log 2>&1 &
SRV=$!
sleep 2
T=$(mktemp)
login() { curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"SenhaForte123\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['token'])"; }
ATOKEN=$(login admin@teste.com); STOKEN=$(login sup@teste.com); S2TOKEN=$(login sup2@teste.com); ETOKEN=$(login exec@teste.com); UTOKEN=$(login user@teste.com)
PSQL="$PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc"

# --- campanhas + lead ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"title":"Camp A","budget":100,"objectives":"camara_publica","event_date":"2026-11-01T19:00:00Z","event_time":"19:00"}')
check "campaign A 201" "201" "$CODE"
CIDA=$(J $T "['id']"); FTOKA=$(J $T "['forms'][0]['public_token']")
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"title":"Camp B","budget":200}')
check "campaign B 201" "201" "$CODE"
$PSQL "UPDATE campaigns SET region_id='11111111-1111-1111-1111-111111111111', status='in_progress', traffic_manager_id='66666666-6666-6666-6666-666666666666' WHERE title='Camp A'" >/dev/null
$PSQL "UPDATE campaigns SET status='completed', region_id=NULL WHERE title='Camp B'" >/dev/null
$PSQL "UPDATE forms SET is_active=true" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/leads/public/$FTOKA -H 'Content-Type: application/json' -d '{"first_name":"Maria","last_name":"Silva","whatsapp":"(11) 98888-7777","email":"maria@teste.com"}')
check "lead 201" "201" "$CODE"
LID=$(J $T "['lead']['id']")

# --- admin report ---
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/reports/admin -H "Authorization: Bearer $ATOKEN")
check "admin report 200" "200" "$CODE"
check "admin totals" "2/1/1/1/0/0" "$(J $T "['totalCampaigns']")/$(J $T "['activeCampaigns']")/$(J $T "['completedCampaigns']")/$(J $T "['totalLeads']")/$(J $T "['conversionRate']")/$(J $T "['pendingApprovals']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/reports/admin -H "Authorization: Bearer $UTOKEN")
check "admin report user 403" "403" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/reports/admin)
check "admin report sem token 401" "401" "$CODE"

# --- system logs ---
$PSQL "INSERT INTO system_logs (id, user_id, action, description, created_at, updated_at) VALUES ('c1111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','login','Admin entrou',now(),now()),('c2222222-2222-2222-2222-222222222222',NULL,'webhook_pix','Pagamento recebido',now(),now())" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" "$BASE/api/reports/system-logs?search=LOGIN" -H "Authorization: Bearer $ATOKEN")
check "logs search 200" "200" "$CODE"
check "logs search achou" "1" "$(python3 -c "import json;print(len(json.load(open('$T'))))")"
check "logs user aninhado" "Ada" "$(python3 -c "import json;print(json.load(open('$T'))[0]['user']['first_name'])")"
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/reports/system-logs -H "Authorization: Bearer $ATOKEN")
check "logs orfão null" "None" "$(python3 -c "import json;d=json.load(open('$T'));print([l['user'] for l in d if l['action']=='webhook_pix'][0])")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/reports/system-logs -H "Authorization: Bearer $STOKEN")
check "logs supervisor 403" "403" "$CODE"

# --- supervisor report ---
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/reports/supervisor -H "Authorization: Bearer $STOKEN")
check "sup report 200" "200" "$CODE"
check "sup região" "SP" "$(J $T "['region']['code']")"
check "sup totais" "1/0/1" "$(J $T "['totalCampaigns']")/$(J $T "['campaignsToApprove']")/$(J $T "['totalLeads']")"
check "sup campanha user" "Comum" "$(python3 -c "import json;print(json.load(open('$T'))['campaigns'][0]['user']['first_name'])")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/reports/supervisor -H "Authorization: Bearer $S2TOKEN")
check "sup sem região 400" "400" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/reports/supervisor -H "Authorization: Bearer $UTOKEN")
check "sup user 403" "403" "$CODE"

# --- executive report ---
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/reports/executive -H "Authorization: Bearer $ETOKEN")
check "exec report 200" "200" "$CODE"
check "exec totais" "1/1/0/1" "$(J $T "['assignedCampaigns']")/$(J $T "['activeCampaigns']")/$(J $T "['completedCampaigns']")/$(J $T "['totalLeadsGathered']")"
check "exec taxa string" "0.00" "$(J $T "['conversionRate']")"
check "exec aninhado" "new" "$(python3 -c "import json;print(json.load(open('$T'))['campaignsPerformance'][0]['forms'][0]['leads'][0]['status'])")"
$PSQL "UPDATE leads SET status='converted' WHERE id='$LID'" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/reports/executive -H "Authorization: Bearer $ETOKEN")
check "exec taxa 100" "100.00" "$(J $T "['conversionRate']")"
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/reports/admin -H "Authorization: Bearer $ATOKEN")
check "admin taxa numérica" "100" "$(J $T "['conversionRate']")"

# --- notificações ---
$PSQL "INSERT INTO notifications (id, user_id, title, message, type, is_read, created_at, updated_at) VALUES ('a1111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','N1','m1','info',false,now(),now()),('a2222222-2222-2222-2222-222222222222','22222222-2222-2222-2222-222222222222','N2','m2','info',false,now(),now()),('a3333333-3333-3333-3333-333333333333','22222222-2222-2222-2222-222222222222','N3','m3','info',true,now(),now()),('b1111111-1111-1111-1111-111111111111','44444444-4444-4444-4444-444444444444','NX','mx','info',false,now(),now())" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/notifications -H "Authorization: Bearer $ATOKEN")
check "notif list 200" "200" "$CODE"
check "notif paginação" "3/1" "$(J $T "['pagination']['total']")/$(J $T "['pagination']['totalPages']")"
CODE=$(curl -s -o $T -w "%{http_code}" "$BASE/api/notifications?unread_only=true" -H "Authorization: Bearer $ATOKEN")
check "notif unread_only" "2" "$(J $T "['pagination']['total']")"
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/notifications/unread-count -H "Authorization: Bearer $ATOKEN")
check "notif count" "2" "$(J $T "['unread_count']")"
CODE=$(curl -s -o $T -w "%{http_code}" -X PATCH $BASE/api/notifications/a1111111-1111-1111-1111-111111111111/read -H "Authorization: Bearer $ATOKEN")
check "notif mark read" "True" "$(J $T "['is_read']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH $BASE/api/notifications/b1111111-1111-1111-1111-111111111111/read -H "Authorization: Bearer $ATOKEN")
check "notif alheia 403" "403" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH $BASE/api/notifications/00000000-0000-0000-0000-000000000000/read -H "Authorization: Bearer $ATOKEN")
check "notif inexistente 404" "404" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/notifications/mark-all-read -H "Authorization: Bearer $ATOKEN")
check "notif mark all" "Todas as notificações marcadas como lidas" "$(J $T "['message']")"
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/notifications/unread-count -H "Authorization: Bearer $ATOKEN")
check "notif count zero" "0" "$(J $T "['unread_count']")"

# --- upload ---
printf '\x89PNG\r\n\x1a\nfakepng' > /tmp/e2e4-img.png
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/upload/image -H "Authorization: Bearer $ATOKEN" -F "image=@/tmp/e2e4-img.png;type=image/png")
check "upload 200" "200" "$CODE"
check "upload msg" "Upload realizado com sucesso" "$(J $T "['message']")"
FURL=$(J $T "['file']['url']"); FID=$(J $T "['file']['id']")
check "upload url prefixo" "http://localhost:3551/uploads/campaigns/" "$(echo "$FURL" | cut -c1-40)"
CODE=$(curl -s -o /tmp/e2e4-dl.png -w "%{http_code}" "$FURL")
check "upload serve 200" "200" "$CODE"
check "upload bytes" "0" "$(cmp -s /tmp/e2e4-img.png /tmp/e2e4-dl.png; echo $?)"
check "upload cache" "public, max-age=604800" "$(curl -s -o /dev/null -D - "$FURL" | grep -i '^cache-control' | tr -d '\r' | cut -d' ' -f2-)"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/upload/$FID -H "Authorization: Bearer $UTOKEN")
check "upload delete alheio 403" "403" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X DELETE $BASE/api/upload/$FID -H "Authorization: Bearer $ATOKEN")
check "upload delete msg" "Arquivo removido com sucesso" "$(J $T "['message']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$FURL")
check "upload após delete 404" "404" "$CODE"
echo "texto" > /tmp/e2e4-txt.txt
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/upload/image -H "Authorization: Bearer $ATOKEN" -F "image=@/tmp/e2e4-txt.txt;type=text/plain")
check "upload txt 500" "500" "$CODE"
check "upload txt msg" "Apenas imagens são permitidas (jpeg, jpg, png, gif, webp)" "$(J $T "['error']")"
python3 -c "open('/tmp/e2e4-big.png','wb').write(bytes(6*1024*1024))"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/upload/image -H "Authorization: Bearer $ATOKEN" -F "image=@/tmp/e2e4-big.png;type=image/png")
check "upload grande 500" "500" "$CODE"
check "upload grande msg" "File too large" "$(J $T "['error']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/upload/image -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{}')
check "upload sem arquivo 400" "400" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/upload/image -F "image=@/tmp/e2e4-img.png;type=image/png")
check "upload sem token 401" "401" "$CODE"

# --- templates: CRUD ---
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/message-templates -H "Authorization: Bearer $ATOKEN")
check "tpl sem campaign 400" "400" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" "$BASE/api/message-templates?campaign_id=$CIDA" -H "Authorization: Bearer $ATOKEN")
check "tpl list 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/message-templates -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d "{\"campaign_id\":\"$CIDA\",\"key\":\"custom\",\"label\":\"Custom\",\"content\":\"Olá {{inscrito_nome}}\"}")
check "tpl create 201" "201" "$CODE"
TPLC=$(J $T "['id']")
check "tpl create editable" "True" "$(J $T "['is_editable']")"
CODE=$(curl -s -o $T -w "%{http_code}" -X PUT $BASE/api/message-templates/$TPLC -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"label":"Custom 2"}')
check "tpl update label" "Custom 2" "$(J $T "['label']")"
CODE=$(curl -s -o $T -w "%{http_code}" -X DELETE $BASE/api/message-templates/$TPLC -H "Authorization: Bearer $ATOKEN")
check "tpl delete msg" "Template removido" "$(J $T "['message']")"
$PSQL "DELETE FROM message_templates WHERE campaign_id='$CIDA'" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/message-templates/campaign/$CIDA/seed-defaults -H "Authorization: Bearer $ATOKEN")
check "tpl seed msg" "7 templates criados" "$(J $T "['message']")"
TPLD=$(python3 -c "import json;print(json.load(open('$T'))['data'][0]['id'])")
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/message-templates/campaign/$CIDA/seed-defaults -H "Authorization: Bearer $ATOKEN")
check "tpl reseed zero" "0 templates criados" "$(J $T "['message']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT $BASE/api/message-templates/$TPLD -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"label":"X"}')
check "tpl default update 403" "403" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/message-templates/$TPLD -H "Authorization: Bearer $ATOKEN")
check "tpl default delete 403" "403" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/message-templates/00000000-0000-0000-0000-000000000000 -H "Authorization: Bearer $ATOKEN")
check "tpl get 404" "404" "$CODE"

# --- templates: links whatsapp ---
TPLP=$($PSQL "SELECT id FROM message_templates WHERE campaign_id='$CIDA' AND key='pedido_confirmacao'")
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/message-templates/$TPLP/lead/$LID/link -H "Authorization: Bearer $ATOKEN")
check "link 200" "200" "$CODE"
check "link phone" "5511988887777" "$(J $T "['phone']")"
check "link nome" "MARIA" "$(python3 -c "import json;print(json.load(open('$T'))['message'][4:9])")"
check "link enviada" "True" "$(J $T "['sent_via_api']")"
check "link url" "True" "$(python3 -c "import json;print(json.load(open('$T'))['whatsapp_url'].startswith('https://api.whatsapp.com/send?phone=5511988887777&text='))")"
check "meow recebeu" "1" "$(cat /tmp/mock-meow.log | wc -l | tr -d ' ')"
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/message-templates/lead/$LID/links -H "Authorization: Bearer $ATOKEN")
check "links 200" "200" "$CODE"
check "links qtd" "7" "$(python3 -c "import json;print(len(json.load(open('$T'))['data']))")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/message-templates/$TPLP/lead/00000000-0000-0000-0000-000000000000/link -H "Authorization:Bearer $ATOKEN")
check "link lead 404" "404" "$CODE"
$PSQL "UPDATE leads SET form_id=NULL WHERE id='$LID'" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/message-templates/$TPLP/lead/$LID/link -H "Authorization: Bearer $ATOKEN")
check "link órfão 400" "400" "$CODE"
check "link órfão msg" "Lead não está vinculado a uma campanha" "$(J $T "['error']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/message-templates/lead/$LID/links -H "Authorization: Bearer $ATOKEN")
check "links órfão 400" "400" "$CODE"

echo "PASS=$PASS FAIL=$FAIL"
kill $SRV $MOCKPID 2>/dev/null; $PG/pg_ctl -D /tmp/pgscratch stop >/dev/null 2>&1
[ "$FAIL" == "0" ] && echo E2E_OK || echo E2E_FALHOU
