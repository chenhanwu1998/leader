package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// 每个账号一个独立的台账库（data/users/<账号>.db），按登录会话路由
var baseDataDir string
var ledgerMu sync.Mutex
var ledgerPool = map[string]*sql.DB{}

const schema = `
CREATE TABLE IF NOT EXISTS classes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  grade TEXT DEFAULT '',
  head_teacher TEXT DEFAULT '',
  classroom TEXT DEFAULT '',
  remark TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS students (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  student_no TEXT DEFAULT '',
  name TEXT NOT NULL,
  gender TEXT DEFAULT '男',
  class_name TEXT DEFAULT '',
  birth_date TEXT DEFAULT '',
  phone TEXT DEFAULT '',
  parent_name TEXT DEFAULT '',
  parent_phone TEXT DEFAULT '',
  address TEXT DEFAULT '',
  status TEXT DEFAULT '在读',
  remark TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime')),
  updated_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_students_class ON students(class_name);

CREATE TABLE IF NOT EXISTS exams (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  exam_date TEXT DEFAULT '',
  term TEXT DEFAULT '',
  type TEXT DEFAULT '期中',
  class_name TEXT DEFAULT '',
  remark TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS scores (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  exam_id INTEGER NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  subject TEXT NOT NULL,
  score REAL DEFAULT 0,
  full_score REAL DEFAULT 100,
  created_at TEXT DEFAULT (datetime('now','localtime')),
  updated_at TEXT DEFAULT (datetime('now','localtime')),
  UNIQUE(exam_id, student_id, subject)
);
CREATE INDEX IF NOT EXISTS idx_scores_exam ON scores(exam_id);
CREATE INDEX IF NOT EXISTS idx_scores_student ON scores(student_id);

CREATE TABLE IF NOT EXISTS leaves (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  type TEXT DEFAULT '事假',
  start_date TEXT DEFAULT '',
  end_date TEXT DEFAULT '',
  days REAL DEFAULT 1,
  reason TEXT DEFAULT '',
  status TEXT DEFAULT '已销假',
  remark TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_leaves_student ON leaves(student_id);

CREATE TABLE IF NOT EXISTS rewards (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  kind TEXT DEFAULT '表扬',
  level TEXT DEFAULT '班级',
  title TEXT NOT NULL,
  record_date TEXT DEFAULT '',
  reason TEXT DEFAULT '',
  remark TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_rewards_student ON rewards(student_id);

CREATE TABLE IF NOT EXISTS contacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  contact_date TEXT DEFAULT '',
  way TEXT DEFAULT '电话',
  target TEXT DEFAULT '家长',
  content TEXT DEFAULT '',
  feedback TEXT DEFAULT '',
  remark TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS seat_layouts (
  class_name TEXT PRIMARY KEY,
  rows INTEGER DEFAULT 5,
  cols INTEGER DEFAULT 8,
  group_size INTEGER DEFAULT 2,
  assignments TEXT DEFAULT '[]',
  colors TEXT DEFAULT '{}',
  updated_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS schedules (
  class_name TEXT PRIMARY KEY,
  config TEXT DEFAULT '{}',
  updated_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS class_subjects (
  class_name TEXT NOT NULL,
  name TEXT NOT NULL,
  color TEXT DEFAULT '',
  teacher TEXT DEFAULT '',
  PRIMARY KEY (class_name, name)
);

-- 心理健康：重点关注学生档案
CREATE TABLE IF NOT EXISTS psy_profiles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  student_name TEXT NOT NULL,
  class_name TEXT DEFAULT '',
  gender TEXT DEFAULT '',
  risk_level TEXT DEFAULT '常规关注',
  tags TEXT DEFAULT '',
  note TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime')),
  updated_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_psy_profiles_class ON psy_profiles(class_name);

-- 心理辅导 / 谈话记录
CREATE TABLE IF NOT EXISTS psy_sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL REFERENCES psy_profiles(id) ON DELETE CASCADE,
  session_date TEXT DEFAULT '',
  mode TEXT DEFAULT '面谈',
  topic TEXT DEFAULT '',
  content TEXT DEFAULT '',
  next_follow TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_psy_sessions_profile ON psy_sessions(profile_id);

-- 量表测评结果
CREATE TABLE IF NOT EXISTS psy_assessments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL REFERENCES psy_profiles(id) ON DELETE CASCADE,
  scale TEXT NOT NULL,
  answers TEXT DEFAULT '[]',
  score REAL DEFAULT 0,
  level TEXT DEFAULT '',
  flagged INTEGER DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_psy_assessments_profile ON psy_assessments(profile_id);

-- 智能体对话历史
CREATE TABLE IF NOT EXISTS agent_conversations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL DEFAULT '新对话',
  mode TEXT NOT NULL DEFAULT 'plain',
  model TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime')),
  updated_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS agent_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  conversation_id INTEGER NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  content TEXT DEFAULT '',
  sources TEXT DEFAULT '',
  created_at TEXT DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IF NOT EXISTS idx_agent_messages_conv ON agent_messages(conversation_id);
`

