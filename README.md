# 企业内网聊天

面向单一公司的 Windows 内网聊天系统。项目由两部分组成：部署在公司服务器上的 Go + Docker 聊天服务，以及员工电脑上的 Electron Windows 客户端。客户端不能单独工作，必须先启动服务器。

当前仓库同时包含服务器源码、客户端源码、技术文档和安装脚本。服务器不是另一个客户端 EXE，而是通过 Docker Compose 运行的后台服务。

## 目录

```text
server/                 Go 后端和数据库迁移
client/                 Electron + Vue 客户端
deploy/server/          Windows Docker 服务器部署脚本
installer/              客户端 Inno Setup 安装脚本
docs/                   技术设计和 API 约定
```

## 开发

```powershell
Copy-Item .env.example .env
# 编辑 .env，替换所有 replace-with-* 的值
docker compose up -d --build
```

服务端启动后检查：`http://服务器IP:8080/health`。客户端登录页的服务器地址填写 `http://服务器固定IP:8080`。

也可以直接执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\deploy\server\install.ps1
```

服务器部署说明见 [deploy/server/README.md](deploy/server/README.md)。

## Windows 安装包

使用 Inno Setup 编译 [installer/Intrachat.iss](installer/Intrachat.iss)。客户端安装包位于 `release/Intrachat-Setup-0.1.0.exe`。安装客户端后，必须先部署服务器，再在登录页填写服务器地址。

本地开发默认服务地址：`http://127.0.0.1:8080`。首次启动管理员由 `INITIAL_ADMIN_USERNAME` 与 `INITIAL_ADMIN_PASSWORD` 初始化。生产环境请修改 JWT、数据库、MinIO 密码，不能使用示例值。

## 文档

- [技术设计](docs/TECHNICAL_DESIGN.md)
- [API 约定](docs/API.md)
