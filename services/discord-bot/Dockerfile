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
ENV NODE_ENV=production
ENV PORT=8080
EXPOSE 8080
VOLUME ["/data"]
ENV DATABASE_PATH=/data/mappings.db
CMD ["node", "dist/index.js"]
