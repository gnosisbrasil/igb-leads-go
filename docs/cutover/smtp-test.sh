#!/bin/bash
# Teste SMTP: prova relay+credenciais (sem enviar nada) e depois a fiação Go
# ponta a ponta (1 e-mail real de reset para a caixa do operador).
# Produção não é alterada: container scratch + usuário temporário removidos.
set -u
PASS=0; FAIL=0
check() { if [ "$2" == "$3" ]; then PASS=$((PASS+1)); else FAIL=$((FAIL+1)); echo "FALHOU: $1 -- esperado [$2] obtido [$3]"; fi }

# --- 1. credencial do app cursos (só na VPS, nunca impressa) ---
export SMTPPASS=$(docker inspect h1poxl42amhlu4qgvi8fwiwj-210912858735 --format "{{range .Config.Env}}{{println .}}{{end}}" | grep ^NUXT_SMTP_PASS= | cut -d= -f2-)
[ -n "$SMTPPASS" ] && echo "PASS cred-ok" || { echo "FALHOU: sem senha cursos"; exit 1; }

# --- 2. AUTH + RCPT sem DATA (zero e-mail enviado) ---
python3 - <<'PYEOF'
import smtplib, os
s = smtplib.SMTP("smtp.hostinger.com", 587, timeout=20)
print("banner:", s.noop()[0])
print("starttls:", s.starttls()[0])
print("login:", s.login("suporte@cursos.gnosisbrasil.com", os.environ["SMTPPASS"])[0])
print("mail:", s.mail("suporte@cursos.gnosisbrasil.com")[0])
print("rcpt:", s.rcpt("thiagokx2011@gmail.com")[0])
s.rset(); s.quit()
print("relay-ok")
PYEOF
check "relay aceita" "relay-ok" "$(python3 - <<'PYEOF' 2>/dev/null | tail -n 1
import smtplib, os
try:
    s = smtplib.SMTP("smtp.hostinger.com", 587, timeout=20)
    s.starttls(); s.login("suporte@cursos.gnosisbrasil.com", os.environ["SMTPPASS"])
    s.mail("suporte@cursos.gnosisbrasil.com"); s.rcpt("thiagokx2011@gmail.com")
    s.rset(); s.quit(); print("relay-ok")
except Exception as e:
    print("FALHA")
PYEOF
)"

# --- 3. scratch Go com SMTP ---
grep -v "^SMTP_\|^UPLOAD_DIR=" /root/leads-go-backend.env > /root/leads-go-smtptest.env
cat >> /root/leads-go-smtptest.env <<'EOF'
SMTP_HOST=smtp.hostinger.com
SMTP_PORT=587
SMTP_SECURE=false
SMTP_USER=suporte@cursos.gnosisbrasil.com
SMTP_FROM=suporte@cursos.gnosisbrasil.com
SMTP_FROM_NAME=Sistema de Leads - Gnosis Brasil
UPLOAD_DIR=/app/uploads
EOF
echo "SMTP_PASSWORD=$SMTPPASS" >> /root/leads-go-smtptest.env
chmod 600 /root/leads-go-smtptest.env
unset SMTPPASS
docker run -d --name leads-go-smtptest --network coolify --env-file /root/leads-go-smtptest.env \
  -p 127.0.0.1:3102:3001 --restart no leads-go-backend:prod >/dev/null
sleep 4
check "sem aviso smtp" "0" "$(docker logs leads-go-smtptest 2>&1 | grep -c 'SMTP incompleto')"

# --- 4. reset de senha de verdade (1 e-mail) ---
H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'
/tmp/dbexec.sh "INSERT INTO users (id,email,password_hash,first_name,last_name,whatsapp,role,status,created_at,updated_at) VALUES ('99999999-9999-9999-9999-999999999999','thiagokx2011@gmail.com','$H','Teste','SMTP','11900000000','user','active',now(),now())" >/dev/null
CODE=$(curl -s -o /tmp/smtp-resp.json -w "%{http_code}" -X POST http://127.0.0.1:3102/api/auth/forgot-password -H 'Content-Type: application/json' -d '{"email":"thiagokx2011@gmail.com"}' --max-time 30)
check "forgot 200" "200" "$CODE"
check "log sem erro" "0" "$(docker logs leads-go-smtptest 2>&1 | grep -c 'Erro no forgot password')"
TOKSET=$(/tmp/dbexec.sh "SELECT count(*) FROM users WHERE email='thiagokx2011@gmail.com' AND reset_password_token IS NOT NULL;")
check "token gerado" "1" "$TOKSET"
/tmp/dbexec.sh "DELETE FROM users WHERE email='thiagokx2011@gmail.com';" >/dev/null
check "limpeza" "0" "$(/tmp/dbexec.sh "SELECT count(*) FROM users WHERE email='thiagokx2011@gmail.com';")"

# --- 5. desfaz scratch, prova produção intocada ---
docker stop leads-go-smtptest >/dev/null && docker rm leads-go-smtptest >/dev/null
rm -f /root/leads-go-smtptest.env
check "prod sem smtp" "0" "$(grep -c '^SMTP_' /root/leads-go-backend.env)"
check "prod rodando" "running" "$(docker inspect leads-go-backend --format '{{.State.Status}}')"
check "prod sem restart" "0" "$(docker inspect leads-go-backend --format '{{.RestartCount}}')"

echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" == "0" ] && echo SMTP_TEST_OK || echo SMTP_TEST_FALHOU
