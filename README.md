# 企业内网聊天

面向单一公司的 Windows 内网聊天系统。第一期目标是建立可运行的聊天基础设施：用户注册与管理员审核、登录、会话/消息接口、WebSocket 实时消息、混合文件存储协议，以及 Electron + Vue 客户端骨架。

## 开发

```powershell
docker compose up -d postgres redis minio
go run ./server
```

## Windows 安装包

使用 Inno Setup 编译 [installer/Intrachat.iss](installer/Intrachat.iss)。当前生成的安装包位于 `release/Intrachat-Setup-0.1.0.exe`。安装后首次登录时在客户端填写聊天服务器地址；服务器端仍需先按 `.env.example` 配置并启动 Docker 服务。

默认服务地址：`http://127.0.0.1:8080`。首次启动管理员由 `INITIAL_ADMIN_USERNAME` 与 `INITIAL_ADMIN_PASSWORD` 初始化。

## 文档

- [技术设计](docs/TECHNICAL_DESIGN.md)
- [API 约定](docs/API.md)
