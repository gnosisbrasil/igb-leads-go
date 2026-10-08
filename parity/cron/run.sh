#!/bin/bash
# Probe de crons (descartável): seed + métodos do scheduler + SMTP/meow mocks.
set -u
PG=/usr/local/bin
DIR="$(cd "$(dirname "$0")" && pwd)"
$PG/pg_ctl -D /tmp/pgscratch -l /tmp/pgscratch.log -o "-p 5544 -k /tmp" start >/dev/null 2>&1
$PG/psql -h /tmp -p 5544 -U postgres -c "DROP DATABASE IF EXISTS leads_cron;" -c "CREATE DATABASE leads_cron;" >/dev/null
$PG/psql -h /tmp -p 5544 -U postgres -d leads_cron -q -f /tmp/leads-schema.sql >/dev/null 2>&1
H='$2b$10$llUC6LXcpiNWl.gyMDfAHOHV5Fi7VlVrxJStckKN238DubmH9TFJW'
TOMORROW=$(python3 -c "import datetime;print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1)).strftime('%Y-%m-%dT19:00:00Z'))")
TODAY=$(python3 -c "import datetime;print(datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%dT19:00:00Z'))")
IN3D=$(python3 -c "import datetime;print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=3)).strftime('%Y-%m-%dT19:00:00Z'))")
YESTERDAY=$(python3 -c "import datetime;print((datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=2)).strftime('%Y-%m-%dT10:00:00Z'))")
IN70H=$(python3 -c "import datetime;print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(hours=70)).strftime('%Y-%m-%dT%H:%M:%SZ'))")
$PG/psql -h /tmp -p 5544 -U postgres -d leads_cron -q <<SQL >/dev/null
INSERT INTO regions (id, name, code, country, is_active, created_at, updated_at)
VALUES ('11111111-1111-1111-1111-111111111111','SP','SP','Brasil',true,now(),now());
INSERT INTO users (id, email, password_hash, first_name, last_name, whatsapp, role, status, created_at, updated_at)
VALUES ('44444444-4444-4444-4444-444444444444','u@t.co','$H','A','B','11977777777','user','active',now(),now());
INSERT INTO campaigns (id, title, status, platform, user_id, auto_relationship, event_date, created_at, updated_at) VALUES
('c0000000-0000-0000-0000-000000000001','Ev Amanha','in_progress','meta_ads','44444444-4444-4444-4444-444444444444',true,'$TOMORROW',now(),now()),
('c0000000-0000-0000-0000-000000000002','Ev Hoje','in_progress','meta_ads','44444444-4444-4444-4444-444444444444',true,'$TODAY',now(),now()),
('c0000000-0000-0000-0000-000000000003','Ev 3d','in_progress','meta_ads','44444444-4444-4444-4444-444444444444',false,'$IN3D',now(),now()),
('c0000000-0000-0000-0000-000000000004','Custo','completed','meta_ads','44444444-4444-4444-4444-444444444444',false,'$TODAY',now(),now()),
('c0000000-0000-0000-0000-000000000005','Ev 70h','in_progress','meta_ads','44444444-4444-4444-4444-444444444444',false,'$IN70H',now(),now());
INSERT INTO forms (id, campaign_id, slug, title, is_active, custom_fields, created_at, updated_at, public_token) VALUES
('f0000000-0000-0000-0000-000000000001','c0000000-0000-0000-0000-000000000001','s1','F1',true,'[]',now(),now(),'a0000000-0000-0000-0000-000000000001'),
('f0000000-0000-0000-0000-000000000002','c0000000-0000-0000-0000-000000000002','s2','F2',true,'[]',now(),now(),'a0000000-0000-0000-0000-000000000002'),
('f0000000-0000-0000-0000-000000000004','c0000000-0000-0000-0000-000000000004','s4','F4',true,'[]',now(),now(),'a0000000-0000-0000-0000-000000000004');
INSERT INTO message_templates (id, campaign_id, key, label, content, sort_order, is_active, created_at, updated_at, phase, is_editable) VALUES
('d0000000-0000-0000-0000-000000000001','c0000000-0000-0000-0000-000000000001','pedido_confirmacao','P','Olá {{inscrito_nome}} confirme {{titulo_campanha}}',1,true,now(),now(),'new',true),
('d0000000-0000-0000-0000-000000000002','c0000000-0000-0000-0000-000000000001','lembrete_evento','L','Lembrete {{titulo_campanha}} {{data_evento}}',2,true,now(),now(),'general',true),
('d0000000-0000-0000-0000-000000000003','c0000000-0000-0000-0000-000000000002','envio_voucher','V','Voucher {{codigo_checkin}} {{url_qrcode_imagem}}',1,true,now(),now(),'confirmed',true);
INSERT INTO leads (id, form_id, first_name, last_name, whatsapp, email, checkin_code, status, confirmation_sent_at, confirmed_at, created_at, updated_at) VALUES
('b0000000-0000-0000-0000-000000000001','f0000000-0000-0000-0000-000000000001','Rem','Inder','11911112222','rem@teste.com','CODE11111111','confirmed','$YESTERDAY','$YESTERDAY','$YESTERDAY','$YESTERDAY'),
('b0000000-0000-0000-0000-000000000002','f0000000-0000-0000-0000-000000000002','Vou','Cher','11933334444','vouch@teste.com','CODE22222222','confirmed','$YESTERDAY','$YESTERDAY','$YESTERDAY','$YESTERDAY'),
('b0000000-0000-0000-0000-000000000003','f0000000-0000-0000-0000-000000000001','Fol','Low','11955556666','fol@teste.com','CODE33333333','new','$YESTERDAY',NULL,'$YESTERDAY','$YESTERDAY'),
('b0000000-0000-0000-0000-000000000004','f0000000-0000-0000-0000-000000000004','C1','X','11900001111','c1@t.co','CODE44444444','new',NULL,NULL,now(),now()),
('b0000000-0000-0000-0000-000000000005','f0000000-0000-0000-0000-000000000004','C2','X','11900002222','c2@t.co','CODE55555555','new',NULL,NULL,now(),now()),
('b0000000-0000-0000-0000-000000000006','f0000000-0000-0000-0000-000000000004','C3','X','11900003333','c3@t.co','CODE66666666','new',NULL,NULL,now(),now());
INSERT INTO email_schedules (id, campaign_id, email_type, scheduled_for, recipient_type, status, created_at, updated_at)
VALUES ('e0000000-0000-0000-0000-000000000001','c0000000-0000-0000-0000-000000000001','event_reminder_1d',now() - interval '1 hour','user','pending',now(),now());
SQL
rm -f /tmp/mock-meow.log /tmp/mock-smtp.log
python3 "$DIR/../mocks/mock-meow-smtp.py" >/dev/null 2>&1 &
MOCKPID=$!
sleep 1
cd "$DIR" && go mod tidy >/dev/null 2>&1
DATABASE_URL="postgres://postgres@/leads_cron?host=/tmp&port=5544&sslmode=disable" \
  JWT_SECRET="segredo-de-teste-com-32-chars-ok!" FRONTEND_URL=http://localhost:5173 API_URL=http://localhost:3551 \
  MEOW_URL=http://127.0.0.1:3592 MEOW_API_KEY=test-key MEOW_SESSION=default \
  SMTP_HOST=127.0.0.1 SMTP_PORT=3525 SMTP_USER=u SMTP_PASSWORD=p SMTP_FROM=n@t.co \
  CID_COST=c0000000-0000-0000-0000-000000000004 SCHED_ID=e0000000-0000-0000-0000-000000000001 \
  go run . 2>&1 | grep -v "^20" | head -n 20
kill $MOCKPID 2>/dev/null
