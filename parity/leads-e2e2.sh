#!/bin/bash
# E2E Fase 2: campanhas, formulários, leads, links públicos, OG/QR.
set -u
PG=/usr/local/bin
BASE=http://localhost:3551
PASS=0; FAIL=0
check() { if [ "$2" == "$3" ]; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); echo "FALHOU: $1 -- esperado [$2] obtido [$3]"; fi }
J() { python3 -c "import json,sys; print(json.load(open('$1'))$2)"; }

$PG/pg_ctl -D /tmp/pgscratch -l /tmp/pgscratch.log -o "-p 5544 -k /tmp" start >/dev/null 2>&1
$PG/psql -h /tmp -p 5544 -U postgres -c "DROP DATABASE IF EXISTS leads;" -c "CREATE DATABASE leads;" >/dev/null
$PG/psql -h /tmp -p 5544 -U postgres -d leads -q -f /tmp/leads-schema.sql >/dev/null 2>&1
H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'
$PG/psql -h /tmp -p 5544 -U postgres -d leads -q <<SQL >/dev/null
INSERT INTO regions (id, name, code, country, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111','São Paulo','SP','Brasil',true,now(),now());
INSERT INTO users (id, email, password_hash, first_name, last_name, whatsapp, role, status, region_id, created_at, updated_at) VALUES
('22222222-2222-2222-2222-222222222222','admin@teste.com','$H','Ada','Admin','11999999999','admin','active','11111111-1111-1111-1111-111111111111',now(),now()),
('33333333-3333-3333-3333-333333333333','exec@teste.com','$H','Ex','Ecutivo','11988888888','executive','active','11111111-1111-1111-1111-111111111111',now(),now()),
('44444444-4444-4444-4444-444444444444','user@teste.com','$H','Comum','User','11977777777','user','active','11111111-1111-1111-1111-111111111111',now(),now());
SQL
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-leads-go" && go build -o /tmp/leads-go . || exit 1
export DATABASE_URL="postgres://postgres@/leads?host=/tmp&port=5544&sslmode=disable"
export JWT_SECRET="segredo-de-teste-com-32-chars-ok!"
export PORT=3551 NODE_ENV=production FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3551
/tmp/leads-go >/tmp/leads-go.log 2>&1 &
SRV=$!
sleep 2
T=$(mktemp)
login() { curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"SenhaForte123\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['token'])"; }
ATOKEN=$(login admin@teste.com); ETOKEN=$(login exec@teste.com); UTOKEN=$(login user@teste.com)

# --- campanha: ciclo de vida ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"title":"Palestra Pública Centro","description":"d","budget":1500.50,"objectives":"camara_publica","address_city":"Campinas","address_state":"SP","event_date":"2026-11-20T19:00:00Z","event_time":"19:00"}')
check "campaign create 201" "201" "$CODE"
CID=$(J $T "['id']"); check "campaign status payment_pending" "payment_pending" "$(J $T "['status']")"
check "campaign display_id" "1" "$(J $T "['display_id']")"
check "campaign form criado" "1" "$(J $T "['forms'].__len__()" 2>/dev/null || python3 -c "import json;print(len(json.load(open('$T'))['forms']))")"
NTPL=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT count(*) FROM message_templates WHERE campaign_id='$CID'")
check "campaign seed 7 templates" "7" "$NTPL"
FTOKEN=$(J $T "['forms'][0]['public_token']")

CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/campaigns/$CID -H "Authorization: Bearer $UTOKEN")
check "campaign get 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT $BASE/api/campaigns/$CID -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"location":"Centro de Convenções"}')
check "campaign update 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/to-draft -H "Authorization: Bearer $UTOKEN")
check "campaign to-draft 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/submit -H "Authorization: Bearer $UTOKEN")
check "campaign submit->payment 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/submit -H "Authorization: Bearer $UTOKEN")
check "campaign submit sem pagar 400" "400" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/mark-paid -H "Authorization: Bearer $ATOKEN")
check "campaign mark-paid 200" "200" "$CODE"
check "campaign paid+waiting" "waiting_executive" "$(J $T "['campaign']['status']")"
FACTIVE=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT is_active FROM forms WHERE campaign_id='$CID'")
check "form ativado" "t" "$FACTIVE"
CODE=$(curl -s -o $T -w "%{http_code}" "$BASE/api/campaigns/available" -H "Authorization: Bearer $ETOKEN")
check "available 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/pick-up -H "Authorization: Bearer $ETOKEN")
check "pick-up 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/start -H "Authorization: Bearer $ETOKEN")
check "start 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/propose-edit -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"address_city":"Nova Cidade","location":"IGNORAR"}')
check "propose-edit 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/approve-edit -H "Authorization: Bearer $ATOKEN")
check "approve-edit 200" "200" "$CODE"
check "approve-edit aplicou" "Nova Cidade" "$(J $T "['campaign']['address_city']")"
check "approve-edit ignorou location" "Centro de Convenções" "$(J $T "['campaign']['location']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/complete -H "Authorization: Bearer $ETOKEN")
check "complete 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/campaigns/$CID/health -H "Authorization: Bearer $ATOKEN")
check "health 200" "200" "$CODE"

