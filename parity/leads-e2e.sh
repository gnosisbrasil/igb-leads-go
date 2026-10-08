#!/bin/bash
# Probe E2E Fase 1 (descartável): schema real + fluxos auth/users/regions.
set -u
PG=/usr/local/bin
BASE=http://localhost:3551
PASS=0; FAIL=0
check() { # <nome> <esperado> <obtido>
  if [ "$2" == "$3" ]; then PASS=$((PASS+1)); # echo "ok: $1"
  else FAIL=$((FAIL+1)); echo "FALHOU: $1 -- esperado [$2] obtido [$3]"; fi
}
code_of() { tail -n1 "$1"; }
body_of() { sed '$d' "$1"; }

echo "== sobe postgres scratch =="
$PG/pg_ctl -D /tmp/pgscratch -l /tmp/pgscratch.log -o "-p 5544 -k /tmp" start >/dev/null 2>&1
$PG/psql -h /tmp -p 5544 -U postgres -c "DROP DATABASE IF EXISTS leads;" -c "CREATE DATABASE leads;" >/dev/null
$PG/psql -h /tmp -p 5544 -U postgres -d leads -q -f /tmp/leads-schema.sql >/dev/null 2>&1
# seed: região + admin (hash bcryptjs de 'SenhaForte123') + supervisor
$PG/psql -h /tmp -p 5544 -U postgres -d leads -q <<'SQL'
INSERT INTO regions (id, name, code, country, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111','São Paulo','SP','Brasil',true,now(),now());
INSERT INTO users (id, email, password_hash, first_name, last_name, whatsapp, role, status, region_id, created_at, updated_at)
VALUES ('22222222-2222-2222-2222-222222222222','admin@teste.com','$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW','Ada','Admin','11999999999','admin','active','11111111-1111-1111-1111-111111111111',now(),now());
SQL
echo "== sobe API Go =="
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-leads-go" && go build -o /tmp/leads-go . || exit 1
export DATABASE_URL="postgres://postgres@/leads?host=/tmp&port=5544&sslmode=disable"
export JWT_SECRET="segredo-de-teste-com-32-chars-ok!"
export PORT=3551 NODE_ENV=production FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3551
/tmp/leads-go >/tmp/leads-go.log 2>&1 &
SRV=$!
sleep 2

T=$(mktemp)
# 1. register
curl -s -o $T -w "%{http_code}" -X POST $BASE/api/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"novo@teste.com","password":"SenhaNova123","first_name":"Novo","last_name":"User","whatsapp":"(11) 98888-7777","state":"SP","city":"São Paulo"}' > $T.code; CODE=$(cat $T.code); BODY=$(cat $T)
check "register 201" "201" "$CODE"
echo "$BODY" | grep -q "Aguarde a aprovação" && check "register msg" ok ok || check "register msg" ok fail
# 2. duplicado
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/register -H 'Content-Type: application/json' -d '{"email":"novo@teste.com","password":"x","first_name":"a","last_name":"b","whatsapp":"1"}')
check "register dup 400" "400" "$CODE"
# 3. login pendente -> 403
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d '{"email":"novo@teste.com","password":"SenhaNova123"}')
check "login pending 403" "403" "$CODE"
grep -q '"status":"pending"' $T && check "login pending body" ok ok || check "login pending body" ok fail
# 4. login admin (hash bcryptjs!) -> 200
curl -s -o $T -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@teste.com","password":"SenhaForte123"}' > $T.code; CODE=$(cat $T.code)
check "login admin 200" "200" "$CODE"
ATOKEN=$(python3 -c "import json;print(json.load(open('$T'))['token'])")
RTOKEN=$(python3 -c "import json;print(json.load(open('$T'))['refreshToken'])")
[ -n "$ATOKEN" ] && check "token presente" ok ok || check "token presente" ok fail
# 5. me
CODE=$(curl -s -o $T -w "%{http_code}" $BASE/api/auth/me -H "Authorization: Bearer $ATOKEN")
check "me 200" "200" "$CODE"
grep -q '"email":"admin@teste.com"' $T && grep -q '"region":{"id":"11111111' $T && check "me region join" ok ok || { check "me region join" ok fail; cat $T; }
grep -q 'password_hash' $T && check "me sem hash" ok fail || check "me sem hash" ok ok
# 6. sem token -> 401
CODE=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/auth/me)
check "me sem token 401" "401" "$CODE"
# 7. approve
NUSER=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT id FROM users WHERE email='novo@teste.com'")
CODE=$(curl -s -o $T -w "%{http_code}" -X PATCH $BASE/api/users/$NUSER/approve -H "Authorization: Bearer $ATOKEN")
check "approve 200" "200" "$CODE"
NOTIF=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT count(*) FROM notifications WHERE user_id='$NUSER' AND type='success'")
check "notificação aprovação" "1" "$NOTIF"
# 8. login aprovado + refresh
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d '{"email":"novo@teste.com","password":"SenhaNova123"}')
check "login aprovado 200" "200" "$CODE"
UTOKEN=$(python3 -c "import json;print(json.load(open('$T'))['token'])")
UREFRESH=$(python3 -c "import json;print(json.load(open('$T'))['refreshToken'])")
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/auth/refresh -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$UREFRESH\"}")
check "refresh 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/refresh -H 'Content-Type: application/json' -d '{"refreshToken":"lixo"}')
check "refresh lixo 401" "401" "$CODE"
# 9. users list (admin)
CODE=$(curl -s -o $T -w "%{http_code}" "$BASE/api/users?page=1&limit=20" -H "Authorization: Bearer $ATOKEN")
check "users list 200" "200" "$CODE"
python3 -c "import json;d=json.load(open('$T'));assert d['pagination']['total']==2, d['pagination'];rows={u['email']:u for u in d['data']};assert rows['admin@teste.com']['region']['code']=='SP', rows;assert rows['novo@teste.com']['region'] is None, rows" && check "users paginação+region" ok ok || check "users paginação+region" ok fail
# 10. users list (user comum) -> 403
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/users" -H "Authorization: Bearer $UTOKEN")
check "users list user 403" "403" "$CODE"
# 11. create + update + suspend + reactivate + delete
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/users -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"first_name":"Cri","last_name":"Ado","email":"cri@teste.com","password":"Senha12345","whatsapp":"11911112222","role":"user","region_id":"11111111-1111-1111-1111-111111111111"}')
check "users create 201" "201" "$CODE"
CID=$(python3 -c "import json;print(json.load(open('$T'))['id'])")
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT $BASE/api/users/$CID -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"city":"Campinas"}')
check "users update 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH $BASE/api/users/$CID/suspend -H "Authorization: Bearer $ATOKEN")
check "users suspend 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d '{"email":"cri@teste.com","password":"Senha12345"}')
check "login suspenso 403" "403" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH $BASE/api/users/$CID/reactivate -H "Authorization: Bearer $ATOKEN")
check "users reactivate 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/users/$CID -H "Authorization: Bearer $ATOKEN")
check "users delete 200" "200" "$CODE"
# 12. regions CRUD
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/regions -H "Authorization: Bearer $ATOKEN" -H 'Content-Type: application/json' -d '{"name":"Rio","code":"rj","description":"x"}')
check "regions create 201" "201" "$CODE"
grep -q '"code":"RJ"' $T && check "regions code upper" ok ok || check "regions code upper" ok fail
RID=$(python3 -c "import json;print(json.load(open('$T'))['id'])")
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/regions -H "Authorization: Bearer $UTOKEN" -H 'Content-Type: application/json' -d '{"name":"x","code":"xx"}')
check "regions create user 403" "403" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/regions/11111111-1111-1111-1111-111111111111 -H "Authorization: Bearer $ATOKEN")
check "regions delete vinculada 400" "400" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE $BASE/api/regions/$RID -H "Authorization: Bearer $ATOKEN")
check "regions delete livre 200" "200" "$CODE"
# 13. forgot + reset (sem SMTP)
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/auth/forgot-password -H 'Content-Type: application/json' -d '{"email":"novo@teste.com"}')
check "forgot 200" "200" "$CODE"
TOKENDB=$($PG/psql -h /tmp -p 5544 -U postgres -d leads -tAc "SELECT reset_password_token FROM users WHERE email='novo@teste.com'")
[ -n "$TOKENDB" ] && check "reset token gravado" ok ok || check "reset token gravado" ok fail
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/reset-password -H 'Content-Type: application/json' -d "{\"token\":\"$TOKENDB\",\"password\":\"NovaSenha999\"}")
check "reset 200" "200" "$CODE"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d '{"email":"novo@teste.com","password":"NovaSenha999"}')
check "login nova senha 200" "200" "$CODE"
# 14. disconnect google (Node sempre 400; Go implementa)
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/disconnect/google -H "Authorization: Bearer $ATOKEN")
check "disconnect 200" "200" "$CODE"
# 15. rate limit: estourar logins (10/15min por IP; já usamos ~8)
for i in $(seq 1 5); do curl -s -o /dev/null -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d '{"email":"x@y.z","password":"w"}'; done
CODE=$(curl -s -o $T -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' -d '{"email":"x@y.z","password":"w"}')
check "rate limit 429" "429" "$CODE"
grep -q "Muitas tentativas" $T && check "rate limit msg" ok ok || check "rate limit msg" ok fail

kill $SRV 2>/dev/null
echo ""
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" == "0" ] && echo E2E_OK || echo E2E_FALHOU
