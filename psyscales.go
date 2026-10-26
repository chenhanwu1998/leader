package main

// 心理筛查量表：数据化题库（可覆盖自定义）+ 通用计分引擎
// 与 leader-sys 的 server/psy-scales.js 一一对应。
// 内置 PHQ-9 / GAD-7 / SAS / SDS；覆盖版本以 JSON 存在当前账号台账库 settings('psy_scales')

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type scaleOption struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type scaleThreshold struct {
	Min   float64 `json:"min"`
	Level string  `json:"level"`
	Note  string  `json:"note"`
}

type psychoScale struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Hint       string           `json:"hint"`
	Builtin    bool             `json:"builtin"`
	Options    []scaleOption    `json:"options"`
	Items      []string         `json:"items"`
	Reverse    []int            `json:"reverse"`
	ScoreMode  string           `json:"score_mode"` // sum | std125
	Thresholds []scaleThreshold `json:"thresholds"`
	FlagItem   int              `json:"flag_item"` // 1 起题号，0 表示无预警题
	FlagMin    float64          `json:"flag_min"`
	FlagNote   string           `json:"flag_note"`
}

var freq4Opts = []scaleOption{{"完全不会", 0}, {"好几天", 1}, {"一半以上的天数", 2}, {"几乎每天", 3}}
var zung4Opts = []scaleOption{{"没有或很少时间有", 1}, {"小部分时间有", 2}, {"相当多时间有", 3}, {"绝大部分或全部时间有", 4}}

