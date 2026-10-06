# ---- baixa o three.js do npm na hora do build ----
# Assim o jogo não depende de CDN: funciona numa LAN sem internet.
FROM node:22-alpine AS vendor
WORKDIR /v
RUN npm pack three@0.170.0 --silent && tar -xzf three-0.170.0.tgz

# ---- nginx: serve a página e repassa /ws para o servidor Go ----
FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY public/ /usr/share/nginx/html/
COPY --from=vendor /v/package/build/three.module.js /usr/share/nginx/html/vendor/three/three.module.js
EXPOSE 80
