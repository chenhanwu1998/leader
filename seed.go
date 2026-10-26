package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
)

// 可复现的伪随机数（与 leader-sys scripts/seed.mjs 同款 mulberry32）
type rng struct{ a uint32 }

func newRng(seed uint32) *rng { return &rng{a: seed} }
func (r *rng) next() float64 {
	r.a += 0x6D2B79F5
	t := r.a
	t = (t ^ (t >> 15)) * (1 | t)
	t = (t + ((t ^ (t >> 7)) * (61 | t))) ^ t
	return float64(t^(t>>14)) / 4294967296.0
}
func (r *rng) pickOne(arr []string) string { return arr[int(math.Floor(r.next()*float64(len(arr))))] }
func (r *rng) below(n int) int             { return int(math.Floor(r.next() * float64(n))) }

var fatherNames = []string{"建国", "志强", "文博", "海涛", "宏伟", "国栋", "永强", "家明", "立军", "德福"}
var motherNames = []string{"秀兰", "桂芳", "玉梅", "丽华", "淑珍", "雅芝", "春燕", "晓红", "慧敏", "婷婷"}
var estates = []string{"阳光小区", "滨河花园", "文华苑", "书香门第", "幸福里", "朝阳新村", "绿城公寓", "和平家园"}

var surnames = strings.Split("王李张刘陈杨赵黄周吴徐孙马朱胡郭何林罗郑梁谢宋唐许韩冯邓曹彭曾肖田董潘袁蔡蒋余于杜叶程苏魏吕丁任沈姚卢姜崔钟谭陆汪范金石廖贾夏韦付方白邹孟熊秦邱江尹薛闫段雷侯龙史陶黎贺顾毛郝龚邵万钱严覃武戴莫孔向汤", "")

var maleGiven = []string{
	"子轩", "浩然", "宇航", "子墨", "俊杰", "明轩", "天佑", "嘉懿", "子豪", "昊天", "梓睿", "一鸣", "俊熙", "润泽",
	"浩宇", "志强", "文博", "海涛", "宏伟", "宇辰", "瑞霖", "铭泽", "景行", "思远", "奕辰", "沐宸", "星辰", "皓轩",
	"雨泽", "梓豪", "家豪", "伟杰", "文轩", "嘉豪", "志远", "哲宇", "昊然", "晟睿", "胤祥", "睿哲", "锦程", "卓然",
	"君浩", "鸿涛", "烨磊", "擎宇", "致远", "俊驰", "鹤轩", "博文", "昊强", "伟宸", "越彬", "晟棋", "明哲", "立诚",
}

var femaleGiven = []string{
	"诗涵", "雨桐", "欣怡", "芷若", "梦琪", "雅静", "雪晴", "佳颖", "婉婷", "若曦", "雪莹", "思琪", "欣悦", "语嫣",
	"诗雨", "静怡", "雨薇", "梦洁", "晓彤", "雅芝", "欣彤", "语桐", "芷晴", "思涵", "雨欣", "佳琪", "静雯", "若彤",
	"梓萱", "可欣", "梦瑶", "紫涵", "雨嘉", "欣然", "雅雯", "慧妍", "曼玉", "若楠", "晨曦", "心怡", "嘉怡", "晓萱",
	"沛玲", "语琴", "听兰", "梦璐", "惠茜", "漫妮", "檀雅", "凌薇", "美琳", "欢馨", "优璇", "雨嘉", "娅楠", "明美",
}

func makeRoster(r *rng, count int) [][2]string {
	used := map[string]bool{}
	out := [][2]string{}
	for guard := 0; len(out) < count && guard < 5000; guard++ {
		male := r.next() < 0.52
		given := femaleGiven
		if male {
			given = maleGiven
		}
		name := r.pickOne(surnames) + r.pickOne(given)
		if used[name] {
			continue
		}
		used[name] = true
		g := "女"
		if male {
			g = "男"
		}
		out = append(out, [2]string{name, g})
	}
	return out
}

type seedClass struct {
	name, grade, headTeacher, classroom, remark string
	birthYear, size                             int
}

