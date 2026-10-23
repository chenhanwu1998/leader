package main

import (
	"math"
	"net/http"
	"strconv"
)

func registerStatsRoutes(mux *http.ServeMux) {
	// ---------- 总览 ----------
	mux.HandleFunc("GET /api/stats/overview", func(w http.ResponseWriter, r *http.Request) {
		recentLeaves, err := queryAll(`
			SELECT l.*, st.name AS student_name, st.class_name FROM leaves l JOIN students st ON st.id = l.student_id
			ORDER BY l.start_date DESC, l.id DESC LIMIT 8`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		genderDist, _ := queryAll(`SELECT gender k, COUNT(*) v FROM students WHERE status='在读' GROUP BY gender`)
		classDist, _ := queryAll(`SELECT class_name k, COUNT(*) v FROM students WHERE status='在读' AND class_name != '' GROUP BY class_name ORDER BY class_name`)
		leaveTrend, _ := queryAll(`
			SELECT strftime('%Y-%m', start_date) k, COUNT(*) v FROM leaves
			WHERE start_date >= date('now','localtime','-11 months','start of month')
			GROUP BY k ORDER BY k`)
		writeJSON(w, 200, map[string]any{
			"students":      queryCount(`SELECT COUNT(*) c FROM students WHERE status = '在读'`),
			"classes":       queryCount(`SELECT COUNT(DISTINCT class_name) c FROM students WHERE class_name != ''`),
			"exams":         queryCount(`SELECT COUNT(*) c FROM exams`),
			"monthLeaves":   queryCount(`SELECT COUNT(*) c FROM leaves WHERE strftime('%Y-%m', start_date) = strftime('%Y-%m','now','localtime')`),
			"pendingLeaves": queryCount(`SELECT COUNT(*) c FROM leaves WHERE status != '已销假'`),
			"rewards":       queryCount(`SELECT COUNT(*) c FROM rewards WHERE kind IN ('表扬','奖励')`),
			"recentLeaves":  recentLeaves,
			"genderDist":    genderDist,
			"classDist":     classDist,
			"leaveTrend":    leaveTrend,
		})
	})

	// ---------- 学生统计 ----------
	mux.HandleFunc("GET /api/stats/students", func(w http.ResponseWriter, r *http.Request) {
		gender, _ := queryAll(`SELECT gender k, COUNT(*) v FROM students GROUP BY gender`)
		byClass, _ := queryAll(`SELECT class_name k, COUNT(*) v FROM students GROUP BY class_name ORDER BY class_name`)
		byStatus, _ := queryAll(`SELECT status k, COUNT(*) v FROM students GROUP BY status`)
		writeJSON(w, 200, map[string]any{
			"gender":   gender,
			"byClass":  byClass,
			"byStatus": byStatus,
			"alerts": map[string]any{
				"withoutParentPhone": queryCount(`SELECT COUNT(*) c FROM students WHERE status='在读' AND parent_phone = ''`),
				"withoutNo":          queryCount(`SELECT COUNT(*) c FROM students WHERE status='在读' AND student_no = ''`),
			},
		})
	})

	// ---------- 成绩统计 ----------
	mux.HandleFunc("GET /api/stats/scores", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		examID := q.Get("exam_id")
		if examID == "" {
			writeJSON(w, 200, map[string]any{"subjects": []any{}, "dist": []any{}, "pass": []any{}})
			return
		}
		full := 100.0
		if v, err := strconv.ParseFloat(q.Get("full_score"), 64); err == nil && v != 0 {
			full = v
		}
		passLine := math.Round(full*0.6*100) / 100
		goodLine := math.Round(full*0.85*100) / 100

		subjects, err := queryAll(`
			SELECT subject k,
			       COUNT(*) count,
			       ROUND(AVG(score),1) avg,
			       MAX(score) max,
			       MIN(score) min,
			       SUM(CASE WHEN score >= ? THEN 1 ELSE 0 END) passCount,
			       SUM(CASE WHEN score >= ? THEN 1 ELSE 0 END) goodCount
			FROM scores WHERE exam_id = ? GROUP BY subject ORDER BY id`, passLine, goodLine, examID)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		for _, s := range subjects {
			cnt := toFloat(s["count"])
			if cnt > 0 {
				s["passRate"] = math.Round(toFloat(s["passCount"])/cnt*1000) / 10
				s["goodRate"] = math.Round(toFloat(s["goodCount"])/cnt*1000) / 10
			} else {
				s["passRate"] = 0
				s["goodRate"] = 0
			}
		}

		// 总分分布直方图（满分总分的 10 等分段）
		totals, _ := queryAll(`SELECT student_id, SUM(score) total FROM scores WHERE exam_id = ? GROUP BY student_id`, examID)
		subjectCount := float64(len(subjects))
		if subjectCount == 0 {
			subjectCount = 1
		}
		fullTotal := full * subjectCount
		const buckets = 10
		dist := make([]map[string]any, 0, buckets)
		for i := 0; i < buckets; i++ {
			dist = append(dist, map[string]any{
				"range": strconv.Itoa(int(math.Round(fullTotal*float64(i)/buckets))) + "-" + strconv.Itoa(int(math.Round(fullTotal*float64(i+1)/buckets))),
				"v":     0,
			})
		}
		for _, t := range totals {
			idx := int(math.Floor(toFloat(t["total"]) / fullTotal * buckets))
			if idx > buckets-1 {
				idx = buckets - 1
			}
			if idx >= 0 {
				dist[idx]["v"] = dist[idx]["v"].(int) + 1
			}
		}

		// 单科分段统计
		bandData, _ := queryAll(`
			SELECT subject k,
			  SUM(CASE WHEN score >= ? THEN 1 ELSE 0 END) a,
			  SUM(CASE WHEN score >= ? AND score < ? THEN 1 ELSE 0 END) b,
			  SUM(CASE WHEN score >= ? AND score < ? THEN 1 ELSE 0 END) c,
			  SUM(CASE WHEN score < ? THEN 1 ELSE 0 END) d
			FROM scores WHERE exam_id = ? GROUP BY subject ORDER BY id`,
			math.Round(full*0.9), math.Round(full*0.75), math.Round(full*0.9), passLine, math.Round(full*0.75), passLine, examID)
		bands := []string{"优秀", "良好", "及格", "不及格"}
		bandStat := make([]map[string]any, 0, len(bandData))
		for _, row := range bandData {
			bandStat = append(bandStat, map[string]any{
				"subject": row["k"], "优秀": row["a"], "良好": row["b"], "及格": row["c"], "不及格": row["d"], "bands": bands,
			})
		}

		writeJSON(w, 200, map[string]any{
			"subjects": subjects, "dist": dist, "bandStat": bandStat,
			"full": full, "passLine": passLine, "goodLine": goodLine,
		})
	})

	// 学生个人历次成绩趋势
	mux.HandleFunc("GET /api/stats/student/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		rows, err := queryAll(`
			SELECT s.subject, s.score, s.full_score, e.name exam_name, e.exam_date, e.id exam_id
			FROM scores s JOIN exams e ON e.id = s.exam_id
			WHERE s.student_id = ?
			ORDER BY e.exam_date, e.id, s.id`, id)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		leaveCount := queryOne(`SELECT COUNT(*) c, IFNULL(SUM(days),0) d FROM leaves WHERE student_id = ?`, id)
		rewardCount, _ := queryAll(`SELECT kind, COUNT(*) c FROM rewards WHERE student_id = ? GROUP BY kind`, id)
		contacts := queryCount(`SELECT COUNT(*) c FROM contacts WHERE student_id = ?`, id)
		writeJSON(w, 200, map[string]any{
			"scores": rows, "leaves": leaveCount, "rewards": rewardCount, "contacts": contacts,
		})
	})

	// ---------- 请假统计 ----------
	mux.HandleFunc("GET /api/stats/leaves", func(w http.ResponseWriter, r *http.Request) {
		className := r.URL.Query().Get("class_name")
		where := ""
		params := []any{}
		if className != "" {
			where = "WHERE st.class_name = ?"
			params = append(params, className)
		}
		byType, _ := queryAll(`SELECT l.type k, COUNT(*) v FROM leaves l JOIN students st ON st.id = l.student_id `+where+` GROUP BY l.type`, params...)
		byMonth, _ := queryAll(`SELECT strftime('%Y-%m', l.start_date) k, COUNT(*) v, ROUND(SUM(l.days),1) d FROM leaves l JOIN students st ON st.id = l.student_id `+where+`
			GROUP BY k HAVING k IS NOT NULL ORDER BY k DESC LIMIT 12`, params...)
		reverseRows(byMonth)
		topStudents, _ := queryAll(`
			SELECT st.name k, st.class_name, COUNT(*) v, ROUND(SUM(l.days),1) d FROM leaves l JOIN students st ON st.id = l.student_id `+where+`
			GROUP BY l.student_id ORDER BY d DESC, v DESC LIMIT 10`, params...)
		byStatus, _ := queryAll(`SELECT l.status k, COUNT(*) v FROM leaves l JOIN students st ON st.id = l.student_id `+where+` GROUP BY l.status`, params...)
		totalDays := queryOne(`SELECT ROUND(IFNULL(SUM(l.days),0),1) c FROM leaves l JOIN students st ON st.id = l.student_id `+where, params...)
		writeJSON(w, 200, map[string]any{
			"byType": byType, "byMonth": byMonth, "topStudents": topStudents, "byStatus": byStatus, "totalDays": totalDays["c"],
		})
	})

	// ---------- 奖惩统计 ----------
	mux.HandleFunc("GET /api/stats/rewards", func(w http.ResponseWriter, r *http.Request) {
		byKind, _ := queryAll(`SELECT kind k, COUNT(*) v FROM rewards GROUP BY kind`)
		byMonth, _ := queryAll(`SELECT strftime('%Y-%m', record_date) k, COUNT(*) v FROM rewards WHERE record_date != '' GROUP BY k ORDER BY k DESC LIMIT 12`)
		reverseRows(byMonth)
		topGood, _ := queryAll(`SELECT st.name k, st.class_name, COUNT(*) v FROM rewards r JOIN students st ON st.id = r.student_id
			WHERE r.kind IN ('表扬','奖励') GROUP BY r.student_id ORDER BY v DESC LIMIT 10`)
		topBad, _ := queryAll(`SELECT st.name k, st.class_name, COUNT(*) v FROM rewards r JOIN students st ON st.id = r.student_id
			WHERE r.kind IN ('批评','处分') GROUP BY r.student_id ORDER BY v DESC LIMIT 10`)
		writeJSON(w, 200, map[string]any{
			"byKind": byKind, "byMonth": byMonth, "topGood": topGood, "topBad": topBad,
		})
	})
}

func reverseRows(rows []map[string]any) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}
