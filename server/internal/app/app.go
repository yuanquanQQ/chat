package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"intrachat/server/internal/config"
	"intrachat/server/internal/store"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type server struct {
	cfg config.Config
	db  *store.Store
	hub *hub
}
type principal struct{ UserID, Role, TokenID string }
type contextKey string

const principalKey contextKey = "principal"

func New(cfg config.Config, db *store.Store) http.Handler {
	s := &server{cfg: cfg, db: db, hub: newHub()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /api/v1/auth/register", s.register)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.Handle("GET /api/v1/admin/users/pending", s.require("admin", http.HandlerFunc(s.pendingUsers)))
	mux.Handle("POST /api/v1/admin/users/{id}/approve", s.require("admin", http.HandlerFunc(s.approveUser)))
	mux.Handle("GET /api/v1/conversations", s.require("", http.HandlerFunc(s.conversations)))
	mux.Handle("POST /api/v1/conversations", s.require("", http.HandlerFunc(s.createConversation)))
	mux.Handle("POST /api/v1/conversations/{id}/members", s.require("", http.HandlerFunc(s.addMembers)))
	mux.Handle("GET /api/v1/conversations/{id}/members", s.require("", http.HandlerFunc(s.conversationMembers)))
	mux.Handle("DELETE /api/v1/conversations/{id}/members/{userId}", s.require("", http.HandlerFunc(s.removeMember)))
	mux.Handle("PATCH /api/v1/conversations/{id}", s.require("", http.HandlerFunc(s.renameConversation)))
	mux.Handle("DELETE /api/v1/conversations/{id}", s.require("", http.HandlerFunc(s.disbandConversation)))
	mux.Handle("POST /api/v1/me/department-request", s.require("", http.HandlerFunc(s.departmentRequest)))
	mux.Handle("GET /api/v1/admin/department-requests", s.require("admin", http.HandlerFunc(s.departmentRequests)))
	mux.Handle("POST /api/v1/admin/department-requests/{id}/approve", s.require("admin", http.HandlerFunc(s.approveDepartmentRequest)))
	mux.Handle("POST /api/v1/admin/department-requests/{id}/reject", s.require("admin", http.HandlerFunc(s.rejectDepartmentRequest)))
	mux.Handle("GET /api/v1/admin/users", s.require("admin", http.HandlerFunc(s.adminUsers)))
	mux.Handle("POST /api/v1/admin/users/{id}/reject", s.require("admin", http.HandlerFunc(s.rejectUser)))
	mux.Handle("POST /api/v1/admin/users/{id}/disable", s.require("admin", http.HandlerFunc(s.disableUser)))
	mux.Handle("POST /api/v1/admin/users/{id}/enable", s.require("admin", http.HandlerFunc(s.enableUser)))
	mux.Handle("POST /api/v1/admin/users/{id}/reset-password", s.require("admin", http.HandlerFunc(s.resetPassword)))
	mux.Handle("GET /api/v1/users", s.require("", http.HandlerFunc(s.users)))
	mux.Handle("GET /api/v1/me", s.require("", http.HandlerFunc(s.me)))
	mux.Handle("GET /api/v1/conversations/{id}/messages", s.require("", http.HandlerFunc(s.messages)))
	mux.Handle("POST /api/v1/conversations/{id}/messages", s.require("", http.HandlerFunc(s.sendMessage)))
	mux.Handle("POST /api/v1/messages/{id}/read", s.require("", http.HandlerFunc(s.readMessage)))
	mux.Handle("POST /api/v1/messages/{id}/retract", s.require("", http.HandlerFunc(s.retractMessage)))
	mux.Handle("POST /api/v1/files/prepare", s.require("", http.HandlerFunc(s.prepareFile)))
	mux.Handle("GET /ws", s.require("", http.HandlerFunc(s.websocket)))
	return recoverAndCORS(mux)
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Pool.Ping(r.Context()); err != nil {
		problem(w, 503, "database unavailable")
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password, RealName, AvatarURL, Department string }
	if !decode(w, r, &in) {
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	in.RealName = strings.TrimSpace(in.RealName)
	in.Department = strings.TrimSpace(in.Department)
	if len(in.Username) < 3 || len(in.Password) < 8 || in.RealName == "" || in.AvatarURL == "" || in.Department == "" {
		problem(w, 400, "用户名至少3位、密码至少8位，真实姓名、头像和部门必填")
		return
	}
	if len([]byte(in.Password)) > 72 {
		problem(w, 400, "密码不能超过 72 字节")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		problem(w, 400, "密码格式无效")
		return
	}
	tx, err := s.db.Pool.Begin(r.Context())
	if err != nil {
		problem(w, 500, "注册失败")
		return
	}
	defer tx.Rollback(r.Context())
	var departmentID string
	err = tx.QueryRow(r.Context(), `INSERT INTO departments(name) VALUES($1) ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, in.Department).Scan(&departmentID)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO users(username,password_hash,real_name,avatar_url,department_id) VALUES($1,$2,$3,$4,$5)`, in.Username, string(hash), in.RealName, in.AvatarURL, departmentID)
	}
	if err != nil {
		if strings.Contains(err.Error(), "users_username_key") {
			problem(w, 409, "用户名已存在")
		} else {
			problem(w, 500, "注册失败")
		}
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		problem(w, 500, "注册失败")
		return
	}
	jsonOut(w, 201, map[string]string{"status": "pending"})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password, DeviceID string }
	if !decode(w, r, &in) || in.DeviceID == "" {
		if in.DeviceID == "" {
			problem(w, 400, "deviceId 必填")
		}
		return
	}
	var id, hash, role, status, realName string
	err := s.db.Pool.QueryRow(r.Context(), `SELECT id,password_hash,role,status,real_name FROM users WHERE username=$1`, in.Username).Scan(&id, &hash, &role, &status, &realName)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		problem(w, 401, "用户名或密码错误")
		return
	}
	if status != "active" {
		problem(w, 403, "账号尚未审核通过或已停用")
		return
	}
	var sessions int
	_ = s.db.Pool.QueryRow(r.Context(), `SELECT count(*) FROM user_sessions WHERE user_id=$1 AND device_id<>$2`, id, in.DeviceID).Scan(&sessions)
	if sessions >= s.cfg.MaxDevices {
		problem(w, 409, "已达到最多 3 台设备限制")
		return
	}
	tokenID := uuid.NewString()
	now := time.Now()
	claims := jwt.MapClaims{"sub": id, "role": role, "jti": tokenID, "iat": now.Unix(), "exp": now.Add(s.cfg.TokenTTL).Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		problem(w, 500, "登录失败")
		return
	}
	_, err = s.db.Pool.Exec(r.Context(), `INSERT INTO user_sessions(user_id,device_id,token_id) VALUES($1,$2,$3) ON CONFLICT(user_id,device_id) DO UPDATE SET token_id=$3,last_seen_at=now()`, id, in.DeviceID, tokenID)
	if err != nil {
		problem(w, 500, "登录失败")
		return
	}
	jsonOut(w, 200, map[string]any{"token": token, "expiresIn": int(s.cfg.TokenTTL.Seconds()), "user": map[string]string{"id": id, "realName": realName, "role": role}})
}

