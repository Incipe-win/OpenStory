# ============================================================
# Stage 1: Build Next.js frontend
# ============================================================
FROM node:22-alpine AS builder

ARG NEXT_PUBLIC_API_URL
ARG HTTP_PROXY
ARG HTTPS_PROXY
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL:-}

# Use Aliyun Alpine mirror for any apk calls
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories

WORKDIR /app

# Configure npm proxy if set
RUN if [ -n "$HTTP_PROXY" ]; then npm config set proxy "$HTTP_PROXY"; fi && \
    if [ -n "$HTTPS_PROXY" ]; then npm config set https-proxy "$HTTPS_PROXY"; fi

# Install dependencies first (cache layer)
COPY package.json package-lock.json* ./
RUN npm ci

# Copy source and build
COPY . .
RUN mkdir -p public && npm run build

# ============================================================
# Stage 2: Production runtime
# ============================================================
FROM node:22-alpine AS runtime

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk add --no-cache tini

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Copy standalone output
COPY --from=builder /app/.next/standalone ./

# Copy static files (must be relative to standalone root)
COPY --from=builder /app/.next/static ./.next/static

# Copy public assets
COPY --from=builder /app/public ./public

EXPOSE 3000

ENV HOSTNAME=0.0.0.0
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["node", "server.js"]