// dbDir 数据目录：优先 exe 同级 data/，不可写时退回 %APPDATA%\teacher-ledger\data
func dbDir() string {
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(exe), "data")
		if err := os.MkdirAll(dir, 0o755); err == nil {
			probe := filepath.Join(dir, ".write-probe")
			if f, err := os.Create(probe); err == nil {
				f.Close()
				os.Remove(probe)
				return dir
			}
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	dir := filepath.Join(base, "teacher-ledger", "data")
	os.MkdirAll(dir, 0o755)
	return dir
}

func ledgerFile(username string) string {
	return filepath.Join(baseDataDir, "users", username+".db")
}

func openLedgerDB(file string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", filepath.ToSlash(file))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite 单写者：限制为一个连接，避免 database/sql 连接池下的 table lock 竞争
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateSeatColors(db); err != nil {
		db.Close()
		return nil, err
	}
	migratePeriods(db)
	migrateDutyCells(db)
	return db, nil
}

// ledgerFor 打开（或复用）某账号的台账库
func ledgerFor(username string) (*sql.DB, error) {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	if d, ok := ledgerPool[username]; ok {
		return d, nil
	}
	d, err := openLedgerDB(ledgerFile(username))
	if err != nil {
		return nil, err
	}
	ledgerPool[username] = d
	return d, nil
}

// closeLedger 关闭并移出连接池（删除账号数据文件前必须先释放句柄，Windows 下文件才被解锁）
func closeLedger(username string) {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()
	if d, ok := ledgerPool[username]; ok {
		d.Close()
		delete(ledgerPool, username)
	}
}

// ledgerDB 按请求会话解析到对应账号的台账库；异常时回退到默认账号
func ledgerDB(r *http.Request) *sql.DB {
	if u := currentUser(r); u != nil {
		if d, err := ledgerFor(u.Username); err == nil {
			return d
		}
	}
	d, _ := ledgerFor(bootUser)
	return d
}

// 旧库迁移：seat_layouts 增加颜色列
func migrateSeatColors(db *sql.DB) error {
	var hasColors bool
	rows, err := db.Query(`PRAGMA table_info(seat_layouts)`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "colors" {
			hasColors = true
		}
	}
	rows.Close()
	if !hasColors {
		if _, err := db.Exec(`ALTER TABLE seat_layouts ADD COLUMN colors TEXT DEFAULT '{}'`); err != nil {
			return err
		}
	}
	return nil
}

// ---------- 查询辅助 ----------

func normalizeRow(row map[string]any) map[string]any {
	for k, v := range row {
		if b, ok := v.([]byte); ok {
			row[k] = string(b)
		}
	}
	return row
}

func queryAll(db *sql.DB, q string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, normalizeRow(m))
	}
	return out, rows.Err()
}

func queryOne(db *sql.DB, q string, args ...any) map[string]any {
	rows, err := queryAll(db, q, args...)
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func queryCount(db *sql.DB, q string, args ...any) int64 {
	row := queryOne(db, q, args...)
	if row == nil {
		return 0
	}
	for _, v := range row {
		return toInt(v)
	}
	return 0
}

func toInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		var x int64
		fmt.Sscan(n, &x)
		return x
	}
	return 0
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case int64:
		return float64(n)
	case float64:
		return n
	}
	return 0
}

func toStr(v any) string {
	switch s := v.(type) {
	case nil:
		return ""
	case string:
		return s
	case []byte:
		return string(s)
	case int64:
		return fmt.Sprintf("%d", s)
	case float64:
		return fmt.Sprintf("%v", s)
	}
	return fmt.Sprintf("%v", v)
}

// ---------- 设置 ----------

func getSetting(db *sql.DB, key, fallback string) string {
	row := queryOne(db, `SELECT value FROM settings WHERE key = ?`, key)
	if row == nil {
		return fallback
	}
	return toStr(row["value"])
}

func setSetting(db *sql.DB, key, value string) {
	db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
}

type Subject struct {
	Name    string `json:"name"`
	Color   string `json:"color"`
	Teacher string `json:"teacher"`
}