var seedClasses = []seedClass{
	{"高一1班", "高一", "王丽华", "教学楼4层401", "实验班", 2010, 53},
	{"高一2班", "高一", "李国强", "教学楼4层402", "", 2010, 55},
	{"高一3班", "高一", "张敏", "教学楼4层403", "", 2010, 52},
}

var seedSubjects = []Subject{
	{"语文", "#fef0f0", "陈静"}, {"数学", "#eef3ff", "刘国栋"}, {"英语", "#f4f0ff", "赵敏"},
	{"物理", "#effaf7", "孙建军"}, {"化学", "#fef6ec", "周海燕"}, {"生物", "#edf9ff", "吴丽萍"},
	{"政治", "#fdf2e8", "郑永刚"}, {"历史", "#f7efe2", "王文博"}, {"地理", "#eef9f0", "林晓东"},
	{"体育", "#fff7e0", "马腾飞"}, {"音乐", "#fdeef5", "许雅琴"}, {"美术", "#f0fbff", "苏曼"},
	{"信息", "#eef1f8", "高翔"},
}

var scoreFull = []struct {
	name string
	full float64
}{
	{"语文", 150}, {"数学", 150}, {"英语", 150}, {"物理", 100}, {"化学", 100},
	{"生物", 100}, {"政治", 100}, {"历史", 100}, {"地理", 100},
}

var seedExams = []struct {
	name, date, term, typ, className, remark string
}{
	{"高一入学摸底测试", "2026-09-05", "2026-2027学年第一学期", "摸底", "", "入学学情摸底"},
	{"高一9月月考", "2026-09-29", "2026-2027学年第一学期", "月考", "", ""},
	{"高一上学期期中考试", "2026-10-30", "2026-2027学年第一学期", "期中", "", "九科联考"},
}

// 记录引用方式：[班级序号, 班内学号序号, ...字段]
var seedLeaves = [][]any{
	{0, 4, "事假", "2026-09-08", "2026-09-08", 1, "家中有事，晚一天报到", "已销假"},
	{0, 1, "病假", "2026-09-11", "2026-09-12", 2, "感冒发烧", "已销假"},
	{0, 27, "事假", "2026-09-15", "2026-09-15", 1, "家中有事", "已销假"},
	{1, 2, "病假", "2026-09-18", "2026-09-19", 2, "急性肠胃炎", "已销假"},
	{1, 18, "事假", "2026-09-22", "2026-09-23", 2, "随父母参加亲戚婚礼", "已销假"},
	{2, 30, "病假", "2026-09-25", "2026-09-26", 2, "过敏性鼻炎", "已销假"},
	{0, 8, "病假", "2026-10-09", "2026-10-10", 2, "发烧", "已销假"},
	{2, 12, "病假", "2026-10-13", "2026-10-14", 2, "咽炎", "已销假"},
	{1, 40, "事假", "2026-10-13", "2026-10-13", 1, "牙齿矫正复诊", "已销假"},
	{2, 5, "病假", "2026-10-15", "2026-10-16", 2, "感冒", "已销假"},
	{0, 33, "其他", "2026-10-17", "2026-10-17", 1, "代表学校参加市青少年科技创新大赛", "已销假"},
	{1, 9, "病假", "2026-10-20", "2026-10-21", 2, "感冒发烧", "已销假"},
	{0, 45, "事假", "2026-10-21", "2026-10-21", 1, "家中有事", "已销假"},
	{2, 20, "病假", "2026-10-22", "2026-10-24", 3, "支原体感染，需输液治疗", "请假中"},
	{1, 25, "事假", "2026-10-23", "2026-10-24", 2, "随父母外出探亲", "请假中"},
}

