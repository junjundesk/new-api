# New API

Copyright (c) QuantumNous.

Licensed under the [GNU Affero General Public License v3.0](./LICENSE).

Original project: <https://github.com/QuantumNous/new-api>

## Docker

镜像发布在 GitHub Container Registry：

```bash
docker pull ghcr.io/junjundesk/new-api:latest
```

简单运行（默认 SQLite，数据保存在 `./data`）：

```bash
docker run -d --name new-api -p 3000:3000 -v ./data:/data \
  ghcr.io/junjundesk/new-api:latest
```

如果服务通过 Nginx、1Panel 或 Cloudflare 反向代理访问，请同时配置可信代理和客户端 IP 请求头，避免使用日志记录为代理服务器 IP：

```bash
docker run -d --name new-api -p 3000:3000 -v ./data:/data \
  -e TRUSTED_PROXIES="172.18.0.1,38.76.209.3" \
  -e TRUSTED_PROXY_HEADERS="CF-Connecting-IP,X-Forwarded-For,X-Real-IP" \
  ghcr.io/junjundesk/new-api:latest
```

请将 `TRUSTED_PROXIES` 替换为实际反向代理的 TCP 地址或 CIDR 网段。使用仓库内 Compose 编排时，可在项目根目录 `.env` 中填写同名变量：

```dotenv
TRUSTED_PROXIES=172.18.0.1,38.76.209.3
TRUSTED_PROXY_HEADERS=CF-Connecting-IP,X-Forwarded-For,X-Real-IP
```

或直接使用仓库内的 compose 编排（含 Redis、PostgreSQL，默认监听 `http://localhost:3000`）：

```bash
docker compose up -d
```

生产环境部署前请修改 `docker-compose.yml` 中的默认密码，并按需配置 `SQL_DSN`、`REDIS_CONN_STRING`、`SESSION_SECRET` 等环境变量。