func (s *server) pendingUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Pool.Query(r.Context(), `SELECT u.id,u.username,u.real_name,u.avatar_url,d.name,u.created_at FROM users u LEFT JOIN departments d ON d.id=u.department_id WHERE u.status='pending' ORDER BY u.created_at`)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, user, name, avatar, dept string
		var created time.Time
		if rows.Scan(&id, &user, &name, &avatar, &dept, &created) == nil {
			items = append(items, map[string]any{"id": id, "username": user, "realName": name, "avatarUrl": avatar, "department": dept, "createdAt": created})
		}
	}
	jsonOut(w, 200, items)
}
func (s *server) approveUser(w http.ResponseWriter, r *http.Request) {
	tag, err := s.db.Pool.Exec(r.Context(), `UPDATE users SET status='active',updated_at=now() WHERE id=$1 AND status='pending'`, r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "待审核用户不存在")
		return
	}
	p := who(r)
	_, _ = s.db.Pool.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,target_type,target_id) VALUES($1,'user.approved','user',$2)`, p.UserID, r.PathValue("id"))
	w.WriteHeader(204)
}

func (s *server) users(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Pool.Query(r.Context(), `SELECT u.id,u.username,u.real_name,u.avatar_url,coalesce(d.name,'') FROM users u LEFT JOIN departments d ON d.id=u.department_id WHERE u.status='active' ORDER BY CASE WHEN u.id=$1 THEN 0 ELSE 1 END,d.name,u.real_name`, who(r).UserID)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, uname, name, avatar, dept string
		if rows.Scan(&id, &uname, &name, &avatar, &dept) == nil {
			items = append(items, map[string]any{"id": id, "username": uname, "realName": name, "avatarUrl": avatar, "department": dept})
		}
	}
	jsonOut(w, 200, items)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	var uname, realName, avatar, dept string
	err := s.db.Pool.QueryRow(r.Context(), `SELECT u.username,u.real_name,u.avatar_url,coalesce(d.name,'') FROM users u LEFT JOIN departments d ON d.id=u.department_id WHERE u.id=$1`, p.UserID).Scan(&uname, &realName, &avatar, &dept)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	jsonOut(w, 200, map[string]string{"id": p.UserID, "username": uname, "realName": realName, "avatarUrl": avatar, "department": dept, "role": p.Role})
}

func (s *server) conversations(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	rows, err := s.db.Pool.Query(r.Context(), `SELECT c.id,c.kind,CASE WHEN c.kind='direct' THEN (SELECT u2.real_name FROM conversation_members m2 JOIN users u2 ON u2.id=m2.user_id WHERE m2.conversation_id=c.id AND m2.user_id<>$1 LIMIT 1) ELSE coalesce(c.name,'') END,c.created_at FROM conversations c JOIN conversation_members m ON m.conversation_id=c.id WHERE m.user_id=$1 ORDER BY c.created_at DESC`, p.UserID)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, kind, name string
		var created time.Time
		_ = rows.Scan(&id, &kind, &name, &created)
		items = append(items, map[string]any{"id": id, "kind": kind, "name": name, "createdAt": created})
	}
	jsonOut(w, 200, items)
}

func (s *server) createConversation(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	var in struct {
		Kind, Name string
		MemberIDs  []string
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "direct" && in.Kind != "group" {
		problem(w, 400, "会话类型必须是 direct 或 group")
		return
	}
	if in.Kind == "direct" && len(in.MemberIDs) != 1 {
		problem(w, 400, "单聊必须指定一名成员")
		return
	}
	if in.Kind == "group" && strings.TrimSpace(in.Name) == "" {
		problem(w, 400, "群聊名称必填")
		return
	}
	if in.Kind == "direct" {
		var existing string
		err := s.db.Pool.QueryRow(r.Context(), `SELECT c.id FROM conversations c JOIN conversation_members m1 ON m1.conversation_id=c.id AND m1.user_id=$1 JOIN conversation_members m2 ON m2.conversation_id=c.id AND m2.user_id=$2 WHERE c.kind='direct' LIMIT 1`, p.UserID, in.MemberIDs[0]).Scan(&existing)
		if err == nil {
			jsonOut(w, 200, map[string]any{"id": existing, "kind": "direct", "name": ""})
			return
		}
	}
	tx, err := s.db.Pool.Begin(r.Context())
	if err != nil {
		problem(w, 500, "创建失败")
		return
	}
	defer tx.Rollback(r.Context())
	var id string
	if err = tx.QueryRow(r.Context(), `INSERT INTO conversations(kind,name,owner_id) VALUES($1,$2,$3) RETURNING id`, in.Kind, in.Name, p.UserID).Scan(&id); err != nil {
		problem(w, 500, "创建失败")
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO conversation_members(conversation_id,user_id,role) VALUES($1,$2,'owner')`, id, p.UserID); err != nil {
		problem(w, 500, "创建失败")
		return
	}
	for _, memberID := range in.MemberIDs {
		if memberID == p.UserID {
			continue
		}
		tag, e := tx.Exec(r.Context(), `INSERT INTO conversation_members(conversation_id,user_id) SELECT $1,id FROM users WHERE id=$2 AND status='active' ON CONFLICT DO NOTHING`, id, memberID)
		if e != nil || tag.RowsAffected() == 0 {
			problem(w, 400, "成员不存在或尚未审核")
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		problem(w, 500, "创建失败")
		return
	}
	jsonOut(w, 201, map[string]any{"id": id, "kind": in.Kind, "name": in.Name})
}

