package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

var classFields = []string{"name", "grade", "head_teacher", "classroom", "remark"}

func registerClassRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/classes", func(w http.ResponseWriter, r *http.Request) {
		rows, err := queryAll(`
			SELECT c.*, (SELECT COUNT(*) FROM students s WHERE s.class_name = c.name AND s.status = '在读') AS student_count
			FROM classes c ORDER BY c.grade, c.name`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	})

	// 班级名下拉（班级表 ∪ 学生表中出现过的班级，兜底旧数据）
	mux.HandleFunc("GET /api/classes/names", func(w http.ResponseWriter, r *http.Request) {
		rows, err := queryAll(`
			SELECT name FROM classes
			UNION
			SELECT DISTINCT class_name AS name FROM students WHERE class_name != ''
			ORDER BY name`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		names := make([]string, 0, len(rows))
		for _, row := range rows {
			names = append(names, toStr(row["name"]))
		}
		writeJSON(w, 200, names)
	})

	// 班级专属课程设置（无班级配置时回退全局科目表）
	mux.HandleFunc("GET /api/classes/{className}/subjects", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, getClassSubjects(r.PathValue("className")))
	})

	mux.HandleFunc("PUT /api/classes/{className}/subjects", func(w http.ResponseWriter, r *http.Request) {
		className := strings.TrimSpace(r.PathValue("className"))
		if className == "" {
			fail(w, 400, "班级名称不能为空")
			return
		}
		raw, err := decodeBodyRaw(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		if len(normalizeSubjects(raw)) == 0 {
			fail(w, 400, "至少保留一个科目")
			return
		}
		if tx, err := db.Begin(); err == nil {
			ensureClass(tx, className)
			tx.Commit()
		}
		writeJSON(w, 200, setClassSubjects(className, raw))
	})

	mux.HandleFunc("POST /api/classes", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		name := strings.TrimSpace(strField(body, "name", ""))
		if name == "" {
			fail(w, 400, "班级名称不能为空")
			return
		}
		if queryOne(`SELECT id FROM classes WHERE name = ?`, name) != nil {
			fail(w, 400, "班级「"+name+"」已存在")
			return
		}
		res, err := db.Exec(`INSERT INTO classes(name, grade, head_teacher, classroom, remark) VALUES (?,?,?,?,?)`,
			name, strField(body, "grade", ""), strField(body, "head_teacher", ""), strField(body, "classroom", ""), strField(body, "remark", ""))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, 200, map[string]any{"id": id})
	})

	mux.HandleFunc("PUT /api/classes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		row := queryOne(`SELECT * FROM classes WHERE id = ?`, id)
		if row == nil {
			fail(w, 404, "班级不存在")
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		data := map[string]string{}
		for _, f := range classFields {
			if v, ok := body[f]; ok {
				data[f] = toStr(v)
			}
		}
		if newName, ok := data["name"]; ok {
			newName = strings.TrimSpace(newName)
			if newName == "" {
				fail(w, 400, "班级名称不能为空")
				return
			}
			if queryOne(`SELECT id FROM classes WHERE name = ? AND id != ?`, newName, id) != nil {
				fail(w, 400, "班级「"+newName+"」已存在")
				return
			}
			data["name"] = newName
		}
		if len(data) == 0 {
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer tx.Rollback()
		sets := []string{}
		args := []any{}
		for _, f := range classFields { // 固定顺序，避免 map 遍历随机性
			if v, ok := data[f]; ok {
				sets = append(sets, f+" = ?")
				args = append(args, v)
			}
		}
		args = append(args, id)
		if _, err := tx.Exec(`UPDATE classes SET `+strings.Join(sets, ",")+` WHERE id = ?`, args...); err != nil {
			fail(w, 500, err.Error())
			return
		}
		oldName := toStr(row["name"])
		renamed := false
		if newName, ok := data["name"]; ok && newName != oldName {
			renamed = true
			// 改名联动：学生、座位表、课程表、考试上的班级名一起改
			for _, q := range [][2]string{
				{`UPDATE students SET class_name = ? WHERE class_name = ?`, ""},
				{`UPDATE seat_layouts SET class_name = ? WHERE class_name = ?`, ""},
				{`UPDATE schedules SET class_name = ? WHERE class_name = ?`, ""},
				{`UPDATE exams SET class_name = ? WHERE class_name = ?`, ""},
				{`UPDATE class_subjects SET class_name = ? WHERE class_name = ?`, ""},
			} {
				if _, err := tx.Exec(q[0], newName, oldName); err != nil {
					fail(w, 500, err.Error())
					return
				}
			}
		}
		if err := tx.Commit(); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "renamed": renamed})
	})

	mux.HandleFunc("DELETE /api/classes/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		row := queryOne(`SELECT * FROM classes WHERE id = ?`, id)
		if row == nil {
			fail(w, 404, "班级不存在")
			return
		}
		name := toStr(row["name"])
		if cnt := queryCount(`SELECT COUNT(*) c FROM students WHERE class_name = ?`, name); cnt > 0 {
			fail(w, 400, "班级「"+name+"」下还有 "+strconv.FormatInt(cnt, 10)+" 名学生，请先将学生调班或删除后再删除班级")
			return
		}
		tx, err := db.Begin()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`DELETE FROM classes WHERE id = ?`, id); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if _, err := tx.Exec(`DELETE FROM seat_layouts WHERE class_name = ?`, name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if _, err := tx.Exec(`DELETE FROM schedules WHERE class_name = ?`, name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if _, err := tx.Exec(`DELETE FROM class_subjects WHERE class_name = ?`, name); err != nil {
			fail(w, 500, err.Error())
			return
		}
		if err := tx.Commit(); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})
}

// ensureClass 导入学生时自动登记新班级
func ensureClass(tx *sql.Tx, name string) {
	n := strings.TrimSpace(name)
	if n == "" {
		return
	}
	tx.Exec(`INSERT OR IGNORE INTO classes(name) VALUES (?)`, n)
}

func classExists(name string) bool {
	if name == "" {
		return true // 允许不填班级
	}
	return queryOne(`SELECT id FROM classes WHERE name = ?`, name) != nil
}
