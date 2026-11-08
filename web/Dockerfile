# Build context is the repository root, not ./web.
#
# The legal documents live in internal/api/handler/legal/*.md — the copies the
# Go API embeds and serves at /api/legal/{slug}. The frontend compiles the same
# files into the bundle and prerenders them to static HTML, so the build needs
# them at the repo-relative path `../../../../internal/...` from
# `src/pages/legal`. Keeping one copy in the repository is what stops the
# rendered policy and the served policy from drifting apart.
FROM node:22-alpine AS builder
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --legacy-peer-deps
COPY web/ ./
COPY internal/api/handler/legal/ /app/internal/api/handler/legal/
# Prerendered pages embed this as <link rel="canonical"> and og:url. Without it
# the pages still ship — they just omit those tags rather than guess a domain.
ARG PUBLIC_ORIGIN=""
ENV PUBLIC_ORIGIN=${PUBLIC_ORIGIN}
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/web/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
