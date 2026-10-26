package main

// 心理健康板块：重点关注档案 / 辅导记录 / 量表测评 / 跟进提醒
// 与 leader-sys 的 server/routes/psy.js 一一对应

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

var riskLevels = map[string]bool{"常规关注": true, "重点关注": true, "高危预警": true}

func registerPsyRoutes(mux *http.ServeMux) {
	// —— 量表题库（前端渲染问卷用，计分在服务端做）——
	mux.HandleFunc("GET /api/psy/scales", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, scaleNames())
	})

	// —— 档案 ——
	mux.HandleFunc("GET /api/psy/profiles", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		q := r.URL.Query()
		sqlStr := `SELECT p.*,
      (SELECT COUNT(*) FROM psy_sessions s WHERE s.profile_id = p.id) AS session_count,
      (SELECT MAX(created_at) FROM psy_assessments a WHERE a.profile_id = p.id) AS last_assessment
    FROM psy_profiles p WHERE 1=1`
		var params []any
		if v := q.Get("class_name"); v != "" {
			sqlStr += " AND p.class_name = ?"
			params = append(params, v)
		}
		if v := q.Get("risk_level"); v != "" {
			sqlStr += " AND p.risk_level = ?"
			params = append(params, v)
		}
		if v := q.Get("keyword"); v != "" {
			sqlStr += " AND (p.student_name LIKE ? OR p.tags LIKE ?)"
			params = append(params, "%"+v+"%", "%"+v+"%")
		}
		sqlStr += ` ORDER BY CASE p.risk_level WHEN '高危预警' THEN 0 WHEN '重点关注' THEN 1 ELSE 2 END, p.updated_at DESC`
		rows, err := queryAll(d, sqlStr, params...)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, rows)
	})

	mux.HandleFunc("POST /api/psy/profiles", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		p := pickProfile(body)
		if p["student_name"] == "" {
			fail(w, 400, "请填写学生姓名")
			return
		}
		risk := p["risk_level"]
		if risk == "" {
			risk = "常规关注"
		}
		res, err := d.Exec(`INSERT INTO psy_profiles(student_name, class_name, gender, risk_level, tags, note) VALUES (?,?,?,?,?,?)`,
			p["student_name"], p["class_name"], p["gender"], risk, p["tags"], p["note"])
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, 200, queryOne(d, `SELECT * FROM psy_profiles WHERE id = ?`, id))
	})

	mux.HandleFunc("GET /api/psy/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		id := r.PathValue("id")
		profile := queryOne(d, `SELECT * FROM psy_profiles WHERE id = ?`, id)
		if profile == nil {
			fail(w, 404, "档案不存在")
			return
		}
		sessions, _ := queryAll(d, `SELECT * FROM psy_sessions WHERE profile_id = ? ORDER BY session_date DESC, id DESC`, id)
		assessments, _ := queryAll(d, `SELECT * FROM psy_assessments WHERE profile_id = ? ORDER BY created_at DESC, id DESC`, id)
		for _, a := range assessments {
			var parsed []any
			if json.Unmarshal([]byte(toStr(a["answers"])), &parsed) == nil && parsed != nil {
				a["answers"] = parsed
			} else {
				a["answers"] = []any{}
			}
		}
		for k, v := range map[string]any{"sessions": sessions, "assessments": assessments} {
			profile[k] = v
		}
		writeJSON(w, 200, profile)
	})

	mux.HandleFunc("PUT /api/psy/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		id := r.PathValue("id")
		if queryOne(d, `SELECT 1 AS x FROM psy_profiles WHERE id = ?`, id) == nil {
			fail(w, 404, "档案不存在")
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		p := pickProfile(body)
		if len(p) > 0 {
			var sets []string
			var params []any
			for _, f := range []string{"student_name", "class_name", "gender", "risk_level", "tags", "note"} {
				if v, ok := p[f]; ok {
					sets = append(sets, f+" = ?")
					params = append(params, v)
				}
			}
			if len(sets) > 0 {
				params = append(params, id)
				if _, err := d.Exec(`UPDATE psy_profiles SET `+strings.Join(sets, ", ")+`, updated_at = datetime('now','localtime') WHERE id = ?`, params...); err != nil {
					fail(w, 500, err.Error())
					return
				}
			}
		}
		writeJSON(w, 200, queryOne(d, `SELECT * FROM psy_profiles WHERE id = ?`, id))
	})

	mux.HandleFunc("DELETE /api/psy/profiles/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		d.Exec(`DELETE FROM psy_profiles WHERE id = ?`, r.PathValue("id")) // 记录与测评级联删除
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// —— 辅导记录 ——
	mux.HandleFunc("POST /api/psy/profiles/{id}/sessions", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		id := r.PathValue("id")
		if queryOne(d, `SELECT 1 AS x FROM psy_profiles WHERE id = ?`, id) == nil {
			fail(w, 404, "档案不存在")
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		s := pickSession(body)
		date := s["session_date"]
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		mode := s["mode"]
		if mode == "" {
			mode = "面谈"
		}
		res, err := d.Exec(`INSERT INTO psy_sessions(profile_id, session_date, mode, topic, content, next_follow) VALUES (?,?,?,?,?,?)`,
			id, date, mode, s["topic"], s["content"], s["next_follow"])
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		sid, _ := res.LastInsertId()
		writeJSON(w, 200, queryOne(d, `SELECT * FROM psy_sessions WHERE id = ?`, sid))
	})

	mux.HandleFunc("PUT /api/psy/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		id := r.PathValue("id")
		if queryOne(d, `SELECT 1 AS x FROM psy_sessions WHERE id = ?`, id) == nil {
			fail(w, 404, "记录不存在")
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		s := pickSession(body)
		var sets []string
		var params []any
		for _, f := range []string{"session_date", "mode", "topic", "content", "next_follow"} {
			if v, ok := s[f]; ok {
				sets = append(sets, f+" = ?")
				params = append(params, v)
			}
		}
		if len(sets) > 0 {
			params = append(params, id)
			if _, err := d.Exec(`UPDATE psy_sessions SET `+strings.Join(sets, ", ")+` WHERE id = ?`, params...); err != nil {
				fail(w, 500, err.Error())
				return
			}
		}
		writeJSON(w, 200, queryOne(d, `SELECT * FROM psy_sessions WHERE id = ?`, id))
	})

	mux.HandleFunc("DELETE /api/psy/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		d.Exec(`DELETE FROM psy_sessions WHERE id = ?`, r.PathValue("id"))
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// —— 量表测评（服务端计分）——
	mux.HandleFunc("POST /api/psy/profiles/{id}/assessments", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		id := r.PathValue("id")
		if queryOne(d, `SELECT 1 AS x FROM psy_profiles WHERE id = ?`, id) == nil {
			fail(w, 404, "档案不存在")
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		scale := strField(body, "scale", "")
		var answers []float64
		if raw, ok := body["answers"].([]any); ok {
			for _, v := range raw {
				answers = append(answers, toFloat(v))
			}
		}
		score, level, flagged, err := scoreAssessment(scale, answers)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		answersJSON, _ := json.Marshal(answers)
		flagInt := 0
		if flagged {
			flagInt = 1
		}
		res, err := d.Exec(`INSERT INTO psy_assessments(profile_id, scale, answers, score, level, flagged) VALUES (?,?,?,?,?,?)`,
			id, scale, string(answersJSON), score, level, flagInt)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		aid, _ := res.LastInsertId()
		writeJSON(w, 200, queryOne(d, `SELECT * FROM psy_assessments WHERE id = ?`, aid))
	})

	mux.HandleFunc("DELETE /api/psy/assessments/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		d.Exec(`DELETE FROM psy_assessments WHERE id = ?`, r.PathValue("id"))
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// —— 跟进提醒：未来 7 天内到期 + 已逾期的跟进计划 ——
	mux.HandleFunc("GET /api/psy/followups", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		rows, err := queryAll(d,
			`SELECT s.id, s.session_date, s.mode, s.topic, s.next_follow, p.id AS profile_id, p.student_name, p.class_name, p.risk_level
       FROM psy_sessions s JOIN psy_profiles p ON p.id = s.profile_id
       WHERE s.next_follow != '' AND date(s.next_follow) <= date('now','localtime','+7 day')
       ORDER BY date(s.next_follow) ASC`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		today := time.Now().Format("2006-01-02")
		for _, row := range rows {
			row["overdue"] = toStr(row["next_follow"]) < today
		}
		writeJSON(w, 200, rows)
	})

	// —— 板块概览统计 ——
	mux.HandleFunc("GET /api/psy/stats", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		byRisk, _ := queryAll(d, `SELECT risk_level, COUNT(*) n FROM psy_profiles GROUP BY risk_level`)
		recent, _ := queryAll(d, `SELECT a.*, p.student_name, p.class_name FROM psy_assessments a JOIN psy_profiles p ON p.id = a.profile_id ORDER BY a.created_at DESC LIMIT 10`)
		flagged := queryCount(d, `SELECT COUNT(*) n FROM psy_assessments WHERE flagged = 1`)
		thisMonth := queryCount(d, `SELECT COUNT(*) n FROM psy_sessions WHERE strftime('%Y-%m', session_date) = strftime('%Y-%m', 'now','localtime')`)
		writeJSON(w, 200, map[string]any{
			"by_risk": byRisk, "recent_assessments": recent,
			"flagged": flagged, "sessions_this_month": thisMonth,
		})
	})
}

func pickProfile(body map[string]any) map[string]string {
	obj := map[string]string{}
	for _, f := range []string{"student_name", "class_name", "gender", "risk_level", "tags", "note"} {
		if v, ok := body[f]; ok && v != nil {
			obj[f] = strings.TrimSpace(toStr(v))
		}
	}
	if rl, ok := obj["risk_level"]; ok && !riskLevels[rl] {
		delete(obj, "risk_level")
	}
	return obj
}

func pickSession(body map[string]any) map[string]string {
	obj := map[string]string{}
	for _, f := range []string{"session_date", "mode", "topic", "content", "next_follow"} {
		if v, ok := body[f]; ok && v != nil {
			obj[f] = strings.TrimSpace(toStr(v))
		}
	}
	return obj
}

