set -uo pipefail
for m in docker.1panel.live hub.rat.dev dockerpull.org docker.nju.edu.cn docker.1ms.run docker.m.daocloud.io; do
  start=$(date +%s)
  if timeout 45 docker pull -q ${m}/library/debian:bookworm-slim >/dev/null 2>&1; then
    echo "OK   $m  $(( $(date +%s) - start ))s"
  else
    echo "FAIL $m  $(( $(date +%s) - start ))s"
  fi
done
docker images --format '{{.Repository}}:{{.Tag}}' | grep debian || echo 'no debian image'
