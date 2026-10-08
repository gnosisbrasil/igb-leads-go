#!/bin/bash
# Paridade Node x Go: mesmas requisições, diff dos JSONs normalizados.
set -u
PG=/usr/local/bin
NORM="python3 -c \"import json,sys,re; d=json.load(sys.stdin); s=json.dumps(d, sort_keys=True); s=re.sub(r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}', 'UUID', s); s=re.sub(r'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+', 'JWT', s); s=re.sub(r'20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z?', 'TS', s); print(s)\""

reset_db() {
  $PG/psql -h /tmp -p 5544 -U postgres -c "DROP DATABASE IF EXISTS leads_parity;" -c "CREATE DATABASE leads_parity;" >/dev/null
  $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -q -f /tmp/leads-schema.sql >/dev/null 2>&1
  $PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -q <<'SQL'
INSERT INTO regions (id, name, code, country, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111','São Paulo','SP','Brasil',true,now(),now());
INSERT INTO users (id, email, password_hash, first_name, last_name, whatsapp, role, status, region_id, created_at, updated_at)
VALUES ('22222222-2222-2222-2222-222222222222','admin@teste.com','$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW','Ada','Admin','11999999999','admin','active','11111111-1111-1111-1111-111111111111',now(),now());
SQL
}

capture() { # <base> <outdir>
  local B=$1 O=$2; mkdir -p $O
  curl -s -X POST $B/api/auth/register -H 'Content-Type: application/json' \
    -d '{"email":"novo@teste.com","password":"SenhaNova123","first_name":"Novo","last_name":"User","whatsapp":"(11) 98888-7777","state":"SP","city":"São Paulo"}' | eval "$NORM" > $O/register.json; echo "register: $(cat $O/register.json)"
  curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' \
    -d '{"email":"admin@teste.com","password":"SenhaForte123"}' > $O/login.raw
  cat $O/login.raw | eval "$NORM" > $O/login.json; echo "login: $(cat $O/login.json)"
  local T=$(python3 -c "import json;print(json.load(open('$O/login.raw'))['token'])")
  curl -s $B/api/auth/me -H "Authorization: Bearer $T" | eval "$NORM" > $O/me.json; echo "me: $(cat $O/me.json)"
  curl -s "$B/api/users?page=1&limit=20" -H "Authorization: Bearer $T" | eval "$NORM" > $O/users.json; echo "users: $(cat $O/users.json)"
  local NID=$(python3 -c "import json;d=json.load(open('$O/users.json'.replace('.json','.raw'))) if False else None" 2>/dev/null)
  NID=$($PG/psql -h /tmp -p 5544 -U postgres -d leads_parity -tAc "SELECT id FROM users WHERE email='novo@teste.com'")
  curl -s $B/api/users/$NID -H "Authorization: Bearer $T" | eval "$NORM" > $O/userbyid.json; echo "userbyid: $(cat $O/userbyid.json)"
  curl -s $B/api/regions -H "Authorization: Bearer $T" | eval "$NORM" > $O/regions.json; echo "regions: $(cat $O/regions.json)"
  curl -s -X POST $B/api/regions -H "Authorization: Bearer $T" -H 'Content-Type: application/json' \
    -d '{"name":"Rio","code":"rj","description":"x"}' | eval "$NORM" > $O/regioncreate.json; echo "regioncreate: $(cat $O/regioncreate.json)"
  # erros
  curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@teste.com","password":"errada"}' > $O/err-login.code
  curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' -d '{"email":"admin@teste.com","password":"errada"}' | eval "$NORM" > $O/err-login.json
  curl -s -o /dev/null -w "%{http_code}" $B/api/auth/me > $O/err-notoken.code
  curl -s $B/api/auth/me | eval "$NORM" > $O/err-notoken.json
  echo "err-login: $(cat $O/err-login.code) $(cat $O/err-login.json) | err-notoken: $(cat $O/err-notoken.code) $(cat $O/err-notoken.json)"
}

echo "== postgres =="
$PG/pg_ctl -D /tmp/pgscratch -l /tmp/pgscratch.log -o "-p 5544 -k /tmp" start >/dev/null 2>&1

echo "===== NODE ====="
reset_db
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-sistema-leads/apps/backend"
PORT=3552 NODE_ENV=production DB_HOST=127.0.0.1 DB_PORT=5544 DB_USER=postgres DB_PASSWORD=x DB_NAME=leads_parity \
  JWT_SECRET="segredo-de-teste-com-32-chars-ok!" FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3552 \
  node src/app.js >/tmp/leads-node.log 2>&1 &
NODEPID=$!
sleep 4
capture http://localhost:3552 /tmp/parity-node
kill $NODEPID 2>/dev/null; sleep 1

echo ""
echo "===== GO ====="
reset_db
cd "/Users/thiago/Documents/Trabalhos/Gnosis/igb-leads-go" && go build -o /tmp/leads-go . || exit 1
DATABASE_URL="postgres://postgres@/leads_parity?host=/tmp&port=5544&sslmode=disable" \
  JWT_SECRET="segredo-de-teste-com-32-chars-ok!" PORT=3551 NODE_ENV=production \
  FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3551 \
  /tmp/leads-go >/tmp/leads-go.log 2>&1 &
GOPID=$!
sleep 2
capture http://localhost:3551 /tmp/parity-go
kill $GOPID 2>/dev/null

echo ""
echo "===== DIFFS ====="
for f in register.json login.json me.json users.json userbyid.json regions.json regioncreate.json err-login.json err-notoken.json; do
  if diff -q /tmp/parity-node/$f /tmp/parity-go/$f >/dev/null 2>&1; then echo "IGUAL: $f"
  else echo "--- DIFF: $f"; diff /tmp/parity-node/$f /tmp/parity-go/$f | head -n 12; fi
done
for f in err-login.code err-notoken.code; do
  a=$(cat /tmp/parity-node/$f); b=$(cat /tmp/parity-go/$f)
  [ "$a" == "$b" ] && echo "IGUAL: $f ($a)" || echo "DIFF: $f node=$a go=$b"
done
