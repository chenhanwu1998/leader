package main

// 量表问卷 DOCX 生成：与 leader-sys 的 server/docx.js 一一对应
// 生成「学生填写版」问卷：标题 + 指导语 + 填写信息行 + 逐题选项（不含分值）

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

const docxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const docxDocRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`

func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

type docxParaOpt struct {
	Bold       bool
	Center     bool
	Sz         int
	SpaceAfter int
}

func docxPara(text string, o docxParaOpt) string {
	jc := ""
	if o.Center {
		jc = `<w:jc w:val="center"/>`
	}
	b := ""
	if o.Bold {
		b = "<w:b/>"
	}
	return fmt.Sprintf(`<w:p><w:pPr>%s<w:spacing w:after="%d"/></w:pPr><w:r><w:rPr>%s<w:sz w:val="%d"/><w:szCs w:val="%d"/></w:rPr><w:t xml:space="preserve">%s</w:t></w:r></w:p>`,
		jc, o.SpaceAfter, b, o.Sz, o.Sz, xmlEsc(text))
}

// buildScaleDocx 由生效量表定义生成问卷 docx 字节
func buildScaleDocx(s *psychoScale) ([]byte, error) {
	parts := []string{}
	parts = append(parts, docxPara(s.Name, docxParaOpt{Bold: true, Center: true, Sz: 36, SpaceAfter: 200}))
	if s.Hint != "" {
		parts = append(parts, docxPara("指导语："+s.Hint, docxParaOpt{Sz: 22, SpaceAfter: 160}))
	}
	parts = append(parts, docxPara("班级：＿＿＿＿＿＿　姓名：＿＿＿＿＿＿　日期：＿＿＿＿年＿＿月＿＿日", docxParaOpt{Center: true, Sz: 22, SpaceAfter: 220}))
	opts := make([]string, 0, len(s.Options))
	for _, o := range s.Options {
		opts = append(opts, "□ "+o.Label)
	}
	optLine := strings.Join(opts, "　　")
	for i, item := range s.Items {
		parts = append(parts, docxPara(fmt.Sprintf("%d. %s", i+1, item), docxParaOpt{Sz: 22, SpaceAfter: 60}))
		parts = append(parts, docxPara("　　"+optLine, docxParaOpt{Sz: 21, SpaceAfter: 160}))
	}
	parts = append(parts, docxPara("————————————", docxParaOpt{Center: true, SpaceAfter: 100}))
	parts = append(parts, docxPara("说明：本问卷由学生自评填写，结果由教师录入「教师台账系统」自动计分并生成解释，仅供筛查参考，不能替代临床诊断。", docxParaOpt{Sz: 18, SpaceAfter: 60}))

	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>` + strings.Join(parts, "") + `
<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>
</w:body></w:document>`

	files := []struct{ name, body string }{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRootRels},
		{"word/document.xml", document},
		{"word/_rels/document.xml.rels", docxDocRels},
	}
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
