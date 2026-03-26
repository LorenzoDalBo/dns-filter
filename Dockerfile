# ===== Stage 1: Build frontend =====
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# ===== Stage 2: Build Go binary =====
FROM golang:1.24-alpine AS backend
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy frontend build into the embed location
COPY --from=frontend /app/web/dist ./internal/api/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /dnsfilter ./cmd/dnsfilter

# ===== Stage 3: Production image =====
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /dnsfilter .
COPY configs/dnsfilter.yaml ./configs/
COPY blocklist.txt allowlist.txt ./

EXPOSE 53/udp 53/tcp 80 8081

ENTRYPOINT ["./dnsfilter"]
CMD ["-config", "configs/dnsfilter.yaml"]