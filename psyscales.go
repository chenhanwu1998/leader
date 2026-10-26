package main

// 常用心理筛查量表题库与计分规则（PHQ-9 / GAD-7 / SAS / SDS）
// 与 leader-sys 的 server/psy-scales.js 一一对应。

import (
	"errors"
	"math"
)

var freq4 = []string{"完全不会", "好几天", "一半以上的天数", "几乎每天"}
var sasFreq = []string{"没有或很少时间有", "小部分时间有", "相当多时间有", "绝大部分或全部时间有"}

type psychoScale struct {
	ID      string
	Name    string
	Hint    string
	Options []string
	Items   []string
	Min     int
	Max     int
	Reverse map[int]bool
}

func (s *psychoScale) reversed(i int) bool { return s.Reverse[i] }

func scaleSum(answers []float64) float64 {
	var raw float64
	for _, v := range answers {
		raw += v
	}
	return raw
}

var psychoScales = map[string]*psychoScale{
	"PHQ-9": {
		ID:   "PHQ-9",
		Name: "PHQ-9 抑郁筛查量表",
		Hint: "在过去的两周里，以下情况有多少天困扰过你？",
		Options: freq4,
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
		Min: 0, Max: 3,
	},
	"GAD-7": {
		ID:   "GAD-7",
		Name: "GAD-7 焦虑筛查量表",
		Hint: "在过去的两周里，以下情况有多少天困扰过你？",
		Options: freq4,
		Items: []string{
			"感觉紧张、焦虑或急切",
			"不能够停止或控制担忧",
			"对各种各样的事情担忧过多",
			"很难放松下来",
			"由于不安而无法静坐",
			"变得容易烦恼或急躁",
			"感到似乎将有可怕的事情发生而害怕",
		},
		Min: 0, Max: 3,
	},
	"SAS": {
		ID:   "SAS",
		Name: "SAS 焦虑自评量表",
		Hint: "根据最近一周的实际情况，选择最符合的程度。带 * 题为反向计分。",
		Options: sasFreq,
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
		Min: 1, Max: 4,
		Reverse: map[int]bool{4: true, 8: true, 12: true, 16: true, 18: true},
	},
	"SDS": {
		ID:   "SDS",
		Name: "SDS 抑郁自评量表",
		Hint: "根据最近一周的实际情况，选择最符合的程度。带 * 题为反向计分。",
		Options: sasFreq,
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
		Min: 1, Max: 4,
		Reverse: map[int]bool{1: true, 4: true, 5: true, 10: true, 11: true, 13: true, 15: true, 16: true, 17: true, 19: true},
	},
}

// 前端渲染问卷用的题库清单
func scaleNames() []map[string]any {
	order := []string{"PHQ-9", "GAD-7", "SAS", "SDS"}
	out := make([]map[string]any, 0, len(order))
	for _, id := range order {
		s := psychoScales[id]
		out = append(out, map[string]any{
			"id": s.ID, "name": s.Name, "hint": s.Hint,
			"options": s.Options, "items": s.Items, "min": s.Min, "max": s.Max,
		})
	}
	return out
}

// 计分：answers 为各题分值数组。返回 score / level / flagged
func scoreAssessment(scale string, answers []float64) (float64, string, bool, error) {
	s, ok := psychoScales[scale]
	if !ok {
		return 0, "", false, errors.New("未知量表: " + scale)
	}
	if len(answers) != len(s.Items) {
		return 0, "", false, errors.New("答题数量与量表题目数不一致")
	}
	ans := func(i int, def float64) float64 {
		if i < len(answers) {
			return answers[i]
		}
		return def
	}
	switch scale {
	case "PHQ-9":
		raw := scaleSum(answers)
		level := "无明显抑郁症状"
		if raw >= 20 {
			level = "重度抑郁"
		} else if raw >= 15 {
			level = "中重度抑郁"
		} else if raw >= 10 {
			level = "中度抑郁"
		} else if raw >= 5 {
			level = "轻度抑郁"
		}
		// 第9题（自伤念头）≥1 分即标记为需要立即安全谈话
		return raw, level, ans(8, 0) >= 1, nil
	case "GAD-7":
		raw := scaleSum(answers)
		level := "无明显焦虑症状"
		if raw >= 15 {
			level = "重度焦虑"
		} else if raw >= 10 {
			level = "中度焦虑"
		} else if raw >= 5 {
			level = "轻度焦虑"
		}
		return raw, level, false, nil
	case "SAS":
		var raw float64
		for i, v := range answers {
			if v == 0 {
				v = 1
			}
			if s.reversed(i) {
				raw += 5 - v
			} else {
				raw += v
			}
		}
		std := math.Floor(raw * 1.25)
		level := "正常范围"
		if std >= 70 {
			level = "重度焦虑"
		} else if std >= 60 {
			level = "中度焦虑"
		} else if std >= 50 {
			level = "轻度焦虑"
		}
		return std, level, false, nil
	case "SDS":
		var raw float64
		for i, v := range answers {
			if v == 0 {
				v = 1
			}
			if s.reversed(i) {
				raw += 5 - v
			} else {
				raw += v
			}
		}
		std := math.Floor(raw * 1.25)
		level := "正常范围"
		if std >= 73 {
			level = "重度抑郁"
		} else if std >= 63 {
			level = "中度抑郁"
		} else if std >= 53 {
			level = "轻度抑郁"
		}
		// 第19题（「我死了别人会更好」）选 3 或 4 提示无价值感明显，需关注
		return std, level, ans(18, 1) >= 3, nil
	}
	return 0, "", false, errors.New("未知量表: " + scale)
}
