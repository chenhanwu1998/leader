package main

// 账号体系：用户表 + 会话（独立于各账号的台账库，单独存 data/users.db）
// 与 leader-sys（Express 版）的 server/users.js 一一对应。

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
)

const bootUser = "lenhar"

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{2,32}$`)
const sessionDays = 30

var udb *sql.DB

type AuthUser struct {
	Username    string
	DisplayName string
	Avatar      string
	Role        string
	CreatedAt   string
}

func (u *AuthUser) Map() map[string]any {
	return map[string]any{
		"username":     u.Username,
		"display_name": u.DisplayName,
		"avatar":       u.Avatar,
		"role":         u.Role,
		"created_at":   u.CreatedAt,
	}
}

func initUsersDB(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "users"), 0o755); err != nil {
		return err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", filepath.ToSlash(filepath.Join(dir, "users.db")))
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(`
CREATE TABLE IF NOT EXISTS users (
  username TEXT PRIMARY KEY,
  password_hash TEXT NOT NULL,
  salt TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  avatar TEXT DEFAULT '',
  role TEXT NOT NULL DEFAULT 'user',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  username TEXT NOT NULL REFERENCES users(username) ON DELETE CASCADE,
  created_at TEXT DEFAULT (datetime('now','localtime')),
  expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT DEFAULT ''
);`); err != nil {
		d.Close()
		return err
	}
	udb = d
	return nil
}

// 与 Node crypto.scryptSync(pw, salt, 32) 同参数：N=16384, r=8, p=1
func hashPassword(password, salt string) string {
	key, err := scrypt.Key([]byte(password), []byte(salt), 16384, 8, 1, 32)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(key)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// 默认头像：首字符 + 按用户名哈希取色的圆形 SVG（未上传头像时使用）
var avatarColors = [][2]string{
	{"#4f6bed", "#7c5bf5"}, {"#e06b5a", "#e89a4b"}, {"#3b9d78", "#5dbb8f"}, {"#4b8fd4", "#6cb3ee"},
	{"#9b62d9", "#b983ec"}, {"#d95f8b", "#ec83ab"}, {"#5aa8a0", "#7cc4bc"}, {"#c98a3d", "#e0a95f"},
}

func defaultAvatar(displayName, username string) string {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = username
	}
	ch := "?"
	if name != "" {
		ch = string([]rune(name)[0])
	}
	var h uint32
	for _, c := range username {
		h = h*31 + uint32(c)
	}
	pair := avatarColors[int(h)%len(avatarColors)]
	replacer := strings.NewReplacer("<", "", ">", "", "&", "", "\"", "")
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs><rect width="128" height="128" rx="64" fill="url(#g)"/><text x="64" y="82" font-size="56" text-anchor="middle" fill="#fff" font-family="sans-serif">%s</text></svg>`, pair[0], pair[1], replacer.Replace(ch))
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

func publicUser(row map[string]any) *AuthUser {
	if row == nil {
		return nil
	}
	avatar := toStr(row["avatar"])
	if avatar == "" {
		avatar = defaultAvatar(toStr(row["display_name"]), toStr(row["username"]))
	}
	return &AuthUser{
		Username:    toStr(row["username"]),
		DisplayName: toStr(row["display_name"]),
		Avatar:      avatar,
		Role:        toStr(row["role"]),
		CreatedAt:   toStr(row["created_at"]),
	}
}

func getUser(username string) *AuthUser {
	return publicUser(queryOne(udb, `SELECT * FROM users WHERE username = ?`, username))
}

func createUser(username, password, displayName, role string) (*AuthUser, error) {
	username = strings.TrimSpace(username)
	if !usernameRe.MatchString(username) {
		return nil, errors.New("账号需为 2-32 位字母、数字或下划线")
	}
	if len(password) < 4 {
		return nil, errors.New("密码至少 4 位")
	}
	if queryOne(udb, `SELECT 1 AS x FROM users WHERE username = ?`, username) != nil {
		return nil, errors.New("该账号已被注册")
	}
	salt := randomHex(16)
	dn := strings.TrimSpace(displayName)
	if dn == "" {
		dn = username
	}
	if role == "" {
		role = "user"
	}
	_, err := udb.Exec(`INSERT INTO users(username, password_hash, salt, display_name, avatar, role) VALUES (?,?,?,?,?,?)`,
		username, hashPassword(password, salt), salt, dn, "", role)
	if err != nil {
		return nil, err
	}
	return getUser(username), nil
}

func verifyUser(username, password string) *AuthUser {
	row := queryOne(udb, `SELECT * FROM users WHERE username = ?`, strings.TrimSpace(username))
	if row == nil {
		return nil
	}
	got := hashPassword(password, toStr(row["salt"]))
	if subtle.ConstantTimeCompare([]byte(got), []byte(toStr(row["password_hash"]))) != 1 {
		return nil
	}
	return publicUser(row)
}

func changePassword(username, oldPassword, newPassword string) error {
	row := queryOne(udb, `SELECT * FROM users WHERE username = ?`, username)
	if row == nil {
		return errors.New("账号不存在")
	}
	if hashPassword(oldPassword, toStr(row["salt"])) != toStr(row["password_hash"]) {
		return errors.New("原密码不正确")
	}
	if len(newPassword) < 4 {
		return errors.New("新密码至少 4 位")
	}
	salt := randomHex(16)
	if _, err := udb.Exec(`UPDATE users SET password_hash = ?, salt = ? WHERE username = ?`, hashPassword(newPassword, salt), salt, username); err != nil {
		return err
	}
	// 改密后旧会话全部失效
	_, err := udb.Exec(`DELETE FROM sessions WHERE username = ?`, username)
	return err
}

