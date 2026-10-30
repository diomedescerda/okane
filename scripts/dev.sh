#!/usr/bin/env bash
# Local dev stack orchestrator: postgres + API + web dev server.
#
# Usage: scripts/dev.sh up | down | status | logs [api|web|db]
#
# - up     ensure docker daemon, start postgres, apply migrations, start API + web
# - down   stop API + web, stop postgres (volume/data is kept)
# - status print what is running
# - logs   tail one service's output (default: api)
#
# PID files and logs live in .run/. Ports can be overridden via API_PORT / WEB_PORT.

set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RUN="$ROOT/.run"
API_PORT="${API_PORT:-8080}"
WEB_PORT="${WEB_PORT:-5174}"
API_LOG="$RUN/api.log"
WEB_LOG="$RUN/web.log"
API_PID="$RUN/api.pid"
WEB_PID="$RUN/web.pid"

C_G=$'\033[32m'; C_Y=$'\033[33m'; C_R=$'\033[31m'; C_B=$'\033[36m'; C_N=$'\033[0m'
ok()   { echo "${C_G}✔${C_N} $*"; }
warn() { echo "${C_Y}!${C_N} $*"; }
die()  { echo "${C_R}✘${C_N} $*" >&2; }

mkrun() { [ -d "$RUN" ] || mkdir -p "$RUN"; }

port_up() { curl -s -o /dev/null -m 2 "http://localhost:$1${2-}"; }

pid_alive() {
    local pid; pid="$(cat "$1" 2>/dev/null)" || return 1
    [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

kill_group() {
    local pid; pid="$(cat "$1" 2>/dev/null)" || return 0
    kill -0 "$pid" 2>/dev/null || return 0
    kill -- -"$pid" 2>/dev/null || kill "$pid" 2>/dev/null || true
    rm -f "$1"
}

wait_for() { # wait_for <desc> <tries> <check-cmd...>
    local desc="$1" tries="$2"; shift 2
    local i=0
    while [ "$i" -lt "$tries" ]; do
        if "$@" >/dev/null 2>&1; then ok "$desc"; return 0; fi
        i=$((i + 1)); sleep 1
    done
    die "$desc (gave up after ${tries}s)"
    return 1
}

docker_daemon_ok() { docker compose ps >/dev/null 2>&1; }

ensure_docker() {
    if docker_daemon_ok; then return 0; fi
    warn "docker daemon not running; trying docker-desktop.service"
    if command -v systemctl >/dev/null 2>&1; then
        systemctl --user start docker-desktop.service 2>/dev/null \
            || systemctl start docker-desktop.service 2>/dev/null || true
    fi
    wait_for "docker daemon" 30 docker_daemon_ok || {
        die "start Docker Desktop manually, then re-run 'dev.sh up'"
        return 1
    }
    return 0
}

postgres_state() {
    (cd "$ROOT" && docker compose ps postgres --format '{{.State}}' 2>/dev/null)
}

ensure_postgres() {
    local state; state="$(postgres_state)"
    case "$state" in
        running) ;; # wait for health below, as in every state
        "") warn "creating postgres container"
            (cd "$ROOT" && docker compose up -d postgres) || { die "docker compose up postgres failed"; return 1; }
            ;;
        *) warn "restart postgres container"
            (cd "$ROOT" && docker compose start postgres) || { die "docker compose start postgres failed"; return 1; }
            ;;
    esac
    wait_for "postgres healthy" 30 docker exec okane-postgres-1 pg_isready -q -U okane -d okane || return 1
    return 0
}

# --- api / web lifecycle ------------------------------------------------------

api_start() {
    : > "$API_LOG"
    nohup setsid "$RUN/okane" serve > "$API_LOG" 2>&1 &
    echo $! > "$API_PID"
}

web_start() {
    : > "$WEB_LOG"
    (
        set -u
        cd "$ROOT/web"
        nohup setsid npm run dev -- --port "$WEB_PORT" --strictPort > "$WEB_LOG" 2>&1 &
        echo $! > "$WEB_PID"
    )
}

