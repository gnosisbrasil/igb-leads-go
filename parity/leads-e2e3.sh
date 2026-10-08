#!/bin/bash
# E2E Fase 3: Pix (mock Efí), webhook real, topup, autorelacionamento (mock meow).
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
('44444444-4444-4444-4444-444444444444','user@teste.com','$H','Comum','User','11977777777','user','active','11111111-1111-1111-1111-111111111111',now(),now());
SQL
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-leads-go" && go build -o /tmp/leads-go . || exit 1
export DATABASE_URL="postgres://postgres@/leads?host=/tmp&port=5544&sslmode=disable"
export JWT_SECRET="segredo-de-teste-com-32-chars-ok!"
export PORT=3551 NODE_ENV=production FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3551
export EFI_CLIENT_ID=id EFI_CLIENT_SECRET=secret EFI_PIX_KEY=chave-teste EFI_SANDBOX=true EFI_BASE_URL=http://127.0.0.1:3591
export MEOW_URL=http://127.0.0.1:3592 MEOW_API_KEY=test-key MEOW_SESSION=default
export SMTP_HOST=127.0.0.1 SMTP_PORT=3525 SMTP_USER=u SMTP_PASSWORD=p SMTP_FROM=noreply@teste.com
rm -f /tmp/mock-meow.log /tmp/mock-smtp.log /tmp/mock-efi-charges.log; echo "ATIVA" > /tmp/mock-efi-status.txt
python3 "$DIR/mocks/mock-efi.py" >/dev/null 2>&1 &
EFIPID=$!
python3 "$DIR/mocks/mock-meow-smtp.py" >/dev/null 2>&1 &
MOCKPID=$!
sleep 1
/tmp/leads-go >/tmp/leads-go.log 2>&1 &
SRV=$!
sleep 2
T=$(mktemp)
login() { curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"SenhaForte123\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['token'])"; }
ATOKEN=$(login admin@teste.com); UTOKEN=$(login user@teste.com)

# --- config ---
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/payment/config -H "Authorization: Bearer $ATOKEN")
check "pay config 200" "200" "$CODE"
check "pay config methods" "['pix']" "$(J $T "['methods']")"
check "pay config configured" "True" "$(J $T "['configured']")"

# --- campanha com auto_relationship + evento amanhã ---
TOMORROW=$(python3 -c "import datetime;print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1)).strftime('%Y-%m-%dT19:00:00Z'))")
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d "{\"title\":\"Camp Pix\",\"budget\":1500.50,\"objectives\":\"camara_publica\",\"address_city\":\"Campinas\",\"event_date\":\"$TOMORROW\",\"auto_relationship\":true}")
check "campaign create 201" "201" "$CODE"
CID=$(J $T "['id']"); FTOKEN=$(J $T "['forms'][0]['public_token']")

# --- initiate pix ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/payment/campaign/$CID -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"method":"pix"}')
check "initiate 200" "200" "$CODE"
check "initiate qr mock" "000201MOCKCAMP" "$(J $T "['payment_qr_code']" | cut -c1-14)"
check "initiate status" "pending" "$(J $T "['payment_status']")"
TXID=$(J $T "['txid']")
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/payment/campaign/$CID -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"method":"credit_card"}')
check "initiate cartão 400" "400" "$CODE"

# --- status poll (ATIVA -> segue pendente) ---
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/payment/campaign/$CID/status -H "Authorization: Bearer $UTOKEN")
check "status poll 200" "200" "$CODE"
check "status ainda pendente" "pending" "$(J $T "['payment_status']")"

# --- vira CONCLUIDA -> poll dá baixa ---
echo "CONCLUIDA" > /tmp/mock-efi-status.txt
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/payment/campaign/$CID/status -H "Authorization: Bearer $UTOKEN")
check "status paid" "paid" "$(J $T "['payment_status']")"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns/$CID/submit -H "Authorization: Bearer $UTOKEN")
check "submit após pagar" "disponível para executivos." "$(J $T "['message']" | rev | cut -c1-27 | rev)"

