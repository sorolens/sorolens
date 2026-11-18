# Build stage: full toolchain + type-checking.
FROM node:20-alpine AS build
WORKDIR /app
RUN apk add --no-cache python3 make g++
COPY package.json ./
RUN npm install
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# Runtime stage: production dependencies only.
FROM node:20-alpine
WORKDIR /app
RUN apk add --no-cache python3 make g++
COPY package.json ./
RUN npm install --omit=dev && apk del python3 make g++
COPY --from=build /app/dist ./dist
# Litestream streams the SQLite link database to S3-compatible storage so
# links survive redeploys on hosts with no persistent disk.
COPY --from=litestream/litestream:0.3.13 /usr/local/bin/litestream /usr/local/bin/litestream
COPY litestream.yml /etc/litestream.yml
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
ENV NODE_ENV=production
ENV PORT=8080
EXPOSE 8080
VOLUME ["/data"]
ENV DATABASE_PATH=/data/mappings.db
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
