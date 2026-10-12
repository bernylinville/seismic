#!/usr/bin/env sh
# Usage: ./docker/wait-ready.sh es8|es7
# Polls the cluster root endpoint until it returns HTTP 200 or times out (300s).

set -u

SERVICE="${1:-}"
TIMEOUT=300

case "$SERVICE" in
  es8)
    PORT=9200
    AUTH="-u elastic:seismic8test"
    ;;
  es7)
    PORT=9210
    AUTH=""
    ;;
  *)
    echo "usage: $0 es8|es7" >&2
    exit 2
    ;;
esac

URL="http://127.0.0.1:${PORT}/"

i=0
while [ "$i" -lt "$TIMEOUT" ]; do
  # shellcheck disable=SC2086
  if curl -s -o /dev/null -w '%{http_code}' --max-time 5 $AUTH "$URL" | grep -q '^200$'; then
    echo "$SERVICE ready"
    # shellcheck disable=SC2086
    curl -s $AUTH "$URL" | grep -E '"cluster_name"|"version"'
    exit 0
  fi
  sleep 2
  i=$((i + 2))
done

echo "$SERVICE not ready after ${TIMEOUT}s" >&2
exit 1
