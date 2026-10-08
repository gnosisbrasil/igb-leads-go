#!/bin/bash
# Recria o container leads-go-backend de forma idêntica (pós-virada Node->Go, 2026-10-07).
# Uso: ./leads-go-backend-run.sh   (remove e recria; imagem leads-go-backend:prod)
set -u
H="itc4t27vg7wy1t7uht0w4pco.145.223.95.103.sslip.io"
docker stop leads-go-backend 2>/dev/null; docker rm leads-go-backend 2>/dev/null
docker run -d --name leads-go-backend --network coolify --env-file /root/leads-go-backend.env \
  -v leads-go-uploads:/app/uploads --restart unless-stopped \
  -l traefik.enable=true \
  -l traefik.http.middlewares.gzip.compress=true \
  -l traefik.http.middlewares.redirect-to-https.redirectscheme.scheme=https \
  -l traefik.http.routers.http-0-leadsgo.entrypoints=http \
  -l traefik.http.routers.http-0-leadsgo.middlewares=redirect-to-https \
  -l "traefik.http.routers.http-0-leadsgo.rule=Host(\`$H\`) && PathPrefix(\`/\`)" \
  -l traefik.http.routers.http-0-leadsgo.service=http-0-leadsgo \
  -l traefik.http.routers.https-0-leadsgo.entrypoints=https \
  -l traefik.http.routers.https-0-leadsgo.middlewares=gzip \
  -l "traefik.http.routers.https-0-leadsgo.rule=Host(\`$H\`) && PathPrefix(\`/\`)" \
  -l traefik.http.routers.https-0-leadsgo.service=https-0-leadsgo \
  -l traefik.http.routers.https-0-leadsgo.tls=true \
  -l traefik.http.routers.https-0-leadsgo.tls.certresolver=letsencrypt \
  -l traefik.http.routers.http-1-leadsgo.entrypoints=http \
  -l "traefik.http.routers.http-1-leadsgo.rule=Host(\`api-leads.gnosisbrasil.com\`) && PathPrefix(\`/\`)" \
  -l traefik.http.routers.http-1-leadsgo.service=http-0-leadsgo \
  -l traefik.http.services.http-0-leadsgo.loadbalancer.server.port=3001 \
  -l traefik.http.services.https-0-leadsgo.loadbalancer.server.port=3001 \
  -l leads.backend=go \
  leads-go-backend:prod
