package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailswindows "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:web/dist
var distFS embed.FS

func apiMux() *http.ServeMux {
	mux := http.NewServeMux()
	registerMiscRoutes(mux)
	registerAuthRoutes(mux)
	registerAdminRoutes(mux)
	registerClassRoutes(mux)
	registerStudentRoutes(mux)
	registerScoreRoutes(mux)
	registerAllRecordRoutes(mux)
	registerLayoutRoutes(mux)
	registerStatsRoutes(mux)
	registerPsyRoutes(mux)
	registerAgentRoutes(mux)
	return mux
}

// rootHandler API + 内嵌前端静态资源（SPA 回退到 index.html）
// 桌面模式前端会探测回环 HTTP 端口直连 API（跨源），这里为 /api 放开 CORS
func rootHandler() http.Handler {
	api := authMiddleware(apiMux())
	files, _ := fs.Sub(distFS, "web/dist")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if r.Header.Get("Origin") != "" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				if r.Method == http.MethodOptions {
					w.WriteHeader(204)
					return
				}
			}
			api.ServeHTTP(w, r)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if f, err := files.Open(p); err == nil {
			f.Close()
			http.FileServer(http.FS(files)).ServeHTTP(w, r)
			return
		}
		// SPA 回退
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		http.FileServer(http.FS(files)).ServeHTTP(w, r2)
	})
	return wrapErr(handler)
}

// 桌面模式下的回环 API：18520 起连续探测 10 个端口，绑定成功的端口由前端探测发现
func startLoopbackAPI() {
	for port := 18520; port < 18530; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		log.Printf("本机回环 API: http://127.0.0.1:%d", port)
		_ = os.WriteFile(filepath.Join(baseDataDir, "loopback.port"), []byte(fmt.Sprint(port)), 0o644)
		if err := http.Serve(ln, rootHandler()); err != nil {
			log.Printf("回环 API 退出: %v", err)
		}
		return
	}
	log.Println("回环端口均被占用，桌面内将使用内置通道（流式回复降级为整段返回）")
}

func main() {
	serve := flag.String("serve", "", "以 HTTP 服务模式运行（如 :8520），不打开桌面窗口")
	dataDirFlag := flag.String("data", "", "数据目录（默认 exe 同级 data/）")
	reseed := flag.Bool("seed", false, "清空现有数据并重新灌入演示数据")
	noseed := flag.Bool("noseed", false, "首次启动不自动灌入演示数据")
	flag.Parse()

	dir := *dataDirFlag
	if dir == "" {
		dir = dbDir()
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("数据目录创建失败: %v", err)
	}
	baseDataDir = dir

	if err := initUsersDB(dir); err != nil {
		log.Fatalf("账号库打开失败: %v", err)
	}
	seedUsers() // 初始化账号（lenhar / admin），迁移旧单库数据

	// 默认账号的台账库（旧的单库数据已归到 lenhar 名下）
	boot, err := ledgerFor(bootUser)
	if err != nil {
		log.Fatalf("台账库打开失败: %v", err)
	}

	if *reseed {
		if err := seedDemo(boot, true); err != nil {
			log.Fatalf("灌入演示数据失败: %v", err)
		}
		log.Println("已重新灌入演示数据")
	} else if !*noseed {
		maybeSeed(boot) // 首次启动自动带出演示数据
	}
	maybeSeedPsy(boot) // 心理板块演示数据（只做一次）

	if *serve != "" {
		log.Printf("教师台账系统（服务模式）: http://localhost%s", *serve)
		log.Fatal(http.ListenAndServe(*serve, rootHandler()))
	}

	// 桌面模式：Wails 内嵌通道不支持 SSE flush，另起本机回环监听承载 API，
	// 前端启动时探测该端口直连（真流式、下载、导出都走这里）。找不到端口时
	// 前端退回同源通道（回复改为整段返回，功能不失效）。
	go startLoopbackAPI()

	applyWindowIcon("教师台账系统")

	err = wails.Run(&options.App{
		Title:     "教师台账系统",
		Width:     1366,
		Height:    860,
		MinWidth:  1080,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Handler: rootHandler(),
		},
		BackgroundColour: &options.RGBA{R: 246, G: 247, B: 251, A: 1},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "teacher-ledger-desktop-8f3c2a",
		},
		Windows: &wailswindows.Options{
			Theme: wailswindows.SystemDefault,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
