FROM node:24-alpine AS web

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/raidy ./cmd/raidy

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 raidy \
    && adduser -S -D -H -u 10001 -G raidy raidy
WORKDIR /app
COPY --from=backend /out/raidy /app/raidy
COPY --from=web /src/web/dist /app/web

ENV HTTP_ADDR=:8080 \
    WEB_DIST_DIR=/app/web
EXPOSE 8080
USER 10001:10001
ENTRYPOINT ["/app/raidy"]