func (s *server) prepareFile(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	var in struct {
		ConversationID, Name, MimeType, SHA256, ClientLocator string
		SizeBytes                                             int64
		KeepOnServer                                          bool
	}
	if !decode(w, r, &in) {
		return
	}
	if in.SizeBytes < 0 || in.SizeBytes > 10*1024*1024*1024 {
		problem(w, 400, "单个文件不能超过 10GB")
		return
	}
	if in.Name == "" || !s.isMember(r.Context(), in.ConversationID, p.UserID) {
		problem(w, 403, "文件信息无效或无权访问会话")
		return
	}
	const threshold int64 = 100 * 1024 * 1024
	mode := "client"
	if in.KeepOnServer || in.SizeBytes < threshold {
		mode = "server"
	}
	if mode == "client" && in.ClientLocator == "" {
		problem(w, 400, "客户端存储文件必须提供本地文件标识")
		return
	}
	var id string
	err := s.db.Pool.QueryRow(r.Context(), `INSERT INTO files(conversation_id,uploader_id,name,size_bytes,mime_type,sha256,storage_mode,client_locator) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, in.ConversationID, p.UserID, in.Name, in.SizeBytes, in.MimeType, in.SHA256, mode, in.ClientLocator).Scan(&id)
	if err != nil {
		problem(w, 500, "文件登记失败")
		return
	}
	out := map[string]any{"id": id, "storageMode": mode, "thresholdBytes": threshold, "uploadRequired": mode == "server"}
	// 服务器模式下一阶段由 MinIO 适配器返回预签名 URL；当前不会假装上传已经完成。
	jsonOut(w, 201, out)
}

func (s *server) messages(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	cid := r.PathValue("id")
	if !s.isMember(r.Context(), cid, p.UserID) {
		problem(w, 403, "无权访问该会话")
		return
	}
	limit := 50
	before := r.URL.Query().Get("before")
	if before == "" {
		before = time.Now().Add(time.Second).Format(time.RFC3339Nano)
	}
	rows, err := s.db.Pool.Query(r.Context(), `SELECT id,sender_id,type,content,reply_to_id,metadata,retracted_at,created_at FROM messages WHERE conversation_id=$1 AND created_at<$2 ORDER BY created_at DESC LIMIT $3`, cid, before, limit)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, sender, typ, content string
		var reply *string
		var retracted *time.Time
		var metadata []byte
		var created time.Time
		if rows.Scan(&id, &sender, &typ, &content, &reply, &metadata, &retracted, &created) == nil {
			var meta any
			_ = json.Unmarshal(metadata, &meta)
			items = append(items, map[string]any{"id": id, "senderId": sender, "type": typ, "content": content, "replyToId": reply, "metadata": meta, "retractedAt": retracted, "createdAt": created})
		}
	}
	jsonOut(w, 200, items)
}

func (s *server) sendMessage(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	cid := r.PathValue("id")
	if !s.isMember(r.Context(), cid, p.UserID) {
		problem(w, 403, "无权访问该会话")
		return
	}
	var in struct {
		Type, Content string
		ReplyToID     *string
		Metadata      map[string]any
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Type == "" {
		in.Type = "text"
	}
	meta, _ := json.Marshal(in.Metadata)
	var id string
	var created time.Time
	err := s.db.Pool.QueryRow(r.Context(), `INSERT INTO messages(conversation_id,sender_id,type,content,reply_to_id,metadata) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,created_at`, cid, p.UserID, in.Type, in.Content, in.ReplyToID, meta).Scan(&id, &created)
	if err != nil {
		problem(w, 500, "发送失败")
		return
	}
	out := map[string]any{"id": id, "conversationId": cid, "senderId": p.UserID, "type": in.Type, "content": in.Content, "replyToId": in.ReplyToID, "metadata": in.Metadata, "createdAt": created}
	s.publishConversation(r.Context(), cid, "message.created", out)
	jsonOut(w, 201, out)
}

func (s *server) readMessage(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	id := r.PathValue("id")
	tag, err := s.db.Pool.Exec(r.Context(), `INSERT INTO message_reads(message_id,user_id) SELECT m.id,$2 FROM messages m JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.user_id=$2 WHERE m.id=$1 ON CONFLICT DO NOTHING`, id, p.UserID)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "消息不存在或无权访问")
		return
	}
	w.WriteHeader(204)
}
func (s *server) retractMessage(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	id := r.PathValue("id")
	var cid string
	err := s.db.Pool.QueryRow(r.Context(), `UPDATE messages SET retracted_at=now(),content='' WHERE id=$1 AND sender_id=$2 AND retracted_at IS NULL AND created_at>=now()-interval '5 minutes' RETURNING conversation_id`, id, p.UserID).Scan(&cid)
	if errors.Is(err, pgx.ErrNoRows) {
		problem(w, 409, "只能在 5 分钟内撤回自己的消息")
		return
	}
	if err != nil {
		problem(w, 500, "撤回失败")
		return
	}
	s.publishConversation(r.Context(), cid, "message.retracted", map[string]string{"id": id, "conversationId": cid})
	w.WriteHeader(204)
}

func (s *server) canManage(ctx context.Context, cid, uid string) bool {
	var role string
	_ = s.db.Pool.QueryRow(ctx, `SELECT role FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, cid, uid).Scan(&role)
	return role == "owner" || role == "admin"
}

