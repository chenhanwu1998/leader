package main

import (
	"net/http"
	"sort"
)

func registerScoreRoutes(mux *http.ServeMux) {
	// ---------- 考试 ----------
	mux.HandleFunc("GET /api/scores/exams", func(w http.ResponseWriter, r *http.Request) {
		rows, err := queryAll(`SELECT * FROM exams ORDER BY exam_date DESC, id DESC`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	})

	mux.HandleFunc("POST /api/scores/exams", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		name := strField(body, "name", "")
		if name == "" {
			fail(w, 400, "考试名称不能为空")
			return
		}
		res, err := db.Exec(`INSERT INTO exams(name, exam_date, term, type, class_name, remark) VALUES (?,?,?,?,?,?)`,
			name, strField(body, "exam_date", ""), strField(body, "term", ""), strField(body, "type", "期中"),
			strField(body, "class_name", ""), strField(body, "remark", ""))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, 200, map[string]any{"id": id})
	})

	mux.HandleFunc("PUT /api/scores/exams/{id}", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		if _, err := db.Exec(`UPDATE exams SET name=?, exam_date=?, term=?, type=?, class_name=?, remark=? WHERE id=?`,
			strField(body, "name", ""), strField(body, "exam_date", ""), strField(body, "term", ""), strField(body, "type", "期中"),
			strField(body, "class_name", ""), strField(body, "remark", ""), r.PathValue("id")); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("DELETE /api/scores/exams/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.Exec(`DELETE FROM exams WHERE id = ?`, r.PathValue("id")); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// ---------- 成绩 ----------
	mux.HandleFunc("GET /api/scores", func(w http.ResponseWriter, r *http.Request) {
		examID := r.URL.Query().Get("exam_id")
		if examID == "" {
			writeJSON(w, 200, []any{})
			return
		}
		rows, err := queryAll(`
			SELECT s.id, s.exam_id, s.student_id, s.subject, s.score, s.full_score,
			       st.name AS student_name, st.student_no, st.class_name
			FROM scores s JOIN students st ON st.id = s.student_id
			WHERE s.exam_id = ?
			ORDER BY CAST(st.student_no AS INTEGER), st.student_no, st.name, s.id
		`, examID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	})

	// 成绩矩阵：按学生聚合 { student_id, name, scores: {subject: score}, total, rank }
	mux.HandleFunc("GET /api/scores/matrix", func(w http.ResponseWriter, r *http.Request) {
		examID := r.URL.Query().Get("exam_id")
		if examID == "" {
			writeJSON(w, 200, map[string]any{"subjects": []any{}, "rows": []any{}})
			return
		}
		subRows, err := queryAll(`SELECT DISTINCT subject FROM scores WHERE exam_id = ? ORDER BY id`, examID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		subjects := make([]string, 0, len(subRows))
		for _, row := range subRows {
			subjects = append(subjects, toStr(row["subject"]))
		}
		rows, err := queryAll(`
			SELECT s.student_id, st.name, st.student_no, st.class_name, s.subject, s.score
			FROM scores s JOIN students st ON st.id = s.student_id
			WHERE s.exam_id = ?
			ORDER BY CAST(st.student_no AS INTEGER), st.student_no, st.name
		`, examID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		type entry struct {
			m     map[string]any
			total float64
		}
		order := []int64{}
		byID := map[int64]*entry{}
		for _, row := range rows {
			sid := toInt(row["student_id"])
			e, ok := byID[sid]
			if !ok {
				e = &entry{m: map[string]any{
					"student_id": sid,
					"name":       row["name"],
					"student_no": row["student_no"],
					"class_name": row["class_name"],
					"scores":     map[string]any{},
					"total":      float64(0),
				}}
				byID[sid] = e
				order = append(order, sid)
			}
			e.m["scores"].(map[string]any)[toStr(row["subject"])] = row["score"]
		}
		list := make([]*entry, 0, len(order))
		for _, sid := range order {
			e := byID[sid]
			total := 0.0
			scores := e.m["scores"].(map[string]any)
			for _, sub := range subjects {
				total += toFloat(scores[sub])
			}
			e.total = total
			e.m["total"] = total
			list = append(list, e)
		}
		sort.SliceStable(list, func(i, j int) bool { return list[i].total > list[j].total })
		out := make([]map[string]any, 0, len(list))
		for i, e := range list {
			e.m["rank"] = i + 1
			out = append(out, e.m)
		}
		writeJSON(w, 200, map[string]any{"subjects": subjects, "rows": out})
	})

	// 批量保存某场考试成绩：[{student_id, subject, score, full_score}]
	mux.HandleFunc("POST /api/scores/bulk", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		examID := toInt(body["exam_id"])
		records, ok := body["records"].([]any)
		if examID == 0 || !ok {
			fail(w, 400, "参数错误")
			return
		}
		tx, err := db.Begin()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		defer tx.Rollback()
		upsert, err := tx.Prepare(`
			INSERT INTO scores(exam_id, student_id, subject, score, full_score)
			VALUES (?,?,?,?,?)
			ON CONFLICT(exam_id, student_id, subject) DO UPDATE SET score = excluded.score, full_score = excluded.full_score, updated_at = datetime('now','localtime')
		`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		for _, rec := range records {
			rm, _ := rec.(map[string]any)
			if rm == nil {
				continue
			}
			sv, exists := rm["score"]
			if !exists || sv == nil || sv == "" {
				continue
			}
			full := 100.0
			if fv, ok := rm["full_score"]; ok && fv != nil && fv != "" {
				full = toFloat(fv)
			}
			if _, err := upsert.Exec(examID, toInt(rm["student_id"]), toStr(rm["subject"]), toFloat(sv), full); err != nil {
				upsert.Close()
				fail(w, 500, err.Error())
				return
			}
		}
		upsert.Close()
		if err := tx.Commit(); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// 删除某学生某场考试的全部成绩
	mux.HandleFunc("DELETE /api/scores/entry/{examId}/{studentId}", func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.Exec(`DELETE FROM scores WHERE exam_id = ? AND student_id = ?`, r.PathValue("examId"), r.PathValue("studentId")); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})
}

