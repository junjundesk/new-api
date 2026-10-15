set -euo pipefail
echo "=== 本地 git 远端 ==="
git -C /root/new-api-build remote -v | head -2
echo "=== 服务器构建目录 HEAD ==="
git -C /root/new-api-build log -1 --format='%H%n%an <%ae>%n%s'
echo "=== 与我推送的提交是否一致 ==="
echo "expect: 133e2a64717148bce26b3299c6ae28d14fd06c23"
echo "actual: $(git -C /root/new-api-build rev-parse HEAD)"
echo "=== 与旧目录 /root/new-api 的区别（旧目录是上游）==="
git -C /root/new-api remote -v | head -1
echo "=== 启动构建（后台 + 日志）==="
cd /root/new-api-build
nohup docker build -t ghcr.io/junjundesk/new-api:latest -t ghcr.io/junjundesk/new-api:build-133e2a647 . > /root/newapi-build.log 2>&1 &
echo $! > /root/newapi-build.pid
echo "build pid: $(cat /root/newapi-build.pid)"
