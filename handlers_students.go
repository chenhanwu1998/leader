package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

var studentFields = []string{"student_no", "name", "gender", "class_name", "birth_date", "phone", "parent_name", "parent_phone", "address", "status", "remark"}

func pickStudent(body map[string]any) map[string]any {
	obj := map[string]any{}
	for _, f := range studentFields {
		if v, ok := body[f]; ok {
			if v == nil {
				v = ""
			}
			obj[f] = v
		}
	}
	return obj
}

func registerStudentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/students", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sqlStr := "SELECT * FROM students WHERE 1=1"
		params := []any{}
		if v := q.Get("class_name"); v != "" {
			sqlStr += " AND class_name = ?"
			params = append(params, v)
		}
		if v := q.Get("status"); v != "" {
			sqlStr += " AND status = ?"
			params = append(params, v)
		}
		if v := q.Get("gender"); v != "" {
			sqlStr += " AND gender = ?"
			params = append(params, v)
		}
		if v := q.Get("keyword"); v != "" {
			sqlStr += " AND (name LIKE ? OR student_no LIKE ?)"
			params = append(params, "%"+v+"%", "%"+v+"%")
		}
		sqlStr += " ORDER BY class_name, CAST(student_no AS INTEGER), student_no, name"
		rows, err := queryAll(ledgerDB(r), sqlStr, params...)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	})

	mux.HandleFunc("GET /api/students/classes", func(w http.ResponseWriter, r *http.Request) {
		rows, err := queryAll(ledgerDB(r), `SELECT DISTINCT class_name AS c FROM students WHERE class_name != '' ORDER BY c`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, toStr(row["c"]))
		}
		writeJSON(w, 200, names)
	})

	mux.HandleFunc("POST /api/students", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		data := pickStudent(body)
		if toStr(data["name"]) == "" {
			fail(w, 400, "姓名不能为空")
			return
		}
		className := strings.TrimSpace(toStr(data["class_name"]))
		data["class_name"] = className
		if className != "" && !classExists(ledgerDB(r), className) {
			fail(w, 400, "班级「"+className+"」不存在，请先在「班级管理」中创建或使用批量导入")
			return
		}
		keys := []string{}
		args := []any{}
		for _, f := range studentFields {
			if v, ok := data[f]; ok {
				keys = append(keys, f)
				args = append(args, v)
			}
		}
		res, err := ledgerDB(r).Exec(`INSERT INTO students(`+strings.Join(keys, ",")+`) VALUES(`+strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")+`)`, args...)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, 200, map[string]any{"id": id})
	})

	mux.HandleFunc("POST /api/students/batch", func(w http.ResponseWriter, r *http.Request) {
		raw, err := decodeBodyRaw(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		var list []map[string]any
		if json.Unmarshal(raw, &list) != nil || len(list) == 0 {
			fail(w, 400, "没有可导入的数据")
			return
		}
		ok, failCnt := 0, 0
		errs := []string{}
		tx, err := ledgerDB(r).Begin()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer tx.Rollback()
		ins, err := tx.Prepare(`INSERT INTO students(student_no, name, gender, class_name, birth_date, phone, parent_name, parent_phone, address, status, remark) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer ins.Close()
		for i, row := range list {
			name := strings.TrimSpace(toStr(row["name"]))
			if name == "" {
				failCnt++
				errs = append(errs, "第"+strconvI(i+1)+"行：姓名为空")
				continue
			}
			gender := "男"
			if toStr(row["gender"]) == "女" {
				gender = "女"
			}
			className := strings.TrimSpace(toStr(row["class_name"]))
			status := toStr(row["status"])
			if _, ok := row["status"]; !ok || status == "" {
				status = "在读"
			}
			if _, err := ins.Exec(toStr(row["student_no"]), name, gender, className,
				toStr(row["birth_date"]), toStr(row["phone"]), toStr(row["parent_name"]), toStr(row["parent_phone"]),
				toStr(row["address"]), status, toStr(row["remark"])); err != nil {
				failCnt++
				errs = append(errs, "第"+strconvI(i+1)+"行："+err.Error())
				continue
			}
			ensureClass(tx, className) // 导入时自动登记新班级
			ok++
		}
		if err := tx.Commit(); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if len(errs) > 20 {
			errs = errs[:20]
		}
		writeJSON(w, 200, map[string]any{"ok": ok, "fail": failCnt, "errors": errs})
	})

	mux.HandleFunc("PUT /api/students/{id}", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		data := pickStudent(body)
		if _, ok := data["class_name"]; ok {
			className := strings.TrimSpace(toStr(data["class_name"]))
			data["class_name"] = className
			if className != "" && !classExists(ledgerDB(r), className) {
				fail(w, 400, "班级「"+className+"」不存在，请先在「班级管理」中创建")
				return
			}
		}
		if len(data) == 0 {
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		sets := []string{}
		args := []any{}
		for _, f := range studentFields {
			if v, ok := data[f]; ok {
				sets = append(sets, f+" = ?")
				args = append(args, v)
			}
		}
		args = append(args, r.PathValue("id"))
		if _, err := ledgerDB(r).Exec(`UPDATE students SET `+strings.Join(sets, ",")+`, updated_at = datetime('now','localtime') WHERE id = ?`, args...); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("DELETE /api/students/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := ledgerDB(r).Exec(`DELETE FROM students WHERE id = ?`, r.PathValue("id")); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("POST /api/students/delete-batch", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		ids, _ := body["ids"].([]any)
		tx, err := ledgerDB(r).Begin()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer tx.Rollback()
		del, err := tx.Prepare(`DELETE FROM students WHERE id = ?`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		for _, id := range ids {
			if _, err := del.Exec(id); err != nil {
				fail(w, 500, err.Error())
				del.Close()
				return
			}
		}
		del.Close()
		if err := tx.Commit(); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "count": len(ids)})
	})
}

func strconvI(i int) string {
	return strconv.Itoa(i)
}
