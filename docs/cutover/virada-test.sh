#!/bin/bash
# Roda na VPS: teste ponta-a-ponta do Go contra dados reais + diff Node vs Go.
set -u
G=http://127.0.0.1:3101
N=http://10.0.1.26:3001
PASS=0; FAIL=0
check() { if [ "$2" == "$3" ]; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); echo "FALHOU: $1 -- esperado [$2] obtido [$3]"; fi }
J() { python3 -c "import json;print(json.load(open('$1'))$2)"; }
NORM="python3 -c \"import json,sys,re; d=json.load(sys.stdin); s=json.dumps(d, sort_keys=True); s=re.sub(r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}', 'UUID', s); s=re.sub(r'20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z?', 'TS', s); print(s)\""
T=$(mktemp); T2=$(mktemp)

DBURL=$(grep ^DATABASE_URL= /root/leads-go-backend.env | cut -d= -f2-)
export PGPASSWORD=$(python3 -c "import urllib.parse;print(urllib.parse.urlparse('$DBURL'.replace(chr(10),chr(0))).password or '')")
DBH=$(python3 -c "import urllib.parse;print(urllib.parse.urlparse('$DBURL'.replace(chr(10),chr(0))).hostname)")
DBN=$(python3 -c "import urllib.parse;p=urllib.parse.urlparse('$DBURL'.replace(chr(10),chr(0))).path;print(p.lstrip('/') or 'postgres')")
PSQL="docker exec -e PGPASSWORD=$PGPASSWORD $DBH psql -U postgres -d $DBN -tAc"
H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'
$PSQL "INSERT INTO users (id,email,password_hash,first_name,last_name,whatsapp,role,status,created_at,updated_at) VALUES ('99999999-9999-9999-9999-999999999999','virada-teste@gnosisbrasil.com','$H','Virada','Teste','11900000000','admin','active',now(),now())" >/dev/null
check "insert user" "1" "$($PSQL "SELECT count(*) FROM users WHERE email='virada-teste@gnosisbrasil.com'")"

CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/auth/login -H 'Content-Type: application/json' -d '{"email":"virada-teste@gnosisbrasil.com","password":"SenhaForte123"}')
check "captcha gate go" "CAPTCHA obrigatório" "$(J $T "['error']")"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $N/api/auth/login -H 'Content-Type: application/json' -d '{"email":"virada-teste@gnosisbrasil.com","password":"SenhaForte123"}')
check "captcha gate node" "CAPTCHA obrigatório" "$(J $T "['error']")"
export JSEC=$(grep ^JWT_SECRET= /root/leads-go-backend.env | cut -d= -f2-)
GTOK=$(python3 /tmp/mint.py)
NTOK=$GTOK
[ -z "$GTOK" ] && { echo "TOKEN VAZIO"; exit 1; }
CODE=$(curl -s -o $T -w "%{http_code}" $G/api/auth/me -H "Authorization: Bearer $GTOK")
check "me no go" "virada-teste@gnosisbrasil.com" "$(J $T "['email']")"
CODE=$(curl -s -o $T -w "%{http_code}" $N/api/auth/me -H "Authorization: Bearer $GTOK")
check "mesmo token vale no node" "virada-teste@gnosisbrasil.com" "$(J $T "['email']")"

curl -s "$N/api/campaigns?page=1&limit=5" -H "Authorization: Bearer $NTOK" | eval "$NORM" > $T
curl -s "$G/api/campaigns?page=1&limit=5" -H "Authorization: Bearer $GTOK" | eval "$NORM" > $T2
check "diff campaigns prod" "igual" "$(cmp -s $T $T2 && echo igual || echo DIFERENTE)"
curl -s "$N/api/reports/admin" -H "Authorization: Bearer $NTOK" | eval "$NORM" > $T
curl -s "$G/api/reports/admin" -H "Authorization: Bearer $GTOK" | eval "$NORM" > $T2
check "diff admin report prod" "igual" "$(cmp -s $T $T2 && echo igual || echo DIFERENTE)"
curl -s "$N/api/notifications?limit=5" -H "Authorization: Bearer $NTOK" | eval "$NORM" > $T
curl -s "$G/api/notifications?limit=5" -H "Authorization: Bearer $GTOK" | eval "$NORM" > $T2
check "diff notifications prod" "igual" "$(cmp -s $T $T2 && echo igual || echo DIFERENTE)"

CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/campaigns -H "Authorization: Bearer $GTOK" -H 'Content-Type: application/json' -d '{"title":"ZZZ TESTE VIRADA","budget":100,"objectives":"camara_publica","event_date":"2026-11-01T19:00:00Z","event_time":"19:00"}')
check "create campanha go" "201" "$CODE"
CID=$(J $T "['id']"); FTOK=$(J $T "['forms'][0]['public_token']")
$PSQL "UPDATE forms SET is_active=true WHERE campaign_id='$CID'" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" "$G/api/message-templates?campaign_id=$CID" -H "Authorization: Bearer $GTOK")
check "templates 7" "7" "$(python3 -c "import json;print(len(json.load(open('$T'))['data']))")"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/leads/public/$FTOK -H 'Content-Type: application/json' -d '{"first_name":"Teste","last_name":"Virada","whatsapp":"11900001111","email":"teste-virada@exemplo.com"}')
check "lead público go" "201" "$CODE"
LID=$(J $T "['lead']['id']")
CODE=$(curl -s -o $T -w "%{http_code}" $G/api/message-templates/lead/$LID/links -H "Authorization: Bearer $GTOK")
check "links 7" "7" "$(python3 -c "import json;print(len(json.load(open('$T'))['data']))")"
TPLP=$($PSQL "SELECT id FROM message_templates WHERE campaign_id='$CID' AND key='pedido_confirmacao'")
CODE=$(curl -s -o $T -w "%{http_code}" $G/api/message-templates/$TPLP/lead/$LID/link -H "Authorization: Bearer $GTOK")
check "link nome" "TESTE" "$(python3 -c "import json;print(json.load(open('$T'))['message'][4:9])")"
check "link não enviado" "False" "$(J $T "['sent_via_api']")"

CODE=$(curl -s -o $T -w "%{http_code}" $G/api/payment/config -H "Authorization: Bearer $GTOK")
check "pay config" "['pix']/True" "$(J $T "['methods']")/$(J $T "['configured']")"
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/payment/campaign/$CID -H "Authorization: Bearer $GTOK" -H 'Content-Type: application/json' -d '{"method":"pix"}')
echo "initiate go: HTTP=$CODE body=$(head -c 150 $T)"
check "initiate sem chave falha alto" "502" "$CODE"
check "initiate msg" "Falha ao gerar pagamento Pix" "$(J $T "['error']")"

printf '\x89PNG\r\n\x1a\nteste' > /tmp/virada-img.png
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/upload/image -H "Authorization: Bearer $GTOK" -F "image=@/tmp/virada-img.png;type=image/png")
check "upload go" "Upload realizado com sucesso" "$(J $T "['message']")"
FURL=$(J $T "['file']['url']"); FID=$(J $T "['file']['id']")
CODE=$(curl -s -o /dev/null -w "%{http_code}" $(echo "$FURL" | sed 's|https://itc4t27vg7wy1t7uht0w4pco.145.223.95.103.sslip.io|http://127.0.0.1:3101|'))
check "serve upload go" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $G/api/upload/$FID -H "Authorization: Bearer $GTOK")
check "delete upload go" "200" "$CODE"

CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $G/api/leads/$LID -H "Authorization: Bearer $GTOK")
check "delete lead go" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $G/api/campaigns/$CID -H "Authorization: Bearer $GTOK")
check "delete campanha go" "200" "$CODE"
$PSQL "DELETE FROM users WHERE email='virada-teste@gnosisbrasil.com'" >/dev/null
check "limpeza user" "0" "$($PSQL "SELECT count(*) FROM users WHERE email='virada-teste@gnosisbrasil.com'")"
check "limpeza campanha" "0" "$($PSQL "SELECT count(*) FROM campaigns WHERE title='ZZZ TESTE VIRADA'")"
check "limpeza lead" "0" "$($PSQL "SELECT count(*) FROM leads WHERE email='teste-virada@exemplo.com'")"
check "limpeza upload" "0" "$($PSQL "SELECT count(*) FROM uploads WHERE id='$FID'")"

echo "PASS=$PASS FAIL=$FAIL"
unset PGPASSWORD
[ "$FAIL" == "0" ] && echo VIRADA_TEST_OK || echo VIRADA_TEST_FALHOU