func (s *server) addMembers(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	cid := r.PathValue("id")
	if !s.canManage(r.Context(), cid, p.UserID) {
		problem(w, 403, "需要群主或管理员权限")
		return
	}
	var in struct{ UserIDs []string }
	if !decode(w, r, &in) {
		return
	}
	for _, uid := range in.UserIDs {
		if uid == p.UserID {
			continue
		}
		_, _ = s.db.Pool.Exec(r.Context(), `INSERT INTO conversation_members(conversation_id,user_id) SELECT $1,id FROM users WHERE id=$2 AND status='active' ON CONFLICT DO NOTHING`, cid, uid)
	}
	s.publishConversation(r.Context(), cid, "conversation.updated", map[string]string{"id": cid})
	w.WriteHeader(204)
}

func (s *server) removeMember(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	cid := r.PathValue("id")
	uid := r.PathValue("userId")
	if uid == p.UserID {
		var role string
		_ = s.db.Pool.QueryRow(r.Context(), `SELECT role FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, cid, uid).Scan(&role)
		if role == "owner" {
			problem(w, 409, "群主不能退出群聊，请先转让或解散群聊")
			return
		}
	} else if !s.canManage(r.Context(), cid, p.UserID) {
		problem(w, 403, "需要群主或管理员权限")
		return
	}
	tag, err := s.db.Pool.Exec(r.Context(), `DELETE FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, cid, uid)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "成员不存在")
		return
	}
	s.publishConversation(r.Context(), cid, "conversation.updated", map[string]string{"id": cid})
	w.WriteHeader(204)
}