# --- webhook real numa 2a campanha ---
echo "ATIVA" > /tmp/mock-efi-status.txt
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"title":"Camp Webhook","budget":100}')
CID2=$(J $T "['id']")
curl -s -o $T -X POST $BASE/api/payment/campaign/$CID2 -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"method":"pix"}' -o /dev/null
TXID2=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT payment_txid FROM campaigns WHERE id='$CID2'")
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/payment/webhook -H 'Content-Type: application/json' -d "{\"pix\":[{\"endToEndId\":\"E123\",\"txid\":\"$TXID2\",\"valor\":\"100.00\"}]}")
check "webhook ATIVA 200" "200" "$CODE"
ST=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT payment_status FROM campaigns WHERE id='$CID2'")
check "webhook ATIVA mantém pendente" "pending" "$ST"
echo "CONCLUIDA" > /tmp/mock-efi-status.txt
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/payment/webhook -H 'Content-Type: application/json' -d "{\"pix\":[{\"endToEndId\":\"E123\",\"txid\":\"$TXID2\",\"valor\":\"100.00\"}]}")
check "webhook 200" "200" "$CODE"
check "webhook ack" "received" "$(J $T "['status']")"
ST=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT payment_status || '/' || status FROM campaigns WHERE id='$CID2'")
check "webhook deu baixa+avançou" "paid/waiting_executive" "$ST"
NOTIF=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT count(*) FROM notifications WHERE user_id='44444444-4444-4444-4444-444444444444' AND title='Pagamento Confirmado'")
check "webhook notificou" "1" "$NOTIF"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/payment/webhook -H 'Content-Type: application/json' -d '{"pix":[{"txid":"DESCONHECIDO123"}]}')
check "webhook txid desconhecido 200" "200" "$CODE"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/campaigns -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"title":"Camp Forjada","budget":100}')
CID3=$(J $T "['id']")
$PG/psql -h /tmp -p 5544 -U postgres -d leads -q -c "UPDATE campaigns SET payment_txid='FORJADO123', payment_status='pending' WHERE id='$CID3'" >/dev/null
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/payment/webhook -H 'Content-Type: application/json' -d '{"pix":[{"txid":"FORJADO123"}]}')
check "webhook forjado 200" "200" "$CODE"
ST=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT payment_status FROM campaigns WHERE id='$CID3'")
check "webhook forjado mantém pendente" "pending" "$ST"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/payment/webhook/pix -H 'Content-Type: application/json' -d '{"pix":[{"txid":"DESCONHECIDO123"}]}')
check "webhook /pix 200" "200" "$CODE"

# --- topup ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/payment/campaign/$CID/topup -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"amount":500}')
check "topup 200" "200" "$CODE"
check "topup total" "2000.5" "$(J $T "['total_budget']")"
BUDGET=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT budget FROM campaigns WHERE id='$CID'")
check "topup budget banco" "2000.50" "$BUDGET"

# --- autorelacionamento: inscrição -> pedido confirmação via meow ---
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/leads/public/$FTOKEN -H 'Content-Type: application/json' -d '{"first_name":"Maria","last_name":"Silva","whatsapp":"11988887777","email":"maria@teste.com"}')
check "lead public 201" "201" "$CODE"
LID=$(J $T "['lead']['id']")
sleep 1
MEOW1=$(cat /tmp/mock-meow.log 2>/dev/null | wc -l | tr -d ' ')
check "meow pedido confirmação" "1" "$MEOW1"
grep -q "MARIA" /tmp/mock-meow.log && check "meow substituiu nome" ok ok || check "meow substituiu nome" ok fail
STAMP=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT confirmation_sent_at IS NOT NULL FROM leads WHERE id='$LID'")
check "stamp confirmation" "t" "$STAMP"

# --- confirmação -> agradecimento ---
LCODE=$(J $T "['lead']['checkin_code']")
curl -s -X POST $BASE/api/leads/confirm/$LCODE -o /dev/null
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/leads/$LID/mark-confirmed -H "Authorization: Bearer $UTOKEN")
check "mark confirmed 200" "200" "$CODE"
sleep 1
MEOW2=$(cat /tmp/mock-meow.log 2>/dev/null | wc -l | tr -d ' ')
check "meow agradecimento" "2" "$MEOW2"

# --- forgot password envia e-mail real (mock SMTP) ---
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/forgot-password -H 'Content-Type: application/json' -d '{"email":"user@teste.com"}')
check "forgot 200" "200" "$CODE"
sleep 1
grep -q "Redefinir Senha" /tmp/mock-smtp.log 2>/dev/null && check "smtp reset enviado" ok ok || check "smtp reset enviado" ok fail

kill $SRV $EFIPID $MOCKPID 2>/dev/null
echo ""
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" == "0" ] && echo E2E_OK || echo E2E_FALHOU
