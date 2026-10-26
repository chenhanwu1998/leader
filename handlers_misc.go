package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func registerMiscRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "app": "teacher-ledger", "time": time.Now().Format(time.RFC3339)})
	})

	// 全局设置（科目列表等）
	mux.HandleFunc("GET /api/settings/subjects", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, getSubjects(ledgerDB(r)))
	})
	mux.HandleFunc("PUT /api/settings/subjects", func(w http.ResponseWriter, r *http.Request) {
		raw, err := decodeBodyRaw(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		var arr json.RawMessage
		if json.Unmarshal(raw, &arr) != nil || len(raw) == 0 || raw[0] != '[' {
			arr = json.RawMessage("[]")
		}
		list := normalizeSubjects(arr)
		out, _ := json.Marshal(list)
		setSetting(ledgerDB(r), "subjects", string(out))
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("GET /api/settings/{key}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"value": getSetting(ledgerDB(r), r.PathValue("key"), "")})
	})
	mux.HandleFunc("PUT /api/settings/{key}", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		setSetting(ledgerDB(r), r.PathValue("key"), strField(body, "value", ""))
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// 全量数据备份（仅管理员）：账号库 + 各账号台账库 + 智能体配置，统一打 zip
	mux.HandleFunc("GET /api/backup", func(w http.ResponseWriter, r *http.Request) {
		if requireAdmin(w, r) == nil {
			return
		}
		buf, err := buildFullBackup()
		if err != nil {
			fail(w, 500, "备份失败："+err.Error())
			return
		}
		name := "teacher-ledger-backup-" + time.Now().Format("2006-01-02") + ".zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"; filename*=UTF-8''`+url.PathEscape(name))
		w.Write(buf)
	})

	// 清空全部业务数据（保留设置）
	mux.HandleFunc("POST /api/reset", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil || strField(body, "confirm", "") != "确认清空" {
			fail(w, 400, "请输入「确认清空」")
			return
		}
		tx, err := ledgerDB(r).Begin()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer tx.Rollback()
		if err := clearAllTables(tx); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if err := tx.Commit(); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// 导出中转：WebView 内无法直接用 blob 下载，改为「上传字节→返回下载链接→导航触发系统下载」
	mux.HandleFunc("POST /api/export/save", handleExportSave)
	mux.HandleFunc("GET /api/export/file/{id}", handleExportFile)
}

// ---------- 导出中转存储 ----------

type exportEntry struct {
	data    []byte
	name    string
	mime    string
	created time.Time
}

var exportStore struct {
	sync.Mutex
	m map[string]*exportEntry
}

func handleExportSave(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 30<<20))
	if err != nil || len(data) == 0 {
		fail(w, 400, "文件内容为空")
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "export.xlsx"
	}
	name = filepath.Base(name) // 防路径注入
	mime := "application/octet-stream"
	switch {
	case strings.HasSuffix(strings.ToLower(name), ".xlsx"):
		mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case strings.HasSuffix(strings.ToLower(name), ".csv"):
		mime = "text/csv; charset=utf-8"
	case strings.HasSuffix(strings.ToLower(name), ".zip"):
		mime = "application/zip"
	case strings.HasSuffix(strings.ToLower(name), ".docx"):
		mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	var idBytes [12]byte
	rand.Read(idBytes[:])
	id := hex.EncodeToString(idBytes[:])

	exportStore.Lock()
	if exportStore.m == nil {
		exportStore.m = map[string]*exportEntry{}
	}
	// 顺手清理 10 分钟前的临时文件
	for k, e := range exportStore.m {
		if time.Since(e.created) > 10*time.Minute {
			delete(exportStore.m, k)
		}
	}
	exportStore.m[id] = &exportEntry{data: data, name: name, mime: mime, created: time.Now()}
	exportStore.Unlock()

	writeJSON(w, 200, map[string]any{"url": "/api/export/file/" + id})
}

func handleExportFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	exportStore.Lock()
	e := exportStore.m[id]
	delete(exportStore.m, id) // 一次性下载
	exportStore.Unlock()
	if e == nil {
		fail(w, 404, "文件不存在或已过期")
		return
	}
	w.Header().Set("Content-Type", e.mime)
	w.Header().Set("Content-Disposition", `attachment; filename="export"; filename*=UTF-8''`+url.PathEscape(e.name))
	w.Write(e.data)
}

