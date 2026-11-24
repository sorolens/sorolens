# syntax=docker/dockerfile:1
# Build from the repository root.
FROM node:22.22.0-bookworm-slim@sha256:dd9d21971ec4395903fa6143c2b9267d048ae01ca6d3ea96f16cb30df6187d94 AS builder
WORKDIR /src
RUN npm install --global pnpm@11.17.0
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/
COPY packages/typescript-sdk/package.json packages/typescript-sdk/
COPY packages/ui/package.json packages/ui/
COPY packages/xdr/package.json packages/xdr/
RUN pnpm install --filter @sorolens/web... --frozen-lockfile --ignore-scripts

COPY apps/web/ apps/web/
COPY docs/openapi.yaml docs/openapi.yaml
COPY packages/ui/ packages/ui/
COPY packages/xdr/ packages/xdr/
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL SOROLENS_STANDALONE=1 NEXT_TELEMETRY_DISABLED=1
RUN pnpm --filter @sorolens/web build

FROM node:22.22.0-bookworm-slim@sha256:dd9d21971ec4395903fa6143c2b9267d048ae01ca6d3ea96f16cb30df6187d94
ENV NODE_ENV=production HOSTNAME=0.0.0.0 PORT=3000 NEXT_TELEMETRY_DISABLED=1
WORKDIR /app
COPY --from=builder --chown=node:node /src/apps/web/.next/standalone ./
COPY --from=builder --chown=node:node /src/apps/web/.next/static ./apps/web/.next/static
COPY --from=builder --chown=node:node /src/apps/web/public ./apps/web/public
USER node
EXPOSE 3000
HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=3 \
  CMD ["node", "-e", "fetch('http://127.0.0.1:' + process.env.PORT).then(r => { if (!r.ok) process.exitCode = 1 }).catch(() => { process.exitCode = 1 })"]
CMD ["node", "apps/web/server.js"]