var defaultScales = []psychoScale{
	{
		ID: "PHQ-9", Name: "PHQ-9 抑郁筛查量表", Hint: "在过去的两周里，以下情况有多少天困扰过你？", Builtin: true,
		Options: freq4Opts,
		Items: []string{
			"做事时提不起劲或没有兴趣",
			"感到心情低落、沮丧或绝望",
			"入睡困难、睡不安稳或睡眠过多",
			"感觉疲倦或没有活力",
			"食欲不振或吃太多",
			"觉得自己很糟，或觉得自己很失败，或让自己、家人失望",
			"对事物专注有困难，例如看报纸或看电视时",
			"动作或说话速度缓慢到别人已经察觉；或正好相反，烦躁坐立不安、动来动去比平时更甚",
			"有不如死掉或用某种方式伤害自己的念头",
		},
		Reverse:   []int{},
		ScoreMode: "sum",
		Thresholds: []scaleThreshold{
			{20, "重度抑郁", "总分已达重度区间。建议立即约谈学生进行安全评估，当天联系监护人并建议尽快到专业机构就诊；在校期间启动重点关注与每日简短跟进。"},
			{15, "中重度抑郁", "中重度抑郁症状明显，学习与生活功能可能已受影响。建议尽快安排深入谈话，联系监护人，建议专业评估；1-2 周后复测。"},
			{10, "中度抑郁", "存在中度抑郁倾向。建议安排谈心了解诱因，加强日常关注与家校沟通，2-4 周后复测观察走势。"},
			{5, "轻度抑郁", "有轻度抑郁情绪迹象。保持关注，营造可求助的班级氛围，可结合睡眠、运动与同伴支持做预防性引导，一个月后复测。"},
			{0, "无明显抑郁症状", "本次筛查未见明显抑郁症状，保持常规关注即可。"},
		},
		FlagItem: 9, FlagMin: 1,
		FlagNote: "第 9 题（自伤念头）出现得分。请尽快安排安全谈话，平静直接地询问其想法与计划；当天联系监护人与心理老师，做好记录并密切跟进。",
	},
	{
		ID: "GAD-7", Name: "GAD-7 焦虑筛查量表", Hint: "在过去的两周里，以下情况有多少天困扰过你？", Builtin: true,
		Options: freq4Opts,
		Items: []string{
			"感觉紧张、焦虑或急切",
			"不能够停止或控制担忧",
			"对各种各样的事情担忧过多",
			"很难放松下来",
			"由于不安而无法静坐",
			"变得容易烦恼或急躁",
			"感到似乎将有可怕的事情发生而害怕",
		},
		Reverse:   []int{},
		ScoreMode: "sum",
		Thresholds: []scaleThreshold{
			{15, "重度焦虑", "焦虑水平已达重度区间，可能伴随明显的躯体症状与功能受损。建议联系监护人并转介专业评估，在校期间降低压力源、密切跟进。"},
			{10, "中度焦虑", "存在中度焦虑倾向。建议谈心了解压力来源，教放松技巧（腹式呼吸、渐进式肌肉放松），2-4 周后复测。"},
			{5, "轻度焦虑", "有轻度焦虑情绪迹象。保持关注，帮助规律作息与运动，必要时考前做放松训练，一个月后复测。"},
			{0, "无明显焦虑症状", "本次筛查未见明显焦虑症状，保持常规关注即可。"},
		},
	},
	{
		ID: "SAS", Name: "SAS 焦虑自评量表", Hint: "根据最近一周的实际情况，选择最符合的程度。带 * 题为反向计分。", Builtin: true,
		Options: zung4Opts,
		Items: []string{
			"我觉得比平常容易紧张和着急",
			"我无缘无故地感到害怕",
			"我容易心里烦乱或觉得惊恐",
			"我觉得我可能将要发疯",
			"*我觉得一切都很好，也不会发生什么不幸",
			"我手脚发抖打颤",
			"我因为头痛、颈痛和背痛而苦恼",
			"我感觉容易衰弱和疲乏",
			"*我觉得心平气和，并且容易安静坐着",
			"我觉得心跳很快",
			"我因为一阵阵头晕而苦恼",
			"我有晕倒发作，或觉得要晕倒似的",
			"*我呼气吸气都感到很容易",
			"我的手脚麻木和刺痛",
			"我因为胃痛和消化不良而苦恼",
			"我常常要小便",
			"*我的手常常是干燥温暖的",
			"我脸红发热",
			"*我容易入睡并且一夜睡得很好",
			"我做恶梦",
		},
		Reverse:   []int{4, 8, 12, 16, 18},
		ScoreMode: "std125",
		Thresholds: []scaleThreshold{
			{70, "重度焦虑", "标准分已达重度焦虑区间，主观痛苦明显。建议尽快转介专业评估，联系监护人，校内给予减压安排与每日关注。"},
			{60, "中度焦虑", "标准分提示中度焦虑。建议谈心了解压力来源，教放松与正念练习，2-4 周后复测。"},
			{50, "轻度焦虑", "标准分提示轻度焦虑。保持关注，帮助规律作息与适量运动，一个月后复测。"},
			{0, "正常范围", "标准分处于正常范围，保持常规关注即可。"},
		},
	},
	{
		ID: "SDS", Name: "SDS 抑郁自评量表", Hint: "根据最近一周的实际情况，选择最符合的程度。带 * 题为反向计分。", Builtin: true,
		Options: zung4Opts,
		Items: []string{
			"我觉得闷闷不乐，情绪低沉",
			"*我觉得一天之中早晨最好",
			"我一阵阵哭出来或觉得想哭",
			"我晚上睡眠不好",
			"*我吃得跟平常一样多",
			"*我与异性密切接触时和以往一样感到愉快",
			"我发觉我的体重在下降",
			"我有便秘的苦恼",
			"我心跳比平时快",
			"我无缘无故地感到疲乏",
			"*我的头脑跟平常一样清楚",
			"*我觉得经常做的事情并没有困难",
			"我觉得不安而平静不下来",
			"*我对将来抱有希望",
			"我比平常容易生气激动",
			"*我觉得作出决定是容易的",
			"*我觉得自己是个有用的人，有人需要我",
			"*我的生活过得很有意思",
			"我认为如果我死了，别人会生活得好些",
			"*平常感兴趣的事我仍然照样感兴趣",
		},
		Reverse:   []int{1, 4, 5, 10, 11, 13, 15, 16, 17, 19},
		ScoreMode: "std125",
		Thresholds: []scaleThreshold{
			{73, "重度抑郁", "标准分已达重度抑郁区间。建议立即安全谈话与专业评估，当天联系监护人，启动重点关注流程。"},
			{63, "中度抑郁", "标准分提示中度抑郁。建议深入谈心并联系监护人，建议专业评估，1-2 周后复测。"},
			{53, "轻度抑郁", "标准分提示轻度抑郁。保持关注与家校沟通，鼓励规律运动与同伴支持，一个月后复测。"},
			{0, "正常范围", "标准分处于正常范围，保持常规关注即可。"},
		},
		FlagItem: 19, FlagMin: 3,
		FlagNote: "第 19 题（无价值感题目）得分偏高。建议尽快关心学生的自我评价与情绪状态，进行安全谈话，必要时按重点关注流程跟进。",
	},
}

