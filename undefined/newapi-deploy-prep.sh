set -euo pipefail
TS=$(date +%Y%m%d-%H%M%S)
echo "TS=$TS"
docker inspect ningmeng > /root/newapi-backup-ningmeng-$TS.json
echo "snapshot saved: /root/newapi-backup-ningmeng-$TS.json"
docker image tag ghcr.io/junjundesk/new-api:latest ghcr.io/junjundesk/new-api:rollback-$TS
echo "rollback tag created: rollback-$TS"
docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}' | grep new-api
echo "--- clone fork main ---"
rm -rf /root/new-api-build
git clone --depth 1 --branch main https://github.com/junjundesk/new-api.git /root/new-api-build 2>&1 | tail -3
echo "HEAD=$(git -C /root/new-api-build rev-parse HEAD)"
echo "$TS" > /root/newapi-deploy-ts.txt
