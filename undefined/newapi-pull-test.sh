set -uo pipefail
echo "--- kill stalled build ---"
PID=$(cat /root/newapi-build.pid 2>/dev/null || true)
if [ -n "${PID:-}" ] && kill -0 "$PID" 2>/dev/null; then kill "$PID"; echo "killed $PID"; else echo "no build process"; fi
sleep 3
ps aux | grep '[d]ocker build' | head -3 || true

echo "--- tag pulls (90s timeout each) ---"
for ref in oven/bun:1 golang:1.26.6-alpine debian:bookworm-slim; do
  start=$(date +%s)
  if timeout 90 docker pull -q "$ref" >/dev/null 2>&1; then
    echo "OK   $ref  $(( $(date +%s) - start ))s"
  else
    echo "FAIL $ref  $(( $(date +%s) - start ))s"
  fi
done

echo "--- digest pulls (what the Dockerfile pins, 90s timeout each) ---"
for ref in oven/bun@sha256:0733e50325078969732ebe3b15ce4c4be5082f18c4ac1a0f0ca4839c2e4e42a7 golang@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 debian@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a; do
  start=$(date +%s)
  if timeout 90 docker pull -q "$ref" >/dev/null 2>&1; then
    echo "OK   ${ref:0:32}  $(( $(date +%s) - start ))s"
  else
    echo "FAIL ${ref:0:32}  $(( $(date +%s) - start ))s"
  fi
done

echo "--- local bases ---"
docker images --format '{{.Repository}}:{{.Tag}} {{.Size}}' | grep -E '^(oven/bun|golang|debian)' || echo none