func adminResetPassword(username, newPassword string) error {
	if len(newPassword) < 4 {
		return errors.New("新密码至少 4 位")
	}
	row := queryOne(udb, `SELECT * FROM users WHERE username = ?`, username)
	if row == nil {
		return errors.New("账号不存在")
	}
	salt := randomHex(16)
	if _, err := udb.Exec(`UPDATE users SET password_hash = ?, salt = ? WHERE username = ?`, hashPassword(newPassword, salt), salt, username); err != nil {
		return err
	}
	_, err := udb.Exec(`DELETE FROM sessions WHERE username = ?`, username)
	return err
}

func updateProfile(username string, body map[string]any) (*AuthUser, error) {
	if queryOne(udb, `SELECT 1 AS x FROM users WHERE username = ?`, username) == nil {
		return nil, errors.New("账号不存在")
	}
	if v, ok := body["display_name"]; ok {
		dn := strings.TrimSpace(toStr(v))
		if dn == "" {
			return nil, errors.New("昵称不能为空")
		}
		r := []rune(dn)
		if len(r) > 32 {
			dn = string(r[:32])
		}
		if _, err := udb.Exec(`UPDATE users SET display_name = ? WHERE username = ?`, dn, username); err != nil {
			return nil, err
		}
	}
	if v, ok := body["avatar"]; ok {
		a := toStr(v)
		if a == "" {
			if _, err := udb.Exec(`UPDATE users SET avatar = '' WHERE username = ?`, username); err != nil {
				return nil, err
			}
		} else {
			if !strings.HasPrefix(a, "data:image/") {
				return nil, errors.New("头像需为图片")
			}
			if len(a) > 900_000 {
				return nil, errors.New("头像图片过大（请小于 700KB）")
			}
			if _, err := udb.Exec(`UPDATE users SET avatar = ? WHERE username = ?`, a, username); err != nil {
				return nil, err
			}
		}
	}
	return getUser(username), nil
}

// —— 会话 ——

func createSession(username string) string {
	token := randomHex(32)
	expires := time.Now().Add(time.Duration(sessionDays) * 24 * time.Hour).UTC().Format(time.RFC3339)
	udb.Exec(`INSERT INTO sessions(token, username, expires_at) VALUES (?,?,?)`, token, username, expires)
	return token
}

func authenticate(token string) *AuthUser {
	if token == "" {
		return nil
	}
	row := queryOne(udb, `SELECT username, expires_at FROM sessions WHERE token = ?`, token)
	if row == nil {
		return nil
	}
	exp, err := time.Parse(time.RFC3339, toStr(row["expires_at"]))
	if err != nil || time.Now().After(exp) {
		udb.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return nil
	}
	return getUser(toStr(row["username"]))
}

func destroySession(token string) {
	udb.Exec(`DELETE FROM sessions WHERE token = ?`, token)
}

// —— 管理员 ——

func listUsers() []*AuthUser {
	rows, err := queryAll(udb, `SELECT * FROM users ORDER BY created_at`)
	if err != nil {
		return nil
	}
	out := make([]*AuthUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, publicUser(r))
	}
	return out
}

func deleteUser(username string) error {
	row := queryOne(udb, `SELECT * FROM users WHERE username = ?`, username)
	if row == nil {
		return errors.New("账号不存在")
	}
	if toStr(row["role"]) == "admin" {
		return errors.New("不能删除管理员账号")
	}
	if _, err := udb.Exec(`DELETE FROM users WHERE username = ?`, username); err != nil { // 会话级联删除
		return err
	}
	closeLedger(username) // 关闭占用中的库句柄，避免 Windows 下文件删不掉
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(ledgerFile(username) + suffix) // 文件被占用时跳过
	}
	return nil
}

func setGlobalSetting(key, value string) {
	udb.Exec(`INSERT INTO app_settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
}

func getGlobalSetting(key, fallback string) string {
	row := queryOne(udb, `SELECT value FROM app_settings WHERE key = ?`, key)
	if row == nil {
		return fallback
	}
	return toStr(row["value"])
}

// —— 初始化：种子账号 + 旧单库数据迁移到 lenhar 名下 ——

func seedUsers() {
	n := queryCount(udb, `SELECT COUNT(*) AS n FROM users`)
	if n > 0 {
		return
	}
	if _, err := createUser("lenhar", "lenhar", "梁老师", "user"); err != nil {
		log.Printf("种子账号 lenhar 创建失败: %v", err)
	}
	if _, err := createUser("admin", "admin", "管理员", "admin"); err != nil {
		log.Printf("种子账号 admin 创建失败: %v", err)
	}
	log.Println("账号初始化完成：lenhar（梁老师）、admin（管理员）")

	// 旧版单库 data/teacher-ledger.db 归属到 lenhar（VACUUM INTO 快照，原文件保留作为备份）
	legacy := filepath.Join(baseDataDir, "teacher-ledger.db")
	target := ledgerFile("lenhar")
	if _, err := os.Stat(legacy); err != nil {
		return
	}
	if _, err := os.Stat(target); err == nil {
		return
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", filepath.ToSlash(legacy))
	src, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Printf("旧数据迁移失败: %v", err)
		return
	}
	defer src.Close()
	escaped := strings.ReplaceAll(filepath.ToSlash(target), `'`, `''`)
	if _, err := src.Exec(`VACUUM INTO '` + escaped + `'`); err != nil {
		log.Printf("旧数据迁移失败: %v", err)
		return
	}
	log.Println("旧数据已迁移到 lenhar 账号（原 teacher-ledger.db 保留作备份）")
}
