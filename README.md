# 教师台账系统 · 桌面版（leader）

[leader-sys](https://github.com/chenhanwu1998/leader-sys)（Vue 3 + Express + better-sqlite3）的桌面应用版：
用 Go 重写了全部后端，前端页面/功能原样迁移，数据仍使用 SQLite（纯 Go 驱动 `modernc.org/sqlite`，无需 gcc），
通过 Wails v2（WebView2）打包为**单文件 Windows 桌面程序 `leader.exe`**，双击即用。

## 功能

与 leader-sys 完全一致：

- 工作台总览（统计卡片、班级人数分布、近 6 个月请假趋势等图表）
- 班级管理、学生花名册（导入/导出 Excel、CSV）
- 成绩管理（考试录入、矩阵录入、成绩分析、分段统计）
- 请假管理、奖惩记录、家校联系（均支持批量导入导出）
- 座位表、课程表（按班级维护）
- 系统设置（科目与颜色配置）、数据备份下载、一键清空

首次启动若无数据，会自动生成一套演示数据（3 个班级 / 160 名学生 / 3 场考试）。

## 运行（免编译）

直接双击 `leader.exe` 即可。数据库保存在 exe 同级的 `data\teacher-ledger.db`
（若 exe 所在目录不可写，则回退到 `%UserConfigDir%\teacher-ledger\data`）。

> 依赖系统自带的 WebView2 运行时（Windows 10/11 默认已装；精简系统请到微软官网下载 WebView2 Runtime）。

## 命令行参数

```
leader.exe              # 桌面窗口模式（默认）
leader.exe -serve       # 纯 HTTP 服务模式，浏览器访问 http://localhost:8530
leader.exe -data <目录>  # 指定数据库目录（默认 exe 同级 data/）
leader.exe -seed        # 启动时强制重新生成演示数据（会先清空现有数据）
leader.exe -noseed      # 启动时不生成演示数据
```

可组合，例如：`leader.exe -serve -data D:\mydata -noseed`

## 从源码构建

环境要求：Go ≥ 1.22、Node ≥ 18（仅构建前端用）。无需 gcc、无需安装 Wails CLI。

```bash
# 1. 构建前端（产物输出到 ../web/dist，供 go:embed 嵌入）
cd frontend
npm install
npm run build
cd ..

# 2. 编译桌面版（注意：desktop,production 两个 build tag 缺一不可，
#    否则运行时会弹出 "Wails applications will not build without the correct build tags"）
CGO_ENABLED=0 go build -tags "desktop,production" \
  -ldflags="-H windowsgui -s -w" -trimpath -o leader.exe .
```

开发调试（HTTP 模式，前端改动用 `cd frontend && npm run dev`，端口 5174，代理到 8530）：

```bash
go run . -serve
```

## 目录结构

```
leader/
├── main.go            # 入口：Wails 窗口 / -serve HTTP 模式，静态资源内嵌
├── db.go              # SQLite 连接、建表、设置存取、科目归一化
├── seed.go            # 演示数据生成
├── handlers_*.go      # REST API（班级/学生/成绩/请假奖惩联系/座位课程表/统计/杂项）
├── frontend/          # Vue 3 + Element Plus + ECharts 前端（从 leader-sys 迁移）
│   └── vite.config.ts # 构建输出到 ../web/dist，base: './'，hash 路由
├── web/dist/          # 前端构建产物（go:embed 嵌入，git 已忽略）
└── data/              # 运行时数据库目录（git 已忽略）
```
