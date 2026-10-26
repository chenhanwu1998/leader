package main

// 全量备份：账号库 users.db + 各账号台账库 users/*.db + 智能体配置，打成单个 zip（仅管理员可下载）

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// 把 WAL 内容合并回主库文件（WAL 允许多连接并发，临时打开一个连接做 checkpoint 即可）
func checkpointLedger(file string) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", filepath.ToSlash(file))
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return
	}
	defer d.Close()
	row := d.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`)
	var a, b, c int
	row.Scan(&a, &b, &c)
}

func buildFullBackup() ([]byte, error) {
	type entry struct {
		arc  string
		file string
	}
	files := []entry{}

	// 账号/会话库：先 checkpoint
	if udb != nil {
		row := udb.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`)
		var a, b, c int
		row.Scan(&a, &b, &c)
		files = append(files, entry{"users.db", filepath.Join(baseDataDir, "users.db")})
	}

	// 所有账号的台账库
	userDBs, _ := filepath.Glob(filepath.Join(baseDataDir, "users", "*.db"))
	sort.Strings(userDBs)
	for _, f := range userDBs {
		checkpointLedger(f)
		files = append(files, entry{"users/" + filepath.Base(f), f})
	}

	// 智能体配置（密钥与知识库绑定信息，用于换机恢复）
	for _, extra := range []string{"aihub-key.txt", "aihub-kb.json"} {
		p := filepath.Join(baseDataDir, extra)
		if _, err := os.Stat(p); err == nil {
			files = append(files, entry{extra, p})
		}
	}

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, e := range files {
		data, err := os.ReadFile(e.file)
		if err != nil {
			continue // 文件瞬间被占/被删时跳过该文件，不拖垮整个备份
		}
		w, err := zw.Create(e.arc)
		if err != nil {
			zw.Close()
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if buf.Len() == 0 {
		return nil, fmt.Errorf("没有可备份的数据文件")
	}
	return buf.Bytes(), nil
}
