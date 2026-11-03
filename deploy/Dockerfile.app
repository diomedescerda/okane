# Builds the Okane API binary with the web front end embedded, for hosts that
# run a single container (e.g. Render free tier). Build from the repo root:
#   docker build -f deploy/Dockerfile.app -t okane-app .
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /src/web/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -tags no_sqlite3,no_libsql,no_mysql,no_duckdb,no_vertica,no_clickhouse \
    -o /out/okane ./cmd/okane

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/okane /okane
# Timezone data is available: cmd/okane imports time/tzdata.
EXPOSE 8080
ENTRYPOINT ["/okane"]
CMD ["serve"]
