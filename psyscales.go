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
	{
		ID: "WHO-5", Name: "WHO-5 身心健康指数", Hint: "在过去的两周里，你有多大比例的时间处于以下状态？", Builtin: true,
		Options: []scaleOption{{"从来没有", 0}, {"偶尔", 1}, {"少于一半时间", 2}, {"超过一半时间", 3}, {"大部分时间", 4}, {"所有时间", 5}},
		Items: []string{
			"我感到愉快，心情好",
			"我感到平静和放松",
			"我感到精力充沛",
			"我睡醒时感到清新，得到了充分休息",
			"我的日常生活充满让我感兴趣的事情",
		},
		Reverse: []int{}, ScoreMode: "sum", // 原始分 0-25，分数越低情绪状态越差
		Thresholds: []scaleThreshold{
			{13, "状态良好", "身心健康状态良好，保持规律作息与运动习惯即可。"},
			{8, "状态偏低", "近两周身心状态偏低。建议关注睡眠、运动与学习压力，主动谈心了解近况，两周后复测。"},
			{0, "情绪低落风险", "WHO-5 ≤ 12 提示身心状态欠佳，≤ 7 建议进一步评估。请安排谈心了解情绪与睡眠状况，结合 PHQ-9 复评，必要时联系心理老师。"},
		},
	},
	{
		ID: "PSS-10", Name: "PSS-10 压力知觉量表", Hint: "在过去的一个月里，你有多大频率出现下列感受？", Builtin: true,
		Options: []scaleOption{{"从不", 0}, {"偶尔", 1}, {"有时", 2}, {"经常", 3}, {"总是", 4}},
		Items: []string{
			"因为一些意想不到的事情发生而感到心烦意乱",
			"感到无法控制自己生活中重要的事情",
			"感到紧张不安和有压力",
			"对自己处理个人问题的能力感到有信心",
			"感到事情正按照自己的期望发展",
			"发现自己无法应付所有必须要做的事情",
			"能够控制生活中令人恼火的事情",
			"感到自己能够掌握一切",
			"因为一些无法控制的事情发生而感到生气",
			"感到困难堆积如山，自己无法克服",
		},
		Reverse: []int{3, 4, 6, 7}, ScoreMode: "sum", // 第 4、5、7、8 题为正向表述，反向计分；总分 0-40
		Thresholds: []scaleThreshold{
			{27, "高压力", "主观压力感处于高水平，可能已影响睡眠与情绪。建议谈心寻找压力源、协助做减法，教放松技巧，联系家长共同减压，两周后复测。"},
			{14, "中等压力", "压力感处于中等水平，属青少年常见区间但需关注。可教番茄钟拆分任务与呼吸放松，大考前后重点留意走势。"},
			{0, "低压力", "主观压力感较低，当前应对资源充足，保持常规关注即可。"},
		},
	},
	{
		ID: "GSES", Name: "GSES 一般自我效能感量表", Hint: "请根据自己的实际情况，选择最符合的程度。", Builtin: true,
		Options: []scaleOption{{"完全不正确", 1}, {"有点正确", 2}, {"多数正确", 3}, {"完全正确", 4}},
		Items: []string{
			"如果我尽力去做的话，我总是能够解决问题的",
			"即使别人反对我，我仍有办法取得我所要的",
			"对我来说，坚持理想和达成目标是轻而易举的",
			"我自信能有效地应付任何突如其来的事情",
			"以我的才智，我定能应付意料之外的情况",
			"如果我付出必要的努力，我一定能解决大多数的难题",
			"我能冷静地面对困难，因为我信赖自己处理问题的能力",
			"面对一个难题时，我通常能找到几个解决方法",
			"有麻烦的时候，我通常能想到一些应付的方法",
			"无论什么事在我身上发生，我都能够应付自如",
		},
		Reverse: []int{}, ScoreMode: "sum", // 总分 10-40，越高自我效能感越强
		Thresholds: []scaleThreshold{
			{31, "自我效能感较高", "对自己应对困难的能力有信心，属资源性特质。可给予适度的挑战性任务与班级角色，进一步发挥带动作用。"},
			{21, "自我效能感中等", "自我效能感处于中等水平。可通过小步子目标、及时反馈与成功体验积累，逐步增强其胜任感。"},
			{0, "自我效能感偏低", "自我效能感偏低，遇事容易自我怀疑。建议把任务拆到「跳一跳够得着」的粒度，记录并放大每一次小成功；同时留意是否伴随习得性无助与情绪低落。"},
		},
	},
	{
		ID: "SES", Name: "SES 自尊量表（Rosenberg）", Hint: "请根据自己的实际情况，选择最符合的程度。带 * 题为反向计分。", Builtin: true,
		Options: []scaleOption{{"很不符合", 1}, {"不符合", 2}, {"符合", 3}, {"非常符合", 4}},
		Items: []string{
			"我认为自己是个有价值的人，至少与别人不相上下",
			"我觉得我有许多优点",
			"*总的来说，我倾向于认为自己是一个失败者",
			"我做事可以做得和大多数人一样好",
			"*我觉得自己没有什么值得自豪的地方",
			"我对自己持有一种肯定的态度",
			"整体而言，我对自己感到满意",
			"*我希望自己能为自己赢得更多尊重",
			"*有时我确实感到自己很没用",
			"*我有时认为自己一无是处",
		},
		Reverse: []int{2, 4, 7, 8, 9}, ScoreMode: "sum", // 第 3、5、8、9、10 题反向计分后，总分越高自尊越高
		Thresholds: []scaleThreshold{
			{33, "自尊较高", "自尊水平较高，自我评价稳定积极。保持即可，可引导其成为班级的正向影响力。"},
			{25, "自尊中等", "自尊处于中等水平（该量表无官方分界，按常用三分段解释）。日常多给具体的肯定与成功体验。"},
			{0, "自尊偏低", "自尊水平偏低，自我否定倾向较明显。长期低自尊与抑郁、社交退缩相关，建议谈心了解其自我评价来源，创造可积累成功感的任务，必要时结合 SDS/PHQ-9 评估情绪状态。"},
		},
	},
	{
		ID: "MSPSS", Name: "MSPSS 领悟社会支持量表", Hint: "以下句子描述了你感受到的支持程度，请按实际情况选择符合程度。", Builtin: true,
		Options: []scaleOption{{"极不同意", 1}, {"不同意", 2}, {"有点不同意", 3}, {"中立", 4}, {"有点同意", 5}, {"同意", 6}, {"极同意", 7}},
		Items: []string{
			"在我遇到问题时有些人（领导、亲戚、同事）会出现在我的身旁",
			"我能够与有些人（领导、亲戚、同事）共享快乐与忧伤",
			"我的家庭能够切实具体地给我帮助",
			"在需要时我能够从家庭获得感情上的帮助和支持",
			"当我有困难时有些人（领导、亲戚、同事）是安慰我的真正源泉",
			"我的朋友们能真正帮助我",
			"在发生困难时我可以依靠我的朋友们",
			"我能与自己的家庭谈论我的难题",
			"我的朋友们能与我分享快乐与忧伤",
			"在我的生活中有些人（领导、亲戚、同事）关心着我的感情",
			"我的家庭能心甘情愿协助我做出各种决定",
			"我能与朋友们讨论自己的难题",
		},
		Reverse: []int{}, ScoreMode: "sum", // 总分 12-84，越高感受到的社会支持越充足
		Thresholds: []scaleThreshold{
			{61, "支持充足", "领悟社会支持充足，家庭与朋友系统都是可用资源，是重要的心理保护因素。"},
			{37, "支持一般", "感受到的社会支持处于一般水平。可关注其与家庭、同伴的联结质量，鼓励参与集体活动。"},
			{0, "支持不足", "领悟社会支持不足，遇到困难时可求助的对象有限，是情绪与行为问题的重要风险因子。建议重点建立与学生的信任关系，了解家庭支持状况，必要时家校联动补充支持资源。"},
		},
	},
	{
		ID: "AIS-8", Name: "AIS 阿森斯失眠量表", Hint: "根据最近一个月内的实际睡眠状况，选择每晚至少出现三次的程度。", Builtin: true,
		Options: []scaleOption{{"无", 0}, {"轻度", 1}, {"中度", 2}, {"重度", 3}},
		Items: []string{
			"入睡时间（关灯后到睡着的时间）",
			"夜间苏醒",
			"比期望的时间早醒",
			"总睡眠时间",
			"总睡眠质量（无论睡多长）",
			"白天情绪",
			"白天身体功能（体力或精神：如记忆力、注意力和精力）",
			"白天思睡",
		},
		Reverse: []int{}, ScoreMode: "sum", // 总分 0-24，越高失眠问题越严重
		Thresholds: []scaleThreshold{
			{6, "失眠倾向", "AIS ≥ 6 提示存在失眠问题（该分界在临床筛查中灵敏度较高）。建议了解睡前习惯（手机使用、思虑过多）与白天状态，做睡眠卫生指导；持续 2 周以上建议家长陪同就医评估。"},
			{4, "轻度失眠风险", "存在轻度失眠倾向。先按睡眠卫生建议调整：固定起床时间、睡前 1 小时不用电子产品、床只用于睡觉，两周后复测。"},
			{0, "无明显失眠", "睡眠状况未见明显问题，保持规律作息即可。"},
		},
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

	if len(raw.Options) < 2 || len(raw.Options) > 7 {
		return s, fmt.Errorf("量表「%s」选项需 2-7 个", s.ID)
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
