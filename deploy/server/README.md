# 聊天服务器部署包

此目录是部署在 Windows 10/Windows Server（Docker Desktop）的服务器端，不是客户端安装包。

## 部署

1. 将整个项目目录复制到服务器，例如 `C:\\IntrachatServer`。
2. 安装并启动 Docker Desktop，启用 Linux containers。
3. 复制配置：`Copy-Item .env.example .env`，编辑 `.env` 中的数据库、MinIO、JWT 和管理员密码。
4. 执行 `powershell -ExecutionPolicy Bypass -File .\\deploy\\server\\install.ps1`。
5. 客户端服务器地址填写 `http://服务器固定IP:8080`。

## 服务

- `server`：Go API + WebSocket，唯一对客户端暴露的端口 8080
- `postgres`：内部数据库，不映射宿主端口
- `redis`：内部缓存
- `minio`：内部文件存储；当前后端仍在接入文件上传适配器

## 管理

```powershell
docker compose ps
docker compose logs -f server
docker compose down
```

首次启动会根据 `INITIAL_ADMIN_USERNAME` / `INITIAL_ADMIN_PASSWORD` 创建管理员。不要把 `.env` 提交到 Git。

