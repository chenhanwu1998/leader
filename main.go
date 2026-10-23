package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
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
	registerClassRoutes(mux)
	registerStudentRoutes(mux)
	registerScoreRoutes(mux)
	registerAllRecordRoutes(mux)
	registerLayoutRoutes(mux)
	registerStatsRoutes(mux)
	return mux
}

// rootHandler API + 内嵌前端静态资源（SPA 回退到 index.html）
func rootHandler() http.Handler {
	api := apiMux()
	files, _ := fs.Sub(distFS, "web/dist")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
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
	if err := openDB(dir); err != nil {
		log.Fatalf("数据库打开失败: %v", err)
	}
	defer closeDB()

	if *reseed {
		if err := seedDemo(true); err != nil {
			log.Fatalf("灌入演示数据失败: %v", err)
		}
		log.Println("已重新灌入演示数据")
	} else if !*noseed {
		maybeSeed() // 首次启动自动带出演示数据
	}

	if *serve != "" {
		log.Printf("教师台账系统（服务模式）: http://localhost%s", *serve)
		log.Fatal(http.ListenAndServe(*serve, rootHandler()))
	}

	err := wails.Run(&options.App{
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