// resolveScales 当前账号生效的量表集合（覆盖了用覆盖版，否则内置版）
func resolveScales(d *sql.DB) []psychoScale {
	raw := getSetting(d, "psy_scales", "")
	if raw != "" {
		var list []psychoScale
		if json.Unmarshal([]byte(raw), &list) == nil && len(list) > 0 {
			return list
		}
	}
	return defaultScales
}

// resolveScale 按 id 取生效量表
func resolveScale(d *sql.DB, id string) *psychoScale {
	for i, s := range resolveScales(d) {
		if s.ID == id {
			return &resolveScales(d)[i]
		}
	}
	return nil
}

var scaleIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func trimRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > n {
		return string(r[:n])
	}
	return string(r)
}

// validateScale 校验并规范化一份量表定义（与 Node 端规则一致），非法返回 error
func validateScale(raw psychoScale) (psychoScale, error) {
	s := psychoScale{}
	s.ID = strings.TrimSpace(raw.ID)
	if !scaleIDRe.MatchString(s.ID) {
		return s, fmt.Errorf("量表编号「%s」需为 1-32 位字母、数字、-、_", s.ID)
	}
	s.Name = strings.TrimSpace(raw.Name)
	if s.Name == "" || runeLen(s.Name) > 50 {
		return s, fmt.Errorf("量表「%s」名称需为 1-50 字", s.ID)
	}
	s.Hint = trimRunes(raw.Hint, 200)
	s.Builtin = raw.Builtin

	items := []string{}
	for _, t := range raw.Items {
		t = strings.TrimSpace(t)
		if t != "" {
			items = append(items, t)
		}
	}
	if len(items) < 1 || len(items) > 60 {
		return s, fmt.Errorf("量表「%s」题目需 1-60 道", s.ID)
	}
	for _, t := range items {
		if runeLen(t) > 120 {
			return s, fmt.Errorf("量表「%s」存在超过 120 字的题目", s.ID)
		}
	}
	s.Items = items

	if len(raw.Options) < 2 || len(raw.Options) > 6 {
		return s, fmt.Errorf("量表「%s」选项需 2-6 个", s.ID)
	}
	opts := append([]scaleOption(nil), raw.Options...)
	for i := range opts {
		opts[i].Label = trimRunes(opts[i].Label, 30)
		if opts[i].Label == "" {
			return s, fmt.Errorf("量表「%s」选项文字需 1-30 字", s.ID)
		}
		if opts[i].Score != float64(int64(opts[i].Score)) || opts[i].Score < 0 || opts[i].Score > 10 {
			return s, fmt.Errorf("量表「%s」选项分值需为 0-10 的整数", s.ID)
		}
	}
	sort.Slice(opts, func(i, j int) bool { return opts[i].Score < opts[j].Score })
	for i := 1; i < len(opts); i++ {
		if opts[i].Score == opts[i-1].Score {
			return s, fmt.Errorf("量表「%s」选项分值不能重复", s.ID)
		}
	}
	s.Options = opts

	revSet := map[int]bool{}
	s.Reverse = []int{}
	for _, i := range raw.Reverse {
		if i >= 0 && i < len(items) && !revSet[i] {
			revSet[i] = true
			s.Reverse = append(s.Reverse, i)
		}
	}
	sort.Ints(s.Reverse)
	if raw.ScoreMode == "std125" {
		s.ScoreMode = "std125"
	} else {
		s.ScoreMode = "sum"
	}

	if len(raw.Thresholds) == 0 {
		return s, fmt.Errorf("量表「%s」至少需要一个结果分级", s.ID)
	}
	thresholds := []scaleThreshold{}
	for _, t := range raw.Thresholds {
		lv := trimRunes(t.Level, 30)
		if lv == "" {
			continue
		}
		thresholds = append(thresholds, scaleThreshold{t.Min, lv, trimRunes(t.Note, 300)})
	}
	if len(thresholds) == 0 {
		return s, fmt.Errorf("量表「%s」结果分级不合法", s.ID)
	}
	sort.Slice(thresholds, func(i, j int) bool { return thresholds[i].Min > thresholds[j].Min })
	if thresholds[len(thresholds)-1].Min != 0 {
		return s, fmt.Errorf("量表「%s」最后一个分级的分数线需为 0（兜底档）", s.ID)
	}
	s.Thresholds = thresholds

	fi := int(raw.FlagItem)
	if fi >= 1 && fi <= len(items) {
		s.FlagItem = fi
		if raw.FlagMin < 1 {
			return s, fmt.Errorf("量表「%s」预警题的预警分值需 ≥ 1", s.ID)
		}
		s.FlagMin = raw.FlagMin
	}
	s.FlagNote = trimRunes(raw.FlagNote, 300)
	return s, nil
}

