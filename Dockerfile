# Build stage - Frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && corepack prepare pnpm@latest --activate && pnpm install --frozen-lockfile || npm ci
COPY web/ ./
RUN npm run build

# Build stage - Backend
FROM golang:1.22-alpine AS backend
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/build ./internal/web/build
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o ocm .

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates sqlite-libs
WORKDIR /app
COPY --from=backend /app/ocm .

# Create directories
RUN mkdir -p /data /root/.ocm

EXPOSE 8080 9999

ENTRYPOINT ["/app/ocm"]
CMD ["serve", "--db", "/data/ocm.db", "--agent-addr", ":9999", "--admin-addr", ":8080"]
