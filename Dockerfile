# Build stage - Frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
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
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o ocm ./cmd/ocm

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates sqlite-libs
WORKDIR /app
COPY --from=backend /app/ocm .

# Create data directory
RUN mkdir -p /data

EXPOSE 8080 9999

ENTRYPOINT ["/app/ocm"]
CMD ["--db", "/data/ocm.db", "--agent-addr", ":9999", "--admin-addr", ":8080"]
