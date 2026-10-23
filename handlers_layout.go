package main

import (
	"encoding/json"
	"net/http"
)

func registerLayoutRoutes(mux *http.ServeMux) {
	// ---------- 座位表 ----------
	mux.HandleFunc("GET /api/seats/{className}", func(w http.ResponseWriter, r *http.Request) {
		className := r.PathValue("className")
		row := queryOne(`SELECT * FROM seat_layouts WHERE class_name = ?`, className)
		if row == nil {
			writeJSON(w, 200, map[string]any{"class_name": className, "rows": 5, "cols": 8, "group_size": 2, "assignments": []any{}, "colors": map[string]any{}})
			return
		}
		assignments := json.RawMessage(toStr(row["assignments"]))
		if len(assignments) == 0 {
			assignments = json.RawMessage("[]")
		}
		colors := json.RawMessage(toStr(row["colors"]))
		if len(colors) == 0 {
			colors = json.RawMessage("{}")
		}
		var a, col any
		json.Unmarshal(assignments, &a)
		json.Unmarshal(colors, &col)
		row["assignments"] = a
		row["colors"] = col
		writeJSON(w, 200, row)
	})

	mux.HandleFunc("PUT /api/seats/{className}", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBodyRaw(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(body, &m) != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		rowsN, colsN, gsN := 5.0, 8.0, 2.0
		json.Unmarshal(m["rows"], &rowsN)
		json.Unmarshal(m["cols"], &colsN)
		json.Unmarshal(m["group_size"], &gsN)
		assignments := m["assignments"]
		if len(assignments) == 0 || string(assignments) == "null" {
			assignments = json.RawMessage("[]")
		}
		colors := m["colors"]
		if len(colors) == 0 || string(colors) == "null" {
			colors = json.RawMessage("{}")
		}
		_, err = db.Exec(`
			INSERT INTO seat_layouts(class_name, rows, cols, group_size, assignments, colors, updated_at)
			VALUES (?,?,?,?,?,?, datetime('now','localtime'))
			ON CONFLICT(class_name) DO UPDATE SET rows=excluded.rows, cols=excluded.cols, group_size=excluded.group_size, assignments=excluded.assignments, colors=excluded.colors, updated_at=excluded.updated_at
		`, r.PathValue("className"), int(rowsN), int(colsN), int(gsN), string(assignments), string(colors))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// ---------- 课程表 ----------
	mux.HandleFunc("GET /api/schedule/{className}", func(w http.ResponseWriter, r *http.Request) {
		className := r.PathValue("className")
		row := queryOne(`SELECT config FROM schedules WHERE class_name = ?`, className)
		var conf any
		if row != nil {
			c := toStr(row["config"])
			if c == "" {
				c = "{}"
			}
			json.Unmarshal([]byte(c), &conf)
		}
		writeJSON(w, 200, map[string]any{"class_name": className, "config": conf})
	})

	mux.HandleFunc("PUT /api/schedule/{className}", func(w http.ResponseWriter, r *http.Request) {
		body, err := decodeBodyRaw(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(body, &m) != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		conf := m["config"]
		if len(conf) == 0 || string(conf) == "null" {
			conf = json.RawMessage("{}")
		}
		_, err = db.Exec(`
			INSERT INTO schedules(class_name, config, updated_at) VALUES (?,?, datetime('now','localtime'))
			ON CONFLICT(class_name) DO UPDATE SET config = excluded.config, updated_at = excluded.updated_at
		`, r.PathValue("className"), string(conf))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})
}