func validateScales(list []psychoScale) ([]psychoScale, error) {
	if len(list) == 0 || len(list) > 20 {
		return nil, errors.New("量表列表需为 1-20 套")
	}
	out := make([]psychoScale, 0, len(list))
	seen := map[string]bool{}
	for _, raw := range list {
		s, err := validateScale(raw)
		if err != nil {
			return nil, err
		}
		if seen[s.ID] {
			return nil, fmt.Errorf("存在重复的量表编号「%s」", s.ID)
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	return out, nil
}

// scoreScale 通用计分：answers 为各题所选项的分值数组。返回 score / level / flagged
func scoreScale(s *psychoScale, answers []float64) (float64, string, bool, error) {
	if len(answers) != len(s.Items) {
		return 0, "", false, errors.New("答题数量与量表题目数不一致")
	}
	lo, hi := s.Options[0].Score, s.Options[len(s.Options)-1].Score
	revSet := map[int]bool{}
	for _, i := range s.Reverse {
		revSet[i] = true
	}
	var raw float64
	vals := make([]float64, len(answers))
	for i, a := range answers {
		v := a
		if v != v { // NaN
			return 0, "", false, fmt.Errorf("第 %d 题答案不是有效分值", i+1)
		}
		// 历史兼容：1 起评的量表（SAS/SDS）里 0 代表旧前端的空值提交，按 1 计；
		// 0 起评的量表（PHQ-9/GAD-7）中 0 是合法分值「完全不会」，不能改
		if v == 0 && lo >= 1 {
			v = 1
		}
		if v < lo || v > hi {
			return 0, "", false, fmt.Errorf("第 %d 题分值超出范围", i+1)
		}
		vals[i] = v
		if revSet[i] {
			raw += lo + hi - v
		} else {
			raw += v
		}
	}
	score := raw
	if s.ScoreMode == "std125" {
		score = float64(int(raw * 1.25))
	}
	level := ""
	for _, t := range s.Thresholds {
		if score >= t.Min {
			level = t.Level
			break
		}
	}
	flagged := false
	if s.FlagItem > 0 && s.FlagItem <= len(vals) {
		fm := s.FlagMin
		if fm < 1 {
			fm = 1
		}
		flagged = vals[s.FlagItem-1] >= fm
	}
	return score, level, flagged, nil
}

// scoreAssessment 兼容旧调用：按内置量表 id 计分（种子数据用）
func scoreAssessment(scaleID string, answers []float64) (float64, string, bool, error) {
	for i := range defaultScales {
		if defaultScales[i].ID == scaleID {
			return scoreScale(&defaultScales[i], answers)
		}
	}
	return 0, "", false, errors.New("未知量表: " + scaleID)
}
