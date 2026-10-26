package main

import (
	"net/http"
	"strings"
)

// 请假 / 奖惩 / 家校联系 三个结构相近的通用 CRUD（与 leader-sys records.js 一致）
type recordConf struct {
	table      string
	fields     []string
	defaults   map[string]any
	orderExpr  string
	filters    [][2]string // {queryParam, column}
	dateColumn string
}

// recordFields 给 POST 用的字段求值：body[f] ?? defaults[f] ?? ''
func (c *recordConf) resolve(body map[string]any) map[string]any {
	obj := map[string]any{}
	for _, f := range c.fields {
		if v, ok := body[f]; ok && v != nil {
			obj[f] = v
		} else if d, ok := c.defaults[f]; ok {
			obj[f] = d
		} else {
			obj[f] = ""
		}
	}
	return obj
}

func registerRecordRoutes(mux *http.ServeMux, prefix string, c recordConf) {
	mux.HandleFunc("GET "+prefix, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sqlStr := `SELECT t.*, st.name AS student_name, st.class_name AS class_name, st.student_no
		           FROM ` + c.table + ` t JOIN students st ON st.id = t.student_id WHERE 1=1`
		params := []any{}
		if v := q.Get("student_id"); v != "" {
			sqlStr += " AND t.student_id = ?"
			params = append(params, v)
		}
		if v := q.Get("class_name"); v != "" {
			sqlStr += " AND st.class_name = ?"
			params = append(params, v)
		}
		if v := q.Get("keyword"); v != "" {
			sqlStr += " AND (st.name LIKE ? OR t.remark LIKE ?)"
			params = append(params, "%"+v+"%", "%"+v+"%")
		}
		for _, f := range c.filters {
			if v := q.Get(f[0]); v != "" {
				sqlStr += " AND " + f[1] + " = ?"
				params = append(params, v)
			}
		}
		if c.dateColumn != "" {
			if v := q.Get("date_from"); v != "" {
				sqlStr += " AND " + c.dateColumn + " >= ?"
				params = append(params, v)
			}
			if v := q.Get("date_to"); v != "" {
				sqlStr += " AND " + c.dateColumn + " <= ?"
				params = append(params, v)
			}
		}
		order := c.orderExpr
		if order == "" {
			order = "t.id DESC"
		}
		sqlStr += " ORDER BY " + order
		rows, err := queryAll(ledgerDB(r), sqlStr, params...)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	})

	mux.HandleFunc("POST "+prefix, func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		studentID := toInt(body["student_id"])
		if studentID == 0 {
			fail(w, 400, "请选择学生")
			return
		}
		obj := c.resolve(body)
		keys := []string{"student_id"}
		args := []any{studentID}
		for _, f := range c.fields {
			if v, ok := obj[f]; ok {
				keys = append(keys, f)
				args = append(args, v)
			}
		}
		res, err := ledgerDB(r).Exec(`INSERT INTO `+c.table+`(`+strings.Join(keys, ",")+`) VALUES (`+strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")+`)`, args...)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, 200, map[string]any{"id": id})
	})

	mux.HandleFunc("PUT "+prefix+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		list := []string{}
		args := []any{}
		for _, f := range c.fields {
			if v, ok := body[f]; ok {
				list = append(list, f)
				if v == nil {
					v = ""
				}
				args = append(args, v)
			}
		}
		if sid := toInt(body["student_id"]); sid != 0 {
			list = append(list, "student_id")
			args = append(args, sid)
		}
		if len(list) == 0 {
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		sets := make([]string, len(list))
		for i, k := range list {
			sets[i] = k + " = ?"
		}
		args = append(args, r.PathValue("id"))
		if _, err := ledgerDB(r).Exec(`UPDATE `+c.table+` SET `+strings.Join(sets, ",")+` WHERE id = ?`, args...); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("DELETE "+prefix+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := ledgerDB(r).Exec(`DELETE FROM `+c.table+` WHERE id = ?`, r.PathValue("id")); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("POST "+prefix+"/batch", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		list, _ := body["records"].([]any)
		okCnt, failCnt := 0, 0
		errs := []string{}
		tx, err := ledgerDB(r).Begin()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer tx.Rollback()
		findStu, err := tx.Prepare(`SELECT id FROM students WHERE name = ? AND (class_name = ? OR ? = '')`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		ins, err := tx.Prepare(`INSERT INTO ` + c.table + `(student_id, ` + strings.Join(c.fields, ",") + `) VALUES (?, ` + strings.TrimSuffix(strings.Repeat("?,", len(c.fields)), ",") + `)`)
		if err != nil {
			findStu.Close()
			fail(w, 500, err.Error())
			return
		}
		for i, rec := range list {
			rm, _ := rec.(map[string]any)
			if rm == nil {
				failCnt++
				errs = append(errs, "第"+strconvI(i+1)+"行：数据格式错误")
				continue
			}
			stuName := strings.TrimSpace(toStr(rm["student_name"]))
			className := toStr(rm["class_name"])
			var stuID int64
			row := findStu.QueryRow(stuName, className, className)
			if row.Scan(&stuID) != nil {
				failCnt++
				errs = append(errs, "第"+strconvI(i+1)+"行：找不到学生「"+stuName+"」")
				continue
			}
			args := []any{stuID}
			for _, f := range c.fields {
				if v, ok := rm[f]; ok && v != nil {
					args = append(args, v)
				} else if d, ok := c.defaults[f]; ok {
					args = append(args, d)
				} else {
					args = append(args, "")
				}
			}
			if _, err := ins.Exec(args...); err != nil {
				failCnt++
				errs = append(errs, "第"+strconvI(i+1)+"行："+err.Error())
				continue
			}
			okCnt++
		}
		findStu.Close()
		ins.Close()
		if err := tx.Commit(); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if len(errs) > 20 {
			errs = errs[:20]
		}
		writeJSON(w, 200, map[string]any{"ok": okCnt, "fail": failCnt, "errors": errs})
	})
}

func registerAllRecordRoutes(mux *http.ServeMux) {
	registerRecordRoutes(mux, "/api/leaves", recordConf{
		table:      "leaves",
		fields:     []string{"type", "start_date", "end_date", "days", "reason", "status", "remark"},
		defaults:   map[string]any{"type": "事假", "status": "已销假", "days": 1},
		orderExpr:  "t.start_date DESC, t.id DESC",
		filters:    [][2]string{{"type", "t.type"}, {"status", "t.status"}},
		dateColumn: "t.start_date",
	})
	registerRecordRoutes(mux, "/api/rewards", recordConf{
		table:      "rewards",
		fields:     []string{"kind", "level", "title", "record_date", "reason", "remark"},
		defaults:   map[string]any{"kind": "表扬", "level": "班级"},
		orderExpr:  "t.record_date DESC, t.id DESC",
		filters:    [][2]string{{"kind", "t.kind"}, {"level", "t.level"}},
		dateColumn: "t.record_date",
	})
	registerRecordRoutes(mux, "/api/contacts", recordConf{
		table:      "contacts",
		fields:     []string{"contact_date", "way", "target", "content", "feedback", "remark"},
		defaults:   map[string]any{"way": "电话", "target": "家长"},
		orderExpr:  "t.contact_date DESC, t.id DESC",
		filters:    [][2]string{{"way", "t.way"}, {"target", "t.target"}},
		dateColumn: "t.contact_date",
	})
}
