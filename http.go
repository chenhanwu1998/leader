package main

import (
	"encoding/json"
	"io"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"message": msg})
}

// decodeBody 解析 JSON 请求体为 map（限制 20MB）
func decodeBody(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func decodeBodyRaw(r *http.Request) (json.RawMessage, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		body = []byte("null")
	}
	return body, nil
}

// strField 从 body 取字符串字段
func strField(body map[string]any, key, def string) string {
	if v, ok := body[key]; ok && v != nil {
		return toStr(v)
	}
	return def
}

// wrapErr 统一把 panic 转成 500 JSON
func wrapErr(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if err, ok := rec.(error); ok {
					fail(w, http.StatusInternalServerError, err.Error())
				} else {
					fail(w, http.StatusInternalServerError, "服务器内部错误")
				}
			}
		}()
		h.ServeHTTP(w, r)
	})
}