func (s *server) renameConversation(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	cid := r.PathValue("id")
	if !s.canManage(r.Context(), cid, p.UserID) {
		problem(w, 403, "需要群主或管理员权限")
		return
	}
	var in struct{ Name string }
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		problem(w, 400, "群聊名称不能为空")
		return
	}
	tag, err := s.db.Pool.Exec(r.Context(), `UPDATE conversations SET name=$1 WHERE id=$2 AND kind='group'`, in.Name, cid)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "会话不存在")
		return
	}
	s.publishConversation(r.Context(), cid, "conversation.updated", map[string]string{"id": cid})
	jsonOut(w, 200, map[string]string{"id": cid, "name": in.Name})
}

func (s *server) disbandConversation(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	cid := r.PathValue("id")
	var role string
	_ = s.db.Pool.QueryRow(r.Context(), `SELECT role FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, cid, p.UserID).Scan(&role)
	if role != "owner" {
		problem(w, 403, "只有群主可以解散群聊")
		return
	}
	tag, err := s.db.Pool.Exec(r.Context(), `DELETE FROM conversations WHERE id=$1 AND kind='group'`, cid)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "会话不存在")
		return
	}
	s.publishConversation(r.Context(), cid, "conversation.updated", map[string]string{"id": cid})
	w.WriteHeader(204)
}

func (s *server) departmentRequest(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	var in struct{ Department, Reason string }
	if !decode(w, r, &in) {
		return
	}
	in.Department = strings.TrimSpace(in.Department)
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Department == "" || in.Reason == "" {
		problem(w, 400, "目标部门和理由必填")
		return
	}
	var exists bool
	_ = s.db.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM department_change_requests WHERE user_id=$1 AND status='pending')`, p.UserID).Scan(&exists)
	if exists {
		problem(w, 409, "已有待审批的转部门申请")
		return
	}
	var deptID string
	tx, err := s.db.Pool.Begin(r.Context())
	if err != nil {
		problem(w, 500, "提交失败")
		return
	}
	defer tx.Rollback(r.Context())
	if err = tx.QueryRow(r.Context(), `INSERT INTO departments(name) VALUES($1) ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, in.Department).Scan(&deptID); err != nil {
		problem(w, 500, "提交失败")
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO department_change_requests(user_id,target_department_id,reason) VALUES($1,$2,$3)`, p.UserID, deptID, in.Reason); err != nil {
		problem(w, 500, "提交失败")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		problem(w, 500, "提交失败")
		return
	}
	jsonOut(w, 201, map[string]string{"status": "pending"})
}

