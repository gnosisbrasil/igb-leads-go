#!/bin/bash
# Pós-virada: tudo pela URL pública (mesmo caminho do frontend).
set -u
G=https://itc4t27vg7wy1t7uht0w4pco.145.223.95.103.sslip.io
PASS=0; FAIL=0
check() { if [ "$2" == "$3" ]; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); echo "FALHOU: $1 -- esperado [$2] obtido [$3]"; fi }
J() { python3 -c "import json;print(json.load(open('$1'))$2)"; }
T=$(mktemp)
DBURL=$(grep ^DATABASE_URL= /root/leads-go-backend.env | cut -d= -f2-)
export PGPASSWORD=$(python3 -c "import urllib.parse;print(urllib.parse.urlparse('$DBURL'.replace(chr(10),chr(0))).password or '')")
DBH=$(python3 -c "import urllib.parse;print(urllib.parse.urlparse('$DBURL'.replace(chr(10),chr(0))).hostname)")
PSQL="docker exec -e PGPASSWORD=$PGPASSWORD $DBH psql -U postgres -d postgres -tAc"
H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'

check "front 200" "200" "$(curl -s -o /dev/null -w "%{http_code}" https://leads.gnosisbrasil.com/ --max-time 20)"
check "front chama api" "3" "$(curl -s https://leads.gnosisbrasil.com/assets/index-DGV7BixL.js --max-time 30 | grep -c 'itc4t27vg7wy1t7uht0w4pco.145.223.95.103.sslip.io/api')"
check "health" "200" "$(curl -s -o $T -w "%{http_code}" $G/health --max-time 20)"
check "health go" "ok" "$(J $T "['status']")"
check "config" "0x4AAAAAADT8hvZahAfefIsu" "$(curl -s $G/api/config --max-time 20 | python3 -c "import json,sys;print(json.load(sys.stdin)['CLOUDFLARE_TURNSTILE_SITE_KEY'])")"

$PSQL "INSERT INTO users (id,email,password_hash,first_name,last_name,whatsapp,role,status,created_at,updated_at) VALUES ('99999999-9999-9999-9999-999999999999','pos-virada@gnosisbrasil.com','$H','Pos','Virada','11900000000','admin','active',now(),now())" >/dev/null
export JSEC=$(grep ^JWT_SECRET= /root/leads-go-backend.env | cut -d= -f2-)
TOK=$(python3 -c "
import hmac,hashlib,base64,json,time,os
sec=os.environ['JSEC'].encode()
b=lambda d: base64.urlsafe_b64encode(d).rstrip(b'=').decode()
now=int(time.time())
h=b(b'{\"alg\":\"HS256\",\"typ\":\"JWT\"}')
p=b(json.dumps({'id':'99999999-9999-9999-9999-999999999999','email':'pos-virada@gnosisbrasil.com','role':'admin','iat':now,'exp':now+600},separators=(',',':')).encode())
print(h+'.'+p+'.'+b(hmac.new(sec,(h+'.'+p).encode(),hashlib.sha256).digest()))")
check "me" "pos-virada@gnosisbrasil.com" "$(curl -s $G/api/auth/me -H "Authorization: Bearer $TOK" --max-time 20 | python3 -c "import json,sys;print(json.load(sys.stdin)['email'])")"
check "campaigns" "200" "$(curl -s -o /dev/null -w "%{http_code}" "$G/api/campaigns?page=1&limit=5" -H "Authorization: Bearer $TOK" --max-time 20)"
check "admin report" "200" "$(curl -s -o /dev/null -w "%{http_code}" $G/api/reports/admin -H "Authorization: Bearer $TOK" --max-time 20)"
check "campaign create" "201" "$(curl -s -o $T -w "%{http_code}" -X POST $G/api/campaigns -H "Authorization: Bearer $TOK" -H 'Content-Type: application/json' -d '{"title":"ZZZ POS VIRADA","budget":50,"objectives":"workshop"}' --max-time 20)"
CID=""; if [ -s $T ]; then CID=$(J $T "['id']" 2>/dev/null || echo ""); fi
check "campaign criada" "não-vazio" "$([ -n "$CID" ] && echo não-vazio || echo vazio)"
FTOK=$($PSQL "SELECT public_token FROM forms WHERE campaign_id='$CID'")
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/leads/public/$FTOK -H 'Content-Type: application/json' -d '{"first_name":"Pos","last_name":"Virada","whatsapp":"11900002222","email":"pos-virada-lead@exemplo.com"}' --max-time 20)
$PSQL "UPDATE forms SET is_active=true WHERE campaign_id='$CID'" >/dev/null
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/leads/public/$FTOK -H 'Content-Type: application/json' -d '{"first_name":"Pos","last_name":"Virada","whatsapp":"11900002222","email":"pos-virada-lead@exemplo.com"}' --max-time 20)
check "lead público" "201" "$CODE"
LID=$(J $T "['lead']['id']")
check "links" "7" "$(curl -s $G/api/message-templates/lead/$LID/links -H "Authorization: Bearer $TOK" --max-time 20 | python3 -c "import json,sys;print(len(json.load(sys.stdin)['data']))")"
check "notificações" "200" "$(curl -s -o /dev/null -w "%{http_code}" $G/api/notifications -H "Authorization: Bearer $TOK" --max-time 20)"
printf '\x89PNG\r\n\x1a\npos' > /tmp/pos-img.png
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $G/api/upload/image -H "Authorization: Bearer $TOK" -F "image=@/tmp/pos-img.png;type=image/png" --max-time 20)
FURL=$(J $T "['file']['url']"); FID=$(J $T "['file']['id']")
check "upload serve público" "200" "$(curl -s -o /dev/null -w "%{http_code}" "$FURL" --max-time 20)"
curl -s -o /dev/null -X DELETE $G/api/upload/$FID -H "Authorization: Bearer $TOK" --max-time 20
curl -s -o /dev/null -X DELETE $G/api/leads/$LID -H "Authorization: Bearer $TOK" --max-time 20
curl -s -o /dev/null -X DELETE $G/api/campaigns/$CID -H "Authorization: Bearer $TOK" --max-time 20
$PSQL "DELETE FROM users WHERE email='pos-virada@gnosisbrasil.com'" >/dev/null
check "limpeza" "0/0/0" "$($PSQL "SELECT count(*) FROM users WHERE email='pos-virada@gnosisbrasil.com'")/$($PSQL "SELECT count(*) FROM campaigns WHERE id='$CID'")/$($PSQL "SELECT count(*) FROM leads WHERE id='$LID'")"
echo "PASS=$PASS FAIL=$FAIL"
unset PGPASSWORD JSEC
[ "$FAIL" == "0" ] && echo POS_VIRADA_OK || echo POS_VIRADA_FALHOU