# --- commands -----------------------------------------------------------------

cmd_up() {
    mkrun
    ensure_docker || return 1
    ensure_postgres || return 1

    # build the server binary (fast, warm cache); no `go run`, so nothing orphans
    go build -o "$RUN/okane" ./cmd/okane

    # migrate: idempotent, keeps a fresh volume usable
    "$RUN/okane" migrate up > "$RUN/migrate.log" 2>&1 || {
        die "migrate failed"; tail -n 10 "$RUN/migrate.log"; return 1
    }

    if port_up "$API_PORT" "/readyz"; then
        warn "API already on :$API_PORT (skipping start)"
    else
        api_start
        wait_for "API ready :$API_PORT" 15 port_up "$API_PORT" "/readyz" || {
            tail -n 15 "$API_LOG"; return 1; }
    fi

    if port_up "$WEB_PORT"; then
        warn "web already on :$WEB_PORT (skipping start)"
    else
        web_start
        wait_for "web ready :$WEB_PORT" 15 port_up "$WEB_PORT" || {
            tail -n 15 "$WEB_LOG"; return 1; }
    fi

    echo
    echo "${C_B}okane dev stack is up${C_N}"
    echo "  web   http://localhost:$WEB_PORT   (demo@okane.dev / demo1234)"
    echo "  api   http://localhost:$API_PORT"
    echo "  logs  $RUN/api.log · $RUN/web.log"
}

cmd_down() {
    mkrun
    kill_group "$WEB_PID"; rm -f "$WEB_PID"
    kill_group "$API_PID"; rm -f "$API_PID"

    # stray processes: cover stack instances started outside this script
    pkill -f 'okane serve' 2>/dev/null && warn "stopped stray API" || true
    pkill -f "vite --port $WEB_PORT" 2>/dev/null && warn "stopped stray vite" || true

    # wait for ports to actually free
    local i=0 frees=1
    while [ "$i" -lt 8 ]; do
        frees=1
        port_up "$API_PORT" "/readyz" && frees=0
        port_up "$WEB_PORT" && frees=0
        [ "$frees" -eq 1 ] && break
        i=$((i + 1)); sleep 1
    done
    if [ "$frees" -eq 0 ]; then
        warn "port :$API_PORT / :$WEB_PORT still busy — something else may not belong to okane"
    fi

    local state; state="$(postgres_state)"
    if [ -n "$state" ]; then
        (cd "$ROOT" && docker compose stop postgres) && ok "postgres stopped (data kept)"
    else
        warn "postgres not managed here (compose not reachable)"
    fi
    echo "stopped"
}

cmd_status() {
    local pg; pg="$(postgres_state)"
    [ "$pg" = "running" ] && ok "postgres running" \
        || { [ -n "$pg" ] && warn "postgres $pg" || warn "postgres unknown/unreachable"; }

    port_up "$API_PORT" "/readyz" && ok "API :$API_PORT up" \
        || warn "API :$API_PORT down"
    port_up "$WEB_PORT" && ok "web :$WEB_PORT up" \
        || warn "web :$WEB_PORT down"
    echo
    echo "web   http://localhost:$WEB_PORT"
    echo "api   http://localhost:$API_PORT/readyz"
}

cmd_logs() {
    case "${1:-api}" in
        api) tail -n 40 -f "$API_LOG" ;;
        web) tail -n 40 -f "$WEB_LOG" ;;
        db)  docker logs -f okane-postgres-1 ;;
        *)   die "unknown service '$1' (api|web|db)"; return 1 ;;
    esac
}

case "${1:-}" in
    up)     shift; cmd_up "$@" ;;
    down)   shift; cmd_down "$@" ;;
    status) shift; cmd_status "$@" ;;
    logs)   shift; cmd_logs "$@" ;;
    *)      sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac
