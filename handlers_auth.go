package main

// 登录 / 注册 / 个人资料 / 管理员：与 leader-sys 的 server/routes/auth.js 一一对应

import (
	"context"
	"net/http"
	"os"
	"strings"
)

type ctxKey struct{ name string }

var userCtxKey = &ctxKey{"user"}

func currentUser(r *http.Request) *AuthUser {
	if u, ok := r.Context().Value(userCtxKey).(*AuthUser); ok {
		return u
	}
	return nil
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return h[7:]
	}
	return ""
}

// authMiddleware 除健康检查、登录注册、公告、一次性导出下载外，其余 /api 都需要登录会话
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/api/health",
			p == "/api/auth/login",
			p == "/api/auth/register",
			p == "/api/auth/announcement",
			strings.HasPrefix(p, "/api/export/file/"): // WebView 直链下载，一次性随机 id 自销毁
			next.ServeHTTP(w, r)
			return
		}
		user := authenticate(bearerToken(r))
		if user == nil {
			fail(w, 401, "未登录或登录已过期，请重新登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, user)))
	})
}

func requireAdmin(w http.ResponseWriter, r *http.Request) *AuthUser {
	u := currentUser(r)
	if u == nil || u.Role != "admin" {
		fail(w, 403, "需要管理员权限")
		return nil
	}
	return u
}

func registerAuthRoutes(mux *http.ServeMux) {
	// 注册：只需账号 + 密码（昵称可留空，默认用账号）
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		username := strings.TrimSpace(strField(body, "username", ""))
		if !usernameRe.MatchString(username) {
			fail(w, 400, "账号需为 2-32 位字母、数字或下划线")
			return
		}
		user, err := createUser(username, strField(body, "password", ""), strField(body, "display_name", ""), "user")
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		ledgerFor(user.Username) // 立即建好个人台账库
		token := createSession(user.Username)
		writeJSON(w, 200, map[string]any{"token": token, "user": user.Map()})
	})

	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		user := verifyUser(strField(body, "username", ""), strField(body, "password", ""))
		if user == nil {
			fail(w, 401, "账号或密码不正确")
			return
		}
		ledgerFor(user.Username)
		token := createSession(user.Username)
		writeJSON(w, 200, map[string]any{"token": token, "user": user.Map()})
	})

	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		destroySession(bearerToken(r))
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, currentUser(r).Map())
	})

	mux.HandleFunc("PUT /api/auth/profile", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		user, err := updateProfile(currentUser(r).Username, body)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, user.Map())
	})

	mux.HandleFunc("PUT /api/auth/password", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		username := currentUser(r).Username
		if err := changePassword(username, strField(body, "old_password", ""), strField(body, "new_password", "")); err != nil {
			fail(w, 400, err.Error())
			return
		}
		// 旧会话已清，直接发新会话免去重新登录
		token := createSession(username)
		writeJSON(w, 200, map[string]any{"ok": true, "token": token})
	})

	// 全局公告（登录页/工作台横幅），所有人可读
	mux.HandleFunc("GET /api/auth/announcement", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"value": getGlobalSetting("announcement", "")})
	})
}

func registerAdminRoutes(mux *http.ServeMux) {
	// 用户列表 + 各账号台账数据统计（只读汇总，不读取具体业务数据）
	mux.HandleFunc("GET /api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		out := []map[string]any{}
		for _, u := range listUsers() {
			stats := map[string]any{"students": 0, "classes": 0, "exams": 0, "scores": 0, "psy_profiles": 0}
			var size int64
			if d, err := ledgerFor(u.Username); err == nil {
				stats["students"] = queryCount(d, `SELECT COUNT(*) n FROM students`)
				stats["classes"] = queryCount(d, `SELECT COUNT(*) n FROM classes`)
				stats["exams"] = queryCount(d, `SELECT COUNT(*) n FROM exams`)
				stats["scores"] = queryCount(d, `SELECT COUNT(*) n FROM scores`)
				stats["psy_profiles"] = queryCount(d, `SELECT COUNT(*) n FROM psy_profiles`)
				if fi, err := os.Stat(ledgerFile(u.Username)); err == nil {
					size = fi.Size()
				}
			}
			m := u.Map()
			m["stats"] = stats
			m["db_size"] = size
			out = append(out, m)
		}
		writeJSON(w, 200, out)
	})

	mux.HandleFunc("PUT /api/admin/users/{username}/password", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		username := r.PathValue("username")
		if getUser(username) == nil {
			fail(w, 404, "账号不存在")
			return
		}
		if err := adminResetPassword(username, strField(body, "new_password", "")); err != nil {
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("DELETE /api/admin/users/{username}", func(w http.ResponseWriter, r *http.Request) {
		admin := requireAdmin(w, r)
		if admin == nil {
			return
		}
		username := r.PathValue("username")
		if username == admin.Username {
			fail(w, 400, "不能删除当前登录的账号")
			return
		}
		if err := deleteUser(username); err != nil {
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("PUT /api/admin/announcement", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		v := strField(body, "value", "")
		runes := []rune(v)
		if len(runes) > 500 {
			v = string(runes[:500])
		}
		setGlobalSetting("announcement", v)
		writeJSON(w, 200, map[string]any{"ok": true})
	})
}