var seedRewards = [][]any{
	{0, 15, "表扬", "班级", "作业书写工整", "2026-09-10", "连续两周作业评为优秀范例"},
	{0, 0, "表扬", "班级", "课堂发言积极", "2026-09-12", "主动举手回答问题，思路清晰"},
	{1, 31, "表扬", "校级", "拾金不昧", "2026-09-16", "在操场捡到运动相机主动上交"},
	{0, 6, "奖励", "市级", "市高中数学竞赛二等奖", "2026-09-24", "开学即代表学校参赛获奖"},
	{1, 0, "表扬", "班级", "热心帮助同学", "2026-09-25", "主动利用课间给同桌讲解物理题"},
	{2, 8, "表扬", "班级", "主动整理图书角", "2026-09-28", "坚持每周整理班级图书角"},
	{0, 20, "奖励", "县区", "区三好学生", "2026-09-30", "初中阶段综合表现突出"},
	{1, 44, "批评", "班级", "课堂打闹", "2026-10-09", "自习课上与同桌打闹，影响课堂秩序"},
	{2, 2, "表扬", "班级", "朗读展示优秀", "2026-10-10", "语文早读领读示范"},
	{0, 38, "奖励", "校级", "英语演讲比赛二等奖", "2026-10-14", "校英语风采大赛高一组二等奖"},
	{1, 13, "表扬", "班级", "学习成绩进步显著", "2026-10-15", "9月月考较摸底测试总分提升46分"},
	{2, 33, "批评", "班级", "作业多次拖欠", "2026-10-16", "一周内三次未交数学作业"},
	{0, 50, "表扬", "班级", "卫生标兵", "2026-10-17", "值日认真负责，台面地面整洁"},
	{1, 48, "奖励", "校级", "校运会1500米第一名", "2026-10-18", "秋季校运会高一男子1500米冠军"},
	{2, 15, "批评", "班级", "多次忘带课本", "2026-10-20", "本周三次忘带课本，已与家长沟通"},
	{1, 52, "批评", "班级", "晚自习喧哗", "2026-10-21", "晚自习与邻座讲话，提醒后仍未改正"},
}

var seedContacts = [][]any{
	{0, 1, "2026-09-12", "电话", "家长", "反馈近期数学计算错误较多，建议每天坚持20分钟限时训练", "家长会配合督促，每天检查作业"},
	{0, 27, "2026-09-15", "微信", "家长", "提醒流感疫苗接种时间，回执单请周四前交回", "已预约周六接种"},
	{1, 18, "2026-09-22", "面谈", "家长", "约谈课堂纪律问题，孩子近期上课注意力分散", "家长表示会配合教育，减少电子产品使用"},
	{1, 31, "2026-09-26", "家访", "家长", "了解在家学习环境，孩子每天坚持阅读半小时", "家庭学习氛围良好"},
	{2, 30, "2026-09-30", "电话", "家长", "表扬孩子乐于助人，主动帮助学习困难的同学", "家长表示感谢老师的关注"},
	{2, 33, "2026-10-10", "电话", "家长", "反馈作业拖欠情况，商定家校共同检查机制", "家长承诺每晚检查作业完成情况并签字"},
	{0, 6, "2026-10-14", "电话", "家长", "报喜：孩子获得市数学竞赛二等奖", "家长很高兴，感谢老师培养"},
	{2, 2, "2026-10-15", "微信", "家长", "提醒补交请假手续（书面请假条）", "明天让孩子带回"},
	{1, 40, "2026-10-17", "家访", "家长", "交流升入高中后的学习适应情况", "孩子适应良好，家长希望关注偏科问题"},
	{0, 4, "2026-10-19", "电话", "家长", "提醒按时接送，近期有两次晚自习放学等待时间较长", "会调整工作安排，准时接送"},
	{0, 45, "2026-10-20", "微信", "学生", "沟通课后答疑安排，鼓励其参加数学竞赛兴趣小组", "学生表示愿意参加"},
	{2, 20, "2026-10-23", "微信", "家长", "询问支原体感染恢复情况，提醒返校后带病历和缓考申请", "预计下周一返校，会带好相关材料"},
}

