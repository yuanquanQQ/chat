# API 约定（v1）

统一前缀：`/api/v1`。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/auth/register` | 注册，状态为 pending |
| POST | `/auth/login` | 登录并返回 token |
| GET | `/admin/users/pending` | 管理员查看待审核用户 |
| POST | `/admin/users/{id}/approve` | 审核通过 |
| GET | `/conversations` | 会话列表 |
| POST | `/conversations` | 创建单聊或群聊 |
| GET | `/conversations/{id}/messages?before=&limit=` | 倒序分页消息 |
| POST | `/conversations/{id}/messages` | 发送消息 |
| POST | `/messages/{id}/read` | 标记已读 |
| POST | `/messages/{id}/retract` | 5 分钟内撤回 |
| GET | `/ws` | WebSocket 实时事件 |
| POST | `/files/uploads` | 小文件或转存服务器 |
| POST | `/files/prepare` | 决定服务器/客户端存储并登记文件 |

WebSocket 事件使用 `{type, data}`，第一期事件：`message.created`、`message.retracted`、`message.read`、`presence.changed`。
