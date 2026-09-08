# 企业内网聊天技术设计（v0.2）

## 目标

为约 50 人的单一公司提供 Windows 客户端内网聊天。服务器固定在 Windows 10 + Docker，客户端使用 Electron。系统支持单聊、群聊、文本/图片/视频/文件消息、分页历史、已读、撤回、搜索，并为未来 AI 中枢和 VPN 访问保留稳定接口。

客户端和服务器是两个独立交付物：客户端安装包只包含桌面 UI；服务器必须在固定内网 IP 的 Windows Docker 主机上运行。客户端登录前需要知道服务器地址。

## 模块与接口

后端按深模块组织：调用者只依赖少量 HTTP/WebSocket 接口，认证、权限、分页、撤回窗口和存储策略藏在模块实现中。

- `auth`：注册、审核、登录、最多 3 台设备、密码重置。
- `conversation`：单聊/群聊、成员和群角色。
- `message`：消息创建、分页、搜索、已读、5 分钟撤回。
- `realtime`：WebSocket 事件分发。
- `file`：小文件服务器存储；大文件客户端存储；用户可显式转存服务器。
- `admin`：用户审核、部门变更、审计日志。
- `ai`：仅预留指定会话的转发接口，第一期不启用业务逻辑。

## 部署

客户端 -> Go API/WebSocket（后续可在前面加入 Nginx）。PostgreSQL 保存业务数据，Redis 保存短期会话/在线状态，MinIO 保存服务器文件。当前 Docker Compose 对外只暴露聊天服务 8080；数据库、Redis、MinIO 管理口仅 Docker 网络可见。正式 HTTPS 和 VPN 接入属于后续运维阶段。

服务器部署入口是 `deploy/server/install.ps1`：它校验 `.env`、构建 Go 镜像、启动 PostgreSQL/Redis/MinIO 和聊天服务。客户端安装入口是 `installer/Intrachat.iss`。

## 混合文件策略

默认阈值为 100MB，可配置。小于阈值的文件上传 MinIO；大于阈值的文件由发送者客户端提供内网直连下载，服务器只保存元数据和在线状态。发送者离线时显示“文件暂时不可用”。上传时可选择“长期保存到服务器”，将大文件转存 MinIO。可执行文件只作为数据传输，不自动打开或执行。

## 数据保护

密码使用 bcrypt；访问令牌为短期 JWT；WebSocket 连接必须先认证。管理员查看聊天内容必须是指定会话授权动作，并写入审计日志。正式部署启用 HTTPS；初期可使用固定内网 IP。

## 数据库核心表

`users`、`departments`、`department_change_requests`、`user_sessions`、`conversations`、`conversation_members`、`messages`、`message_reads`、`files`、`audit_logs`、`system_settings`。

## 演进顺序

1. 账号/审核/登录与健康检查。
2. 会话、消息、WebSocket 与分页。
3. 文件上传、预览元数据和客户端直连协议。
4. Electron 客户端完整聊天界面。
5. 备份恢复、HTTPS、AI 指定会话转发、VPN 运维。

## 当前交付边界

已交付的是可启动的基础纵向切片：认证、审核、会话/消息 REST、WebSocket、数据库迁移、客户端登录和基础聊天界面。文件接口目前完成存储模式登记和 10GB 校验，真实 MinIO 分片上传、客户端大文件直连、完整管理员页面及消息高级能力仍按演进顺序实现，不能将当前客户端安装包理解为功能完整的微信替代品。
