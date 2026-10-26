package main

// 智能体：代理 aihub 平台（ai-route 部署）的流式聊天，心理模式注入知识库检索结果
// 对话历史按账号落在各自的台账库（agent_conversations / agent_messages）。
// 与 leader-sys 的 server/routes/agent.js 一一对应

import (
	"bytes"
	"context"
	"database/sql"
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

// —— 对话历史存取 ——

type convSource struct {
	N     int    `json:"n"`
	Title string `json:"title"`
}

// createConversation 用首条用户输入生成会话标题
func createConversation(d *sql.DB, mode, model, firstUserMsg string) int64 {
	title := strings.TrimSpace(firstUserMsg)
	r := []rune(title)
	if len(r) > 30 {
		title = string(r[:30]) + "…"
	}
	if title == "" {
		title = "新对话"
	}
	res, err := d.Exec(`INSERT INTO agent_conversations(title, mode, model) VALUES (?,?,?)`, title, mode, model)
	if err != nil {
		return 0
	}
	id, _ := res.LastInsertId()
	return id
}

func conversationExists(d *sql.DB, id int64) bool {
	return queryOne(d, `SELECT id FROM agent_conversations WHERE id = ?`, id) != nil
}

func saveAgentMessage(d *sql.DB, convId int64, role, content string, sources []convSource) {
	src := ""
	if len(sources) > 0 {
		if b, err := json.Marshal(sources); err == nil {
			src = string(b)
		}
	}
	d.Exec(`INSERT INTO agent_messages(conversation_id, role, content, sources) VALUES (?,?,?,?)`, convId, role, content, src)
	d.Exec(`UPDATE agent_conversations SET updated_at = datetime('now','localtime') WHERE id = ?`, convId)
}

// sseDeltaCollector 边转发边累计助手回复（解析上游 SSE 的 delta.content）
type sseDeltaCollector struct {
	carry   strings.Builder // 未按行切齐的残留
	content strings.Builder
}

func (c *sseDeltaCollector) feed(chunk []byte) {
	c.carry.Write(chunk)
	s := c.carry.String()
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(s[:i], "\r")
		s = s[i+1:]
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(line[5:])
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var j struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(payload), &j) == nil && len(j.Choices) > 0 {
			c.content.WriteString(j.Choices[0].Delta.Content)
		}
	}
	c.carry.Reset()
	c.carry.WriteString(s)
}

func (c *sseDeltaCollector) text() string { return c.content.String() }

func registerAgentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agent/meta", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"models": agentModels, "default_model": "kimi-k3",
			"ready": aihubKey() != "", "knowledge_base": aihubKbID() != "",
		})
	})

	// 会话列表（最近更新在前，含最后一条预览）
	mux.HandleFunc("GET /api/agent/conversations", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		rows, err := queryAll(d, `
SELECT c.id, c.title, c.mode, c.model, c.created_at, c.updated_at,
  (SELECT COUNT(*) FROM agent_messages m WHERE m.conversation_id = c.id) AS msg_count,
  COALESCE((SELECT m.content FROM agent_messages m WHERE m.conversation_id = c.id AND m.role = 'user' ORDER BY m.id DESC LIMIT 1), '') AS preview
FROM agent_conversations c ORDER BY c.updated_at DESC, c.id DESC`)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, normalizeRow(row))
		}
		writeJSON(w, 200, out)
	})

	mux.HandleFunc("POST /api/agent/conversations", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		body, _ := decodeBody(r)
		mode := strField(body, "mode", "plain")
		if mode != "psych" {
			mode = "plain"
		}
		res, err := d.Exec(`INSERT INTO agent_conversations(title, mode, model) VALUES ('新对话', ?, ?)`, mode, strField(body, "model", ""))
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, 200, map[string]any{"id": id, "title": "新对话", "mode": mode})
	})

	mux.HandleFunc("GET /api/agent/conversations/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		id := r.PathValue("id")
		conv := queryOne(d, `SELECT id, title, mode, model, created_at, updated_at FROM agent_conversations WHERE id = ?`, id)
		if conv == nil {
			fail(w, 404, "对话不存在")
			return
		}
		msgs, err := queryAll(d, `SELECT id, role, content, sources, created_at FROM agent_messages WHERE conversation_id = ? ORDER BY id`, id)
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
		list := make([]map[string]any, 0, len(msgs))
		for _, m := range msgs {
			sources := []convSource{}
			if s := toStr(m["sources"]); s != "" {
				json.Unmarshal([]byte(s), &sources)
			}
			list = append(list, map[string]any{
				"id": toInt(m["id"]), "role": toStr(m["role"]), "content": toStr(m["content"]),
				"sources": sources, "created_at": toStr(m["created_at"]),
			})
		}
		out := normalizeRow(conv)
		out["messages"] = list
		writeJSON(w, 200, out)
	})

	mux.HandleFunc("PUT /api/agent/conversations/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		id := r.PathValue("id")
		if !conversationExists(d, toInt(id)) {
			fail(w, 404, "对话不存在")
			return
		}
		body, _ := decodeBody(r)
		title := strings.TrimSpace(strField(body, "title", ""))
		rn := []rune(title)
		if len(rn) == 0 || len(rn) > 40 {
			fail(w, 400, "标题需为 1-40 字")
			return
		}
		d.Exec(`UPDATE agent_conversations SET title = ? WHERE id = ?`, title, id)
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("DELETE /api/agent/conversations/{id}", func(w http.ResponseWriter, r *http.Request) {
		d := ledgerDB(r)
		d.Exec(`DELETE FROM agent_conversations WHERE id = ?`, r.PathValue("id")) // 消息外键级联
		writeJSON(w, 200, map[string]any{"ok": true})
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
		mode := "plain"
		system := sysPlain
		if psych {
			mode = "psych"
			system = sysPsych
		}
		sources := []convSource{}
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
						sources = append(sources, convSource{N: i + 1, Title: title})
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

		// 会话归属：未指定则按首条提问自动建会话；先落库用户消息
		d := ledgerDB(r)
		convId := toInt(body["conversation_id"])
		if convId > 0 && !conversationExists(d, convId) {
			convId = 0
		}
		if convId == 0 {
			convId = createConversation(d, mode, usedModel, chatMessages[len(chatMessages)-1].Content)
		}
		userContent := chatMessages[len(chatMessages)-1].Content
		saveAgentMessage(d, convId, "user", userContent, nil)
		d.Exec(`UPDATE agent_conversations SET mode = ?, model = ? WHERE id = ?`, mode, usedModel, convId)

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

		// 桌面模式（Wails 内嵌通道）writer 不一定支持 Flush：能刷就刷（真流式），
		// 不能刷就缓存到结束一次性给出（回环 HTTP 监听可用时前端会自动走真流式通道）
		flusher, _ := w.(http.Flusher)
		flush := func() {
			if flusher != nil {
				flusher.Flush()
			}
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(200)
		fmt.Fprintf(w, "event: conversation\ndata: {\"id\":%d}\n\n", convId)
		if len(sources) > 0 {
			payload, _ := json.Marshal(sources)
			fmt.Fprintf(w, "event: sources\ndata: %s\n\n", payload)
		}
		flush()

		// 逐块转发上游 SSE，同时累计助手全文用于落库（请求被客户端取消时 r.Context 会中断上游读取）
		collector := &sseDeltaCollector{}
		buf := make([]byte, 4096)
		for {
			n, readErr := upstream.Body.Read(buf)
			if n > 0 {
				collector.feed(buf[:n])
				if _, werr := w.Write(buf[:n]); werr != nil {
					break
				}
				flush()
			}
			if readErr != nil {
				break
			}
		}
		// 中途断开也保留已生成的部分内容
		if text := collector.text(); text != "" {
			saveAgentMessage(d, convId, "assistant", text, sources)
		}
	})
}