# --- segunda campanha: reject/cancel/delete ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"title":"Segunda Campanha","objectives":"workshop"}')
CID2=$(J $T "['id']")
curl -s -o /dev/null -X POST $BASE/api/campaigns/$CID2/mark-paid -H "Authorization: Bearer $ATOKEN"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/$CID2/reject -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"reason":"teste"}')
check "reject 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/$CID2/cancel -H "Authorization: Bearer $UTOKEN")
check "cancel 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/campaigns/$CID2 -H "Authorization: Bearer $UTOKEN")
check "delete cancelada 200" "200" "$CODE"

# --- forms públicos ---
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/forms/token/$FTOKEN)
check "form by token 200" "200" "$CODE"
SLUG=$(J $T "['slug']")
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/forms/slug/$SLUG)
check "form by slug 200" "200" "$CODE"
FID=$(J $T "['id']")
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT $BASE/api/forms/$FID/short-code -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"short_code":"palestra-x"}')
check "short-code 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -o /dev/null -w "%{http_code}" --max-redirs 0 $BASE/api/forms/s/palestra-x)
check "short redirect 302" "302" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/forms/$FID/short-code -H "Authorization: Bearer $ATOKEN")
check "short remove 200" "200" "$CODE"

# --- leads: inscrição pública -> confirma -> checkin ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/leads/public/$FTOKEN -H 'Content-Type: application/json' -d '{"first_name":"Maria","last_name":"Silva","whatsapp":"(11) 98888-7777","email":"maria@teste.com"}')
check "lead public create 201" "201" "$CODE"
LID=$(J $T "['lead']['id']"); LCODE=$(J $T "['lead']['checkin_code']")
check "lead city da campanha" "Nova Cidade" "$(J $T "['lead']['city']")"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/leads/confirm/$LCODE)
check "validate confirm 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/leads/confirm/$LCODE)
check "do confirm 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/leads/checkin/$LCODE)
check "do checkin 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/leads/checkin/$LCODE)
check "checkin duplo 400" "400" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/leads/$LID -H "Authorization: Bearer $UTOKEN")
check "lead get 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" "$BASE/api/leads/search?q=maria" -H "Authorization: Bearer $UTOKEN")
check "lead search 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/leads/history" -H "Authorization: Bearer $UTOKEN")
check "lead history 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" "$BASE/api/leads/states" -H "Authorization: Bearer $UTOKEN")
check "lead states 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/leads/export/$CID" -H "Authorization: Bearer $UTOKEN")
check "lead export 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/leads/$LID/generate-qr -H "Authorization: Bearer $UTOKEN")
check "generate-qr 200" "200" "$CODE"
CODE=$(curl -s -o /tmp/qr1.png -w "%{http_code}" $BASE/api/qr/$LCODE); CT=$(file -b /tmp/qr1.png | cut -d, -f1)
check "qr png 200" "200" "$CODE"; check "qr é PNG" "PNG image data" "$CT"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/leads/qr/$LCODE)
check "leads qr 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/leads/$LID/mark-qr-sent -H "Authorization: Bearer $UTOKEN")
check "mark-qr-sent 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/leads/$LID/revert -H "Authorization: Bearer $UTOKEN")
check "revert 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH $BASE/api/leads/$LID/status -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"status":"converted"}')
check "update status 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH $BASE/api/leads/$LID/status -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"status":"invalido"}')
check "status inválido 400" "400" "$CODE"

# --- link público ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/public-link -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"password":"segredo"}')
check "gen link 200" "200" "$CODE"
PTOKEN=$(J $T "['token']")
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/public/$PTOKEN/leads -H 'Content-Type: application/json' -d '{}')
check "public sem senha 200" "200" "$CODE"
grep -q requires_password $T && check "requires_password" ok ok || check "requires_password" ok fail
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/public/$PTOKEN/leads -H 'Content-Type: application/json' -d '{"password":"errada"}')
check "public senha errada 401" "401" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/public/$PTOKEN/leads -H 'Content-Type: application/json' -d '{"password":"segredo"}')
check "public leads 200" "200" "$CODE"
check "public stats total" "1" "$(J $T "['stats']['total']")"
TID=$(J $T "['templates'][0]['id']")
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/public/$PTOKEN/leads/$LID/mark_confirmed -H 'Content-Type: application/json' -d '{"password":"segredo"}')
check "public action 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/public/$PTOKEN/leads/$LID/whatsapp/$TID -H 'Content-Type: application/json' -d '{"password":"segredo"}')
check "public wa link 200" "200" "$CODE"
grep -q "MARIA" $T && check "wa message substituída" ok ok || { check "wa message substituída" ok fail; cat $T; }
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/campaigns/public/$PTOKEN/checkin -H 'Content-Type: application/json' -d "{\"password\":\"segredo\",\"code\":\"$LCODE\"}")
check "public checkin 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/campaigns/$CID/public-status -H "Authorization: Bearer $UTOKEN")
check "public status 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/campaigns/$CID/public-link -H "Authorization: Bearer $UTOKEN")
check "revoke link 200" "200" "$CODE"

# --- OG ---
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/og/$FTOKEN)
check "og 200" "200" "$CODE"
grep -q "og:title" $T && check "og tags" ok ok || check "og tags" ok fail
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/og/inexistente)
check "og 404" "404" "$CODE"

# --- acesso negado entre usuários ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"title":"Do Admin"}')
CID3=$(J $T "['id']")
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/campaigns/$CID3 -H "Authorization: Bearer $UTOKEN")
check "user não vê campanha alheia 403" "403" "$CODE"

kill $SRV 2>/dev/null
echo ""
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" == "0" ] && echo E2E_OK || echo E2E_FALHOU
