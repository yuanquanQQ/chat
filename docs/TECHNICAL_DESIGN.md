# 企业内网聊天技术设计（v1.0）

## 1. 交付物和边界

本项目由服务器和客户端两个交付物组成：服务器是项目源码、Docker Compose 和 Windows PowerShell 部署脚本；客户端是 Electron Windows 安装包。客户端没有本地聊天服务器，必须连接服务器固定内网 IP 的 8080 端口。服务器运行在 Windows 10 + Docker Desktop，未来通过 VPN 让外部用户访问同一入口。

## 2. 总体架构

```text
Electron + Vue 客户端
          │ HTTP(S) / WebSocket
          ▼
Go 聊天服务（唯一宿主暴露端口 8080）
    ├── Auth：注册、审核、登录、设备会话
    ├── Conversation：单聊、群聊、成员角色
    ├── Message：发送、分页、搜索、已读、撤回
    ├── Realtime：消息、已读、在线状态事件
    ├── File：混合存储决策、元数据、授权下载
    ├── Admin：账号、部门申请、审计
    └── AI 预留：指定会话 HTTP 转发 seam
          │
          ├── PostgreSQL：持久化业务数据
          ├── Redis：在线状态、短期状态（预留）
          └── MinIO：服务器文件（预留适配器）
```

Nginx/HTTPS 是正式部署的后续层；第一版 Compose 直接暴露 Go 服务，数据库、Redis、MinIO 不暴露宿主端口。

## 3. 模块接口与不变量

外部调用者只依赖 REST 和 WebSocket 接口；认证、权限、消息游标、撤回窗口和文件策略隐藏在模块实现中。

- **Auth**：注册资料进入 pending；管理员审核后才可登录；密码 bcrypt；最多三台设备。
- **Conversation**：所有消息读取和发送前验证成员资格；群成员角色为 owner/admin/member。
- **Message**：消息属于会话和发送者；回复必须引用同会话消息；发送者只能在五分钟内撤回自己的消息。
- **File**：先登记元数据再选择存储模式；两种模式都必须检查会话成员权限。
- **Admin**：部门变更、账号操作和受授权查看动作写入审计日志。

## 4. 混合文件策略

默认阈值为 100MB，可配置。小文件实际内容放 MinIO；大文件默认由发送者客户端提供内网直连下载，发送者离线时不可用；用户可显式转存服务器以保证长期可用。服务器始终保存文件元数据、会话关系、哈希和权限。单文件上限 10GB，默认永久保存。可执行文件只当作数据传输，禁止自动打开/执行。

文件状态：`prepared → uploading → available/failed`；客户端文件在发送者下线时变为 `unavailable`。预览只读处理图片、视频、音频、PDF 和文本；Office 第一版提供下载。

## 5. 数据模型

核心表：`users`、`departments`、`department_change_requests`、`user_sessions`、`conversations`、`conversation_members`、`messages`、`message_reads`、`files`、`audit_logs`、`system_settings`。用户属于部门；会话拥有成员；消息属于会话和发送者；文件元数据属于会话和上传者；已读是消息与用户的关系。

## 6. 安全边界

生产环境必须使用强 `JWT_SECRET`、数据库密码和 MinIO 密码，`.env` 不入 Git。WebSocket 使用 Bearer 凭证或 `bearer` 子协议；Origin 仅允许受信任客户端。正式环境启用 HTTPS。管理员默认不能查看全部聊天，只能在指定会话授权后查看并留下审计记录。

## 7. 部署、备份与演进

服务器入口为 `deploy/server/install.ps1`，它校验 `.env`、构建 Go 镜像并启动四个容器；停止用 `stop.ps1`。客户端入口为 `installer/Intrachat.iss`。后续补充 Nginx HTTPS、SMB/另一台电脑备份、数据库和 MinIO 一键恢复、VPN、真实 MinIO 分片上传、客户端直连文件服务和 AI 指定会话转发。

## 8. 当前实现边界

当前交付已覆盖账号/审核基础链路、会话/消息 REST、WebSocket 服务、迁移、基础客户端界面和文件策略登记；客户端注册/管理员界面、完整群管理、真实文件传输、预览、备份恢复和 AI 转发仍按演进顺序实现。