func (s *server) departmentRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Pool.Query(r.Context(), `SELECT r.id,u.username,u.real_name,d.name,r.reason,r.created_at FROM department_change_requests r JOIN users u ON u.id=r.user_id JOIN departments d ON d.id=r.target_department_id WHERE r.status='pending' ORDER BY r.created_at`)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, uname, real, dept, reason string
		var created time.Time
		if rows.Scan(&id, &uname, &real, &dept, &reason, &created) == nil {
			items = append(items, map[string]any{"id": id, "username": uname, "realName": real, "targetDepartment": dept, "reason": reason, "createdAt": created})
		}
	}
	jsonOut(w, 200, items)
}

func (s *server) approveDepartmentRequest(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	id := r.PathValue("id")
	tx, err := s.db.Pool.Begin(r.Context())
	if err != nil {
		problem(w, 500, "审批失败")
		return
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), `UPDATE department_change_requests SET status='approved',reviewed_by=$1,reviewed_at=now() WHERE id=$2 AND status='pending'`, p.UserID, id)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "申请不存在")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE users SET department_id=(SELECT target_department_id FROM department_change_requests WHERE id=$1),updated_at=now() WHERE id=(SELECT user_id FROM department_change_requests WHERE id=$1)`, id); err != nil {
		problem(w, 500, "审批失败")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		problem(w, 500, "审批失败")
		return
	}
	_, _ = s.db.Pool.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,target_type,target_id) VALUES($1,'dept.approved','department_change_request',$2)`, p.UserID, id)
	w.WriteHeader(204)
}

