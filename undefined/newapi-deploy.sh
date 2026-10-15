set -euo pipefail
cd /opt/1panel/apps/new-api/new-api
echo "--- before ---"
docker inspect ningmeng --format 'image={{.Config.Image}} id={{.Image}}'
docker images --format '{{.Repository}}:{{.Tag}} {{.ID}} {{.CreatedSince}}' | grep new-api || true

echo "--- compose pull ---"
docker compose pull new-api

echo "--- compose up -d ---"
docker compose up -d new-api
sleep 10

echo "--- after ---"
docker ps --filter name=ningmeng --format '{{.Names}} | {{.Image}} | {{.Status}}'
docker inspect ningmeng --format 'imageID={{.Image}} health={{.State.Health.Status}}'

echo "--- smoke test (port 3252) ---"
curl -s -o /dev/null -w 'GET /api/status              -> %{http_code}\n' http://localhost:3252/api/status
curl -s -o /dev/null -w 'GET /api/user/activity/offers -> %{http_code}  (401=new code live, 404=old image)\n' http://localhost:3252/api/user/activity/offers
