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

或直接使用仓库内的 compose 编排（含 Redis、PostgreSQL，默认监听 `http://localhost:3000`）：

```bash
docker compose up -d
```

生产环境部署前请修改 `docker-compose.yml` 中的默认密码，并按需配置 `SQL_DSN`、`REDIS_CONN_STRING`、`SESSION_SECRET` 等环境变量。