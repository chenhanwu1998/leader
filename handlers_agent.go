package main

// 智能体：代理 aihub 平台（ai-route 部署）的流式聊天，心理模式注入知识库检索结果
// 与 leader-sys 的 server/routes/agent.js 一一对应

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const aihubBase = "https://testaihub.fdyqcloud.com"

var agentModels = []map[string]string{
	{"id": "kimi-k3", "label": "kimi-k3（默认）"},
	{"id": "deepseek-v4-flash", "label": "deepseek-v4-flash"},
	{"id": "deepseek-v4.1-flash", "label": "deepseek-v4.1-flash"},
}

func aihubKey() string {
	b, err := os.ReadFile(filepath.Join(baseDataDir, "aihub-key.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func aihubKbID() string {
	b, err := os.ReadFile(filepath.Join(baseDataDir, "aihub-kb.json"))
	if err != nil {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	return strings.TrimSpace(toStr(m["kb_id"]))
}

const sysPlain = "你是「教师台账」里的班主任智能助手。用简明、实用、有同理心的中文回答班级管理、学生沟通、家校联系、成绩分析等问题；必要时给出可直接执行的步骤建议。"

const sysPsych = "你是学校心理辅导室的助手，服务对象是班主任和心理健康教师。回答必须基于给定的心理知识库资料；资料未覆盖的内容不要编造，可说明「资料中未涉及」并给出一般性、安全的建议方向。" +
	"回答风格：专业、温和、具体，多用条目。涉及自伤、自杀、危机信号时，优先提示立即线下介入、联系监护人并转介专业机构，不要只给文字安慰。"

type kbHit struct {
	Content string
	Title   string
}

// 知识库混合检索（失败不阻塞主对话）
func retrieveKb(ctx context.Context, query, key, kbId string) []kbHit {
	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"query_text": query})
	req, err := http.NewRequestWithContext(cctx, "POST", aihubBase+"/api/rag/v1/knowledge-bases/"+kbId+"/hybrid-search", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var data struct {
		Data []map[string]any `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&data) != nil {
		return nil
	}
	out := []kbHit{}
	for i, h := range data.Data {
		if i >= 4 {
			break
		}
		title := toStr(h["title"])
		if title == "" {
			title = toStr(h["knowledge_title"])
		}
		out = append(out, kbHit{Content: toStr(h["content"]), Title: title})
	}
	return out
}

func registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agent/meta", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"models": agentModels, "default_model": "kimi-k3",
			"ready": aihubKey() != "", "knowledge_base": aihubKbID() != "",
		})
	})

	mux.HandleFunc("POST /api/agent/chat", func(w http.ResponseWriter, r *http.Request) {
		key := aihubKey()
		if key == "" {
			fail(w, 503, "未配置智能体密钥（data/aihub-key.txt）")
			return
		}
		body, err := decodeBody(r)
		if err != nil {
			fail(w, 400, "请求格式错误")
			return
		}
		rawMsgs, ok := body["messages"].([]any)
		if !ok || len(rawMsgs) == 0 {
			fail(w, 400, "缺少对话内容")
			return
		}
		type chatMsg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		chatMessages := []chatMsg{}
		for _, m := range rawMsgs {
			mm, ok := m.(map[string]any)
			if !ok {
				continue
			}
			role := toStr(mm["role"])
			if role != "user" && role != "assistant" {
				continue
			}
			content := toStr(mm["content"])
			if len(content) > 4000 {
				content = content[:4000]
			}
			chatMessages = append(chatMessages, chatMsg{Role: role, Content: content})
			if len(chatMessages) > 20 {
				chatMessages = chatMessages[len(chatMessages)-20:]
			}
		}
		if len(chatMessages) == 0 || chatMessages[len(chatMessages)-1].Role != "user" {
			fail(w, 400, "最后一句话需要是用户输入")
			return
		}

		psych := body["psych"] == true
		system := sysPlain
		if psych {
			system = sysPsych
		}
		type source struct {
			N     int    `json:"n"`
			Title string `json:"title"`
		}
		sources := []source{}
		if psych {
			if kbId := aihubKbID(); kbId != "" {
				hits := retrieveKb(r.Context(), chatMessages[len(chatMessages)-1].Content, key, kbId)
				if len(hits) > 0 {
					refs := []string{}
					for i, h := range hits {
						c := h.Content
						if len(c) > 1200 {
							c = c[:1200]
						}
						refs = append(refs, fmt.Sprintf("【资料%d】\n%s", i+1, c))
						title := h.Title
						if title == "" {
							title = fmt.Sprintf("资料%d", i+1)
						}
						sources = append(sources, source{N: i + 1, Title: title})
					}
					system += "\n\n以下是从校园心理健康知识库检索到的相关资料：\n" + strings.Join(refs, "\n\n")
				}
			}
		}

		usedModel := strField(body, "model", "")
		valid := false
		for _, m := range agentModels {
			if m["id"] == usedModel {
				valid = true
				break
			}
		}
		if !valid {
			usedModel = "kimi-k3"
		}

		upBody, _ := json.Marshal(map[string]any{
			"model":    usedModel,
			"messages": append([]chatMsg{{Role: "system", Content: system}}, chatMessages...),
			"stream":   true,
		})
		ureq, err := http.NewRequestWithContext(r.Context(), "POST", aihubBase+"/v1/chat/completions", bytes.NewReader(upBody))
		if err != nil {
			fail(w, 502, "无法连接智能体服务："+err.Error())
			return
		}
		ureq.Header.Set("Authorization", "Bearer "+key)
		ureq.Header.Set("Content-Type", "application/json")
		ureq.Header.Set("Accept", "text/event-stream")
		upstream, err := http.DefaultClient.Do(ureq)
		if err != nil {
			fail(w, 502, "无法连接智能体服务："+err.Error())
			return
		}
		defer upstream.Body.Close()
		if upstream.StatusCode != 200 {
			text, _ := io.ReadAll(io.LimitReader(upstream.Body, 4096))
			if len(text) > 200 {
				text = text[:200]
			}
			fail(w, 502, fmt.Sprintf("智能体服务返回 %d：%s", upstream.StatusCode, string(text)))
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			fail(w, 500, "当前服务不支持流式响应")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(200)
		if len(sources) > 0 {
			payload, _ := json.Marshal(sources)
			fmt.Fprintf(w, "event: sources\ndata: %s\n\n", payload)
			flusher.Flush()
		}
		// 逐块转发上游 SSE（请求被客户端取消时 r.Context 会中断上游读取）
		buf := make([]byte, 4096)
		for {
			n, err := upstream.Body.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					return
				}
				flusher.Flush()
			}
			if err != nil {
				return
			}
		}
	})
}