var defaultSubjects = []Subject{
	{"语文", "#fef0f0", ""}, {"数学", "#eef3ff", ""}, {"英语", "#f4f0ff", ""},
	{"物理", "#effaf7", ""}, {"化学", "#fef6ec", ""}, {"生物", "#edf9ff", ""},
	{"政治", "#fdf2e8", ""}, {"历史", "#f7efe2", ""}, {"地理", "#eef9f0", ""},
	{"体育", "#fff7e0", ""}, {"音乐", "#fdeef5", ""}, {"美术", "#f0fbff", ""},
	{"信息", "#eef1f8", ""},
}

var presetColors = func() map[string]string {
	m := map[string]string{}
	for _, s := range defaultSubjects {
		m[s.Name] = s.Color
	}
	return m
}()

var fallbackPalette = []string{"#fef0f0", "#eef3ff", "#f4f0ff", "#effaf7", "#fef6ec", "#edf9ff", "#fdf2e8", "#f7efe2", "#eef9f0", "#fff7e0", "#fdeef5", "#f0fbff", "#eef1f8"}

// normalizeSubjects 兼容字符串数组（旧格式）与对象数组，统一为 { name, color, teacher }
func normalizeSubjects(raw json.RawMessage) []Subject {
	out := []Subject{}
	seen := map[string]bool{}
	var items []any
	if json.Unmarshal(raw, &items) != nil {
		return out
	}
	for _, item := range items {
		var name, color, teacher string
		switch v := item.(type) {
		case string:
			name = strings.TrimSpace(v)
		case map[string]any:
			name = strings.TrimSpace(toStr(v["name"]))
			color = strings.TrimSpace(toStr(v["color"]))
			teacher = strings.TrimSpace(toStr(v["teacher"]))
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if color == "" {
			if c, ok := presetColors[name]; ok {
				color = c
			} else {
				color = fallbackPalette[len(out)%len(fallbackPalette)]
			}
		}
		out = append(out, Subject{Name: name, Color: color, Teacher: teacher})
	}
	return out
}

func getSubjects(db *sql.DB) []Subject {
	v := getSetting(db, "subjects", "")
	if v != "" {
		if list := normalizeSubjects(json.RawMessage(v)); len(list) > 0 {
			return list
		}
	}
	raw, _ := json.Marshal(defaultSubjects)
	return normalizeSubjects(raw)
}

// 值班类条目：始终补在班级课程之后（缺哪个补哪个），供课程表显示值班教师用。
// 它们不进全局科目表，不会出现在成绩录入里。
var dutyDefaults = []Subject{
	{Name: "早读", Color: "#fdf6ec"},
	{Name: "晚修", Color: "#eef0fb"},
	{Name: "自习", Color: "#f5f6fa"},
	{Name: "班会", Color: "#f0faf5"},
}

func withDutyEntries(list []Subject) []Subject {
	out := append([]Subject{}, list...)
	for _, d := range dutyDefaults {
		found := false
		for _, s := range out {
			if s.Name == d.Name {
				found = true
				break
			}
		}
		if !found {
			out = append(out, d)
		}
	}
	return out
}

// 班级专属课程：有配置按班级的，没有则回退全局科目表
func getClassSubjects(db *sql.DB, className string) []Subject {
	rows, err := queryAll(db, `SELECT name, color, teacher FROM class_subjects WHERE class_name = ? ORDER BY rowid`, className)
	if err == nil && len(rows) > 0 {
		raw, _ := json.Marshal(rows)
		if list := normalizeSubjects(raw); len(list) > 0 {
			return withDutyEntries(list)
		}
	}
	return withDutyEntries(getSubjects(db))
}

func setClassSubjects(db *sql.DB, className string, raw json.RawMessage) []Subject {
	list := normalizeSubjects(raw)
	tx, err := db.Begin()
	if err == nil {
		tx.Exec(`DELETE FROM class_subjects WHERE class_name = ?`, className)
		for _, s := range list {
			tx.Exec(`INSERT INTO class_subjects(class_name, name, color, teacher) VALUES (?,?,?,?)`, className, s.Name, s.Color, s.Teacher)
		}
		tx.Commit()
	}
	return list
}

// 一次性迁移：给已有课程表补上「早读」（插到最前，格子索引整体后移）和两节「晚修」（追加到末尾）
func migratePeriods(db *sql.DB) {
	if getSetting(db, "mig_periods_v1", "") == "1" {
		return
	}
	rows, err := queryAll(db, `SELECT class_name, config FROM schedules`)
	if err == nil {
		touched := 0
		for _, row := range rows {
			var cfg map[string]any
			if json.Unmarshal([]byte(toStr(row["config"])), &cfg) != nil {
				continue
			}
			rawTimes, ok := cfg["times"].([]any)
			if !ok {
				continue
			}
			times := make([]string, 0, len(rawTimes)+3)
			for _, t := range rawTimes {
				times = append(times, toStr(t))
			}
			notes := []string{}
			if rawNotes, ok := cfg["timeNotes"].([]any); ok {
				for _, n := range rawNotes {
					notes = append(notes, toStr(n))
				}
			}
			for len(notes) < len(times) {
				notes = append(notes, "")
			}
			breaks := []any{}
			if rawBreaks, ok := cfg["breaksAfter"].([]any); ok {
				breaks = rawBreaks
			}
			hasMorning, hasEvening := false, false
			for _, t := range times {
				if strings.Contains(t, "早读") {
					hasMorning = true
				}
				if strings.Contains(t, "晚修") {
					hasEvening = true
				}
			}
			shift := 0
			changed := false
			if !hasMorning {
				times = append([]string{"早读"}, times...)
				notes = append([]string{"07:20-07:50"}, notes...)
				shift = 1
				changed = true
			}
			if !hasEvening {
				times = append(times, "晚修1", "晚修2")
				notes = append(notes, "19:00-19:45", "19:55-20:40")
				changed = true
			}
			if !changed {
				continue
			}
			if shift == 1 {
				cells, _ := cfg["cells"].(map[string]any)
				remapped := map[string]any{}
				for k, v := range cells {
					if idx := strings.LastIndex(k, "-"); idx > 0 {
						var p int
						if _, err := fmt.Sscan(k[idx+1:], &p); err == nil {
							remapped[k[:idx+1]+fmt.Sprintf("%d", p+1)] = v
							continue
						}
					}
					remapped[k] = v
				}
				cfg["cells"] = remapped
				newBreaks := []any{}
				for _, b := range breaks {
					newBreaks = append(newBreaks, toInt(b)+1)
				}
				cfg["breaksAfter"] = newBreaks
			}
			cfg["times"] = times
			cfg["timeNotes"] = notes
			updated, _ := json.Marshal(cfg)
			db.Exec(`UPDATE schedules SET config = ?, updated_at = datetime('now','localtime') WHERE class_name = ?`, string(updated), toStr(row["class_name"]))
			touched++
		}
		if touched > 0 {
			fmt.Printf("课程表迁移：%d 个班级已补上早读/晚修节次\n", touched)
		}
	}
	setSetting(db, "mig_periods_v1", "1")
}

// 一次性迁移：把课表里「早读 / 晚修」行的空格填满（早读轮排语文/英语，晚修排自习），值班教师在课程设置里维护
func migrateDutyCells(db *sql.DB) {
	if getSetting(db, "mig_duty_cells_v1", "") == "1" {
		return
	}
	rows, err := queryAll(db, `SELECT class_name, config FROM schedules`)
	if err == nil {
		touched := 0
		for _, row := range rows {
			var cfg map[string]any
			if json.Unmarshal([]byte(toStr(row["config"])), &cfg) != nil {
				continue
			}
			rawTimes, ok := cfg["times"].([]any)
			if !ok {
				continue
			}
			rawDays, ok := cfg["days"].([]any)
			if !ok {
				continue
			}
			cells, _ := cfg["cells"].(map[string]any)
			if cells == nil {
				cells = map[string]any{}
			}
			changed := false
			for i, t := range rawTimes {
				name := toStr(t)
				isMorning := strings.Contains(name, "早读")
				isEvening := strings.Contains(name, "晚修")
				if !isMorning && !isEvening {
					continue
				}
				for _, d := range rawDays {
					day := toInt(d)
					key := fmt.Sprintf("%d-%d", day, i)
					if v, ok := cells[key]; ok && toStr(v) != "" {
						continue
					}
					if isMorning {
						if day%2 == 0 {
							cells[key] = "语文"
						} else {
							cells[key] = "英语"
						}
					} else {
						cells[key] = "自习"
					}
					changed = true
				}
			}
			if !changed {
				continue
			}
			cfg["cells"] = cells
			updated, _ := json.Marshal(cfg)
			db.Exec(`UPDATE schedules SET config = ?, updated_at = datetime('now','localtime') WHERE class_name = ?`, string(updated), toStr(row["class_name"]))
			touched++
		}
		if touched > 0 {
			fmt.Printf("课表填充：%d 个班级的早读/晚修已排好\n", touched)
		}
	}
	setSetting(db, "mig_duty_cells_v1", "1")
}