// 高一1班课程表（cells 键为 “星期-节次”，均从 0 起）
var seedSchedule = map[string]any{
	"days":  []int{0, 1, 2, 3, 4},
	"times": []string{"早读", "第1节", "第2节", "第3节", "第4节", "第5节", "第6节", "第7节", "第8节", "晚修1", "晚修2"},
	"timeNotes": []string{"07:20-07:50", "07:50-08:30", "08:40-09:20", "09:40-10:20", "10:30-11:10",
		"14:00-14:40", "14:50-15:30", "15:50-16:30", "16:40-17:20", "19:00-19:45", "19:55-20:40"},
	"breaksAfter": []int{5, 9},
	"cells": map[string]string{
		"0-0": "语文", "0-1": "语文", "0-2": "数学", "0-3": "英语", "0-4": "物理", "0-5": "化学", "0-6": "体育", "0-7": "自习", "0-8": "班会", "0-9": "自习", "0-10": "自习",
		"1-0": "英语", "1-1": "数学", "1-2": "英语", "1-3": "语文", "1-4": "生物", "1-5": "地理", "1-6": "政治", "1-7": "体育", "1-8": "自习", "1-9": "自习", "1-10": "自习",
		"2-0": "语文", "2-1": "英语", "2-2": "语文", "2-3": "数学", "2-4": "物理", "2-5": "历史", "2-6": "化学", "2-7": "美术", "2-8": "自习", "2-9": "自习", "2-10": "自习",
		"3-0": "英语", "3-1": "数学", "3-2": "语文", "3-3": "英语", "3-4": "生物", "3-5": "物理", "3-6": "政治", "3-7": "信息", "3-8": "自习", "3-9": "自习", "3-10": "自习",
		"4-0": "语文", "4-1": "语文", "4-2": "数学", "4-3": "地理", "4-4": "历史", "4-5": "化学", "4-6": "音乐", "4-7": "体育", "4-8": "自习", "4-9": "自习", "4-10": "自习",
	},
}

type seededStudent struct {
	id   int64
	name string
}

func clearAllTables(tx *sql.Tx) error {
	for _, t := range []string{"scores", "leaves", "rewards", "contacts", "exams", "students", "classes", "seat_layouts", "schedules", "class_subjects", "psy_sessions", "psy_assessments", "psy_profiles", "agent_messages", "agent_conversations"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return err
		}
	}
	return nil
}