func (s *server) rejectDepartmentRequest(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	id := r.PathValue("id")
	tag, err := s.db.Pool.Exec(r.Context(), `UPDATE department_change_requests SET status='rejected',reviewed_by=$1,reviewed_at=now() WHERE id=$2 AND status='pending'`, p.UserID, id)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "申请不存在")
		return
	}
	w.WriteHeader(204)
}

func (s *server) adminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Pool.Query(r.Context(), `SELECT u.id,u.username,u.real_name,coalesce(d.name,''),u.status,u.created_at FROM users u LEFT JOIN departments d ON d.id=u.department_id ORDER BY u.created_at`)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, uname, real, dept, status string
		var created time.Time
		if rows.Scan(&id, &uname, &real, &dept, &status, &created) == nil {
			items = append(items, map[string]any{"id": id, "username": uname, "realName": real, "department": dept, "status": status, "createdAt": created})
		}
	}
	jsonOut(w, 200, items)
}

func (s *server) rejectUser(w http.ResponseWriter, r *http.Request) {
	s.adminStatusUpdate(w, r, "pending", "rejected")
}
func (s *server) disableUser(w http.ResponseWriter, r *http.Request) {
	s.adminStatusUpdate(w, r, "active", "disabled")
}
func (s *server) enableUser(w http.ResponseWriter, r *http.Request) {
	s.adminStatusUpdate(w, r, "disabled", "active")
}

func (s *server) adminStatusUpdate(w http.ResponseWriter, r *http.Request, from, to string) {
	p := who(r)
	id := r.PathValue("id")
	tag, err := s.db.Pool.Exec(r.Context(), `UPDATE users SET status=$1,updated_at=now() WHERE id=$2 AND status=$3`, to, id, from)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 409, "用户当前状态不允许该操作")
		return
	}
	_, _ = s.db.Pool.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,target_type,target_id) VALUES($1,'user.'||$2,'user',$3)`, p.UserID, to, id)
	w.WriteHeader(204)
}

func (s *server) resetPassword(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	id := r.PathValue("id")
	var in struct{ NewPassword string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.NewPassword) < 8 || len([]byte(in.NewPassword)) > 72 {
		problem(w, 400, "新密码至少 8 位、不超过 72 字节")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		problem(w, 500, "重置失败")
		return
	}
	tag, err := s.db.Pool.Exec(r.Context(), `UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2`, string(hash), id)
	if err != nil || tag.RowsAffected() == 0 {
		problem(w, 404, "用户不存在")
		return
	}
	_, _ = s.db.Pool.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,target_type,target_id) VALUES($1,'user.password_reset','user',$2)`, p.UserID, id)
	w.WriteHeader(204)
}

func (s *server) conversationMembers(w http.ResponseWriter, r *http.Request) {
	p := who(r)
	cid := r.PathValue("id")
	if !s.isMember(r.Context(), cid, p.UserID) {
		problem(w, 403, "无权访问该会话")
		return
	}
	rows, err := s.db.Pool.Query(r.Context(), `SELECT u.id,u.username,u.real_name,m.role FROM conversation_members m JOIN users u ON u.id=m.user_id WHERE m.conversation_id=$1 ORDER BY m.joined_at`, cid)
	if err != nil {
		problem(w, 500, "查询失败")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, uname, real, role string
		if rows.Scan(&id, &uname, &real, &role) == nil {
			items = append(items, map[string]any{"id": id, "username": uname, "realName": real, "role": role})
		}
	}
	jsonOut(w, 200, items)
}

