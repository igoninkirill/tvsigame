# syntax=docker/dockerfile:1

# --- Frontend build ---
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# --- Backend build ---
FROM golang:1.23-alpine AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# --- Runtime ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /srv
COPY --from=backend /server /srv/server
COPY --from=frontend /app/frontend/dist /srv/static
ENV STATIC_DIR=/srv/static
EXPOSE 8080
ENTRYPOINT ["/srv/server"]