// seedDemo 灌入演示数据：高一 3 个班（每班 50+ 人）、九科成绩、请假/奖惩/家校、座位表、课程表
func seedDemo(d *sql.DB, force bool) error {
	if !force {
		if c := queryCount(d, `SELECT COUNT(*) c FROM students`); c > 0 {
			return fmt.Errorf("数据库已有 %d 名学生，未做改动", c)
		}
	}
	r := newRng(20261023)

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if force {
		if err := clearAllTables(tx); err != nil {
			return err
		}
	}

	// 基础设置
	for _, kv := range [][2]string{
		{"teacher_name", "王丽华"},
		{"term", "2026-2027学年第一学期"},
	} {
		if _, err := tx.Exec(`INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, kv[0], kv[1]); err != nil {
			return err
		}
	}
	subJSON, _ := json.Marshal(seedSubjects)
	if _, err := tx.Exec(`INSERT INTO settings(key, value) VALUES('subjects', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(subJSON)); err != nil {
		return err
	}

	// 班级
	insClass, err := tx.Prepare(`INSERT INTO classes(name, grade, head_teacher, classroom, remark) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, c := range seedClasses {
		if _, err := insClass.Exec(c.name, c.grade, c.headTeacher, c.classroom, c.remark); err != nil {
			return err
		}
	}
	insClass.Close()

	// 学生（学号为班内序号）
	insStudent, err := tx.Prepare(`INSERT INTO students(student_no, name, gender, class_name, birth_date, phone, parent_name, parent_phone, address, status, remark)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	classStudents := make([][]seededStudent, len(seedClasses))
	studentCount := 0
	phonePrefixes := []string{"30", "35", "37", "38", "39", "50", "51", "55", "56", "58", "86", "88"}
	for ci, c := range seedClasses {
		roster := makeRoster(r, c.size)
		ids := []seededStudent{}
		for i, item := range roster {
			name, gender := item[0], item[1]
			birthYear := c.birthYear
			if r.next() >= 0.85 {
				birthYear--
			}
			birth := fmt.Sprintf("%d-%02d-%02d", birthYear, 1+r.below(12), 1+r.below(28))
			surname := string([]rune(name)[0])
			parentName := surname + motherNames[r.below(len(motherNames))]
			if r.next() < 0.55 {
				parentName = surname + fatherNames[r.below(len(fatherNames))]
			}
			phone := fmt.Sprintf("1%s%08d", r.pickOne(phonePrefixes), r.below(100000000))
			parentPhone := fmt.Sprintf("1%s%08d", r.pickOne(phonePrefixes), r.below(100000000))
			address := fmt.Sprintf("%s%d栋%d单元%d0%d室", r.pickOne(estates), 1+r.below(20), 1+r.below(4), 1+r.below(18), 1+r.below(9))
			if r.next() >= 0.3 {
				phone = ""
			}
			res, err := insStudent.Exec(fmt.Sprintf("%d", i+1), name, gender, c.name, birth, phone, parentName, parentPhone, address, "在读", "")
			if err != nil {
				return err
			}
			id, _ := res.LastInsertId()
			ids = append(ids, seededStudent{id, name})
			studentCount++
		}
		classStudents[ci] = ids
	}
	insStudent.Close()
	stuAt := func(ci, ni int) seededStudent { return classStudents[ci][ni] }

	// 考试 + 成绩（语数英满分 150，其余 100；带个体内差异与逐次微升趋势）
	insExam, err := tx.Prepare(`INSERT INTO exams(name, exam_date, term, type, class_name, remark) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	insScore, err := tx.Prepare(`INSERT INTO scores(exam_id, student_id, subject, score, full_score) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	scoreCount := 0
	ability := map[int64]float64{}
	for _, list := range classStudents {
		for _, s := range list {
			ability[s.id] = 0.58 + r.next()*0.34
		}
	}
	for examIdx, e := range seedExams {
		res, err := insExam.Exec(e.name, e.date, e.term, e.typ, e.className, e.remark)
		if err != nil {
			return err
		}
		examID, _ := res.LastInsertId()
		for _, list := range classStudents {
			for _, s := range list {
				ab := ability[s.id] + float64(examIdx)*0.012
				for _, sub := range scoreFull {
					score := math.Round(sub.full * (ab + (r.next()*0.16 - 0.08)))
					score = math.Max(math.Round(sub.full*0.35), math.Min(sub.full, score))
					if _, err := insScore.Exec(examID, s.id, sub.name, score, sub.full); err != nil {
						return err
					}
					scoreCount++
				}
			}
		}
	}
	insExam.Close()
	insScore.Close()

	// 请假
	insLeave, err := tx.Prepare(`INSERT INTO leaves(student_id, type, start_date, end_date, days, reason, status, remark) VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, l := range seedLeaves {
		s := stuAt(l[0].(int), l[1].(int))
		if _, err := insLeave.Exec(s.id, l[2], l[3], l[4], l[5], l[6], l[7], ""); err != nil {
			return err
		}
	}
	insLeave.Close()

	// 奖惩
	insReward, err := tx.Prepare(`INSERT INTO rewards(student_id, kind, level, title, record_date, reason, remark) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, rw := range seedRewards {
		s := stuAt(rw[0].(int), rw[1].(int))
		if _, err := insReward.Exec(s.id, rw[2], rw[3], rw[4], rw[5], rw[6], ""); err != nil {
			return err
		}
	}
	insReward.Close()

	// 家校联系
	insContact, err := tx.Prepare(`INSERT INTO contacts(student_id, contact_date, way, target, content, feedback, remark) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, ct := range seedContacts {
		s := stuAt(ct[0].(int), ct[1].(int))
		if _, err := insContact.Exec(s.id, ct[2], ct[3], ct[4], ct[5], ct[6], ""); err != nil {
			return err
		}
	}
	insContact.Close()

	// 座位表：高一1班 7×8，按学号顺序就座
	c1 := classStudents[0]
	assignments := make([]map[string]any, 0, len(c1))
	for i, s := range c1 {
		assignments = append(assignments, map[string]any{"row": i / 8, "col": i % 8, "student_id": s.id})
	}
	assignJSON, _ := json.Marshal(assignments)
	if _, err := tx.Exec(`INSERT INTO seat_layouts(class_name, rows, cols, group_size, assignments) VALUES (?,?,?,?,?)`,
		seedClasses[0].name, 7, 8, 2, string(assignJSON)); err != nil {
		return err
	}

	// 课程表：高一1班
	schedJSON, _ := json.Marshal(seedSchedule)
	if _, err := tx.Exec(`INSERT INTO schedules(class_name, config) VALUES (?,?)`, seedClasses[0].name, string(schedJSON)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	setSetting(d, "seeded", "1")
	log.Printf("演示数据：%d 个班级 / %d 名学生 / %d 场考试 %d 条成绩", len(seedClasses), studentCount, len(seedExams), scoreCount)
	return nil
}

// maybeSeed 首次启动（settings 无 seeded 标记）时自动灌入演示数据
func maybeSeed(d *sql.DB) {
	if getSetting(d, "seeded", "") != "" {
		return
	}
	if err := seedDemo(d, false); err != nil {
		log.Printf("演示数据灌入跳过：%v", err)
		setSetting(d, "seeded", "1")
	}
}

// —— 心理板块演示数据（独立于主演示数据灌入，老库升级后首次启动也会补） ——

// 按比例构造原始答案：count 道题、每题范围 [minV,maxV]、总和尽量贴近 target
func psyAnswers(count, minV, target int) []float64 {
	ans := make([]float64, count)
	for i := range ans {
		ans[i] = float64(minV)
	}
	left := target - minV*count
	for i := 0; left > 0; i = (i + 1) % count {
		ans[i]++
		left--
	}
	return ans
}

type psySeedProfile struct {
	idxCls, idxStu      int
	risk, tags, note    string
	sessions            [][]string // date, mode, topic, content, next_follow
	scales              [][2]int   // [量表序号, 目标原始分]
}

// maybeSeedPsy 心理档案/辅导记录/测评的演示数据（只做一次；用户清空数据后不会复活）
func maybeSeedPsy(d *sql.DB) {
	if getSetting(d, "seed_psy_v1", "") == "1" {
		return
	}
	defer setSetting(d, "seed_psy_v1", "1")
	if queryCount(d, `SELECT COUNT(*) c FROM psy_profiles`) > 0 {
		return
	}
	rows, err := queryAll(d, `SELECT id, name, gender, class_name FROM students ORDER BY class_name, CAST(student_no AS INTEGER)`)
	if err != nil || len(rows) < 20 {
		return
	}
	byClass := map[string][]map[string]any{}
	cls := []string{}
	for _, row := range rows {
		c := toStr(row["class_name"])
		if _, ok := byClass[c]; !ok {
			cls = append(cls, c)
		}
		byClass[c] = append(byClass[c], row)
	}
	sort.Strings(cls)
	pick := func(ci, idx int) map[string]any {
		if ci >= len(cls) {
			ci = 0
		}
		list := byClass[cls[ci]]
		if idx >= len(list) {
			return nil
		}
		return list[idx]
	}

	scaleOrder := []string{"PHQ-9", "GAD-7", "SAS", "SDS"}
	scaleItems := map[string]int{"PHQ-9": 9, "GAD-7": 7, "SAS": 20, "SDS": 20}
	scaleMin := map[string]int{"PHQ-9": 0, "GAD-7": 0, "SAS": 1, "SDS": 1}

	plans := []psySeedProfile{
		{
			0, 6, "高危预警", "情绪低落,家庭变故,自我评价低",
			"父母近期离异，上课注意力明显下降，两次被同学发现在厕所哭泣；自我评价低，觉得「自己很多余」。",
			[][]string{
				{"2026-09-15", "面谈", "入学适应与家庭情况了解", "首次谈话建立信任。学生讲述父母暑假离异，目前随母亲生活，父亲较少联系。情绪低落但能正常交流，约定每周谈心一次。", "2026-09-22"},
				{"2026-10-09", "面谈", "情绪低落随访", "近两周睡眠差，常凌晨一两点才能入睡。课堂有发呆现象。教了腹式呼吸放松法，建议规律作息；已与母亲电话沟通其在家的情绪表现。", "2026-10-16"},
				{"2026-10-22", "面谈", "危机筛查后的跟进面谈", "PHQ-9 测评提示中重度抑郁倾向且第9题有分。当面温和核实时学生承认偶有「活着没意思」的念头但无具体计划。已联系监护人建议尽快到专科医院评估，启动重点关注流程，每三日跟进。", "2026-10-29"},
			},
			[][2]int{{0, 15}, {3, 52}},
		},
		{
			1, 9, "重点关注", "考试焦虑,失眠,完美主义",
			"临近月考反复出现心慌、手心出汗，担心「考砸了就完了」，自我要求高，睡前反复检查书包。",
			[][]string{
				{"2026-10-13", "面谈", "考试焦虑初谈", "学生对成绩的灾难化想象明显（一次考差=前途尽毁）。用认知重构练习识别自动思维，布置「担忧记录表」。", "2026-10-20"},
				{"2026-10-24", "面谈", "放松训练与作息调整", "教导渐进式肌肉放松与4-7-8呼吸法，睡前一小时不碰手机。月考临近，与其约定「完成比完美重要」的应考口号。", "2026-10-24"},
			},
			[][2]int{{1, 11}, {2, 48}},
		},
		{
			0, 21, "常规关注", "人际适应,性格内向",
			"性格内向，课间常独处；主动求助想「交朋友但不知道怎么开口」，未见情绪异常。",
			[][]string{
				{"2026-10-17", "面谈", "人际交往困扰", "学生表达交友意愿但缺乏方法。讨论从同桌、小组合作切入，布置「每周一次主动搭话」小任务；鼓励参加班级图书角值班。", ""},
			},
			[][2]int{{0, 5}},
		},
	}

	tx, err := d.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	for _, p := range plans {
		stu := pick(p.idxCls, p.idxStu)
		if stu == nil {
			continue
		}
		res, err := tx.Exec(`INSERT INTO psy_profiles(student_name, class_name, gender, risk_level, tags, note, created_at, updated_at)
			VALUES (?,?,?,?,?,?, datetime('now','localtime','-30 days'), datetime('now','localtime'))`,
			toStr(stu["name"]), toStr(stu["class_name"]), toStr(stu["gender"]), p.risk, p.tags, p.note)
		if err != nil {
			return
		}
		pid, _ := res.LastInsertId()
		for _, s := range p.sessions {
			if _, err := tx.Exec(`INSERT INTO psy_sessions(profile_id, session_date, mode, topic, content, next_follow, created_at)
				VALUES (?,?,?,?,?,?, ? || ' 15:30:00')`, pid, s[0], s[1], s[2], s[3], s[4], s[0]); err != nil {
				return
			}
		}
		for _, sc := range p.scales {
			scale := scaleOrder[sc[0]]
			answers := psyAnswers(scaleItems[scale], scaleMin[scale], sc[1])
			score, level, flagged, err := scoreAssessment(scale, answers)
			if err != nil {
				return
			}
			flag := 0
			if flagged {
				flag = 1
			}
			ansJSON, _ := json.Marshal(answers)
			if _, err := tx.Exec(`INSERT INTO psy_assessments(profile_id, scale, answers, score, level, flagged, created_at)
				VALUES (?,?,?,?,?,?, datetime('now','localtime','-5 days'))`, pid, scale, string(ansJSON), score, level, flag); err != nil {
				return
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return
	}
	log.Println("心理板块演示数据已就绪：3 份档案（含辅导记录与量表测评）")
}