func (s *server) isMember(ctx context.Context, cid, uid string) bool {
	var ok bool
	_ = s.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)`, cid, uid).Scan(&ok)
	return ok
}

func (s *server) require(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if raw == "" {
			protocols := websocket.Subprotocols(r)
			if len(protocols) == 2 && protocols[0] == "bearer" {
				raw = protocols[1]
			}
		}
		if raw == "" {
			problem(w, 401, "需要 Bearer 凭证")
			return
		}
		token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("invalid signing method")
			}
			return []byte(s.cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			problem(w, 401, "身份凭证无效")
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			problem(w, 401, "身份凭证无效")
			return
		}
		uid, uok := claims["sub"].(string)
		userRole, rok := claims["role"].(string)
		tokenID, tok := claims["jti"].(string)
		if !uok || !rok || !tok {
			problem(w, 401, "身份凭证无效")
			return
		}
		p := principal{UserID: uid, Role: userRole, TokenID: tokenID}
		var exists bool
		_ = s.db.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM user_sessions WHERE user_id=$1 AND token_id=$2)`, p.UserID, p.TokenID).Scan(&exists)
		if !exists {
			problem(w, 401, "登录已失效")
			return
		}
		if role != "" && p.Role != role {
			problem(w, 403, "权限不足")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
	})
}
func who(r *http.Request) principal { return r.Context().Value(principalKey).(principal) }

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "null" || strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost:")
}}

func (s *server) websocket(w http.ResponseWriter, r *http.Request) {
	responseHeader := http.Header{}
	if protocols := websocket.Subprotocols(r); len(protocols) > 0 && protocols[0] == "bearer" {
		responseHeader.Set("Sec-WebSocket-Protocol", "bearer")
	}
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return
	}
	p := who(r)
	client := s.hub.add(p.UserID, conn)
	defer s.hub.remove(p.UserID, client)
	for {
		if _, _, err = conn.ReadMessage(); err != nil {
			return
		}
	}
}

type clientConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}
type hub struct {
	mu      sync.RWMutex
	clients map[string]map[*clientConn]struct{}
}

func newHub() *hub { return &hub{clients: map[string]map[*clientConn]struct{}{}} }
func (h *hub) add(uid string, c *websocket.Conn) *clientConn {
	client := &clientConn{conn: c}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[uid] == nil {
		h.clients[uid] = map[*clientConn]struct{}{}
	}
	h.clients[uid][client] = struct{}{}
	return client
}
func (h *hub) remove(uid string, client *clientConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[uid], client)
	_ = client.conn.Close()
}
func (h *hub) publish(users []string, cid, typ string, data any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	payload, _ := json.Marshal(map[string]any{"type": typ, "data": data, "conversationId": cid})
	for _, uid := range users {
		for client := range h.clients[uid] {
			client.writeMu.Lock()
			_ = client.conn.WriteMessage(websocket.TextMessage, payload)
			client.writeMu.Unlock()
		}
	}
}
func (s *server) publishConversation(ctx context.Context, cid, typ string, data any) {
	rows, err := s.db.Pool.Query(ctx, `SELECT user_id FROM conversation_members WHERE conversation_id=$1`, cid)
	if err != nil {
		return
	}
	defer rows.Close()
	var users []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			users = append(users, id)
		}
	}
	s.hub.publish(users, cid, typ, data)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		problem(w, 400, "请求内容格式错误")
		return false
	}
	return true
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, message string) {
	jsonOut(w, status, map[string]string{"error": message})
}
func recoverAndCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || origin == "null" || strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost:") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		defer func() {
			if recover() != nil {
				problem(w, 500, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
