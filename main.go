// 户型装修设计 —— 静态前端 + SQLite 持久化。
//
// 只做三件事：
//  1. 把 index.html 与 static/（本地化的 three.js，不依赖 CDN）内嵌进二进制，任何目录都能直接跑；
//  2. plans 表存整个方案（房间地面 / 家具 / 拆墙 / 风格）的 JSON；
//  3. items 表存「用料与备注清单」，整表替换式保存（行数少，够用且不会出现半更新）。
//
// 前端所有 URL 都是相对路径，所以挂在子路径（-base /floorplan）或反向代理子目录下都能直接用。
package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite" // 纯 Go 版 SQLite 驱动，注册名为 "sqlite"，无需 cgo
)

//go:embed index.html static
var assets embed.FS

const maxBody = 8 << 20 // 8MB，方案 JSON 远小于此

const schema = `
CREATE TABLE IF NOT EXISTS plans (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL DEFAULT '',
  data       TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS items (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  category TEXT    NOT NULL DEFAULT '',
  name     TEXT    NOT NULL DEFAULT '',
  brand    TEXT    NOT NULL DEFAULT '',
  spec     TEXT    NOT NULL DEFAULT '',
  unit     TEXT    NOT NULL DEFAULT '',
  qty      REAL    NOT NULL DEFAULT 0,
  price    REAL    NOT NULL DEFAULT 0,
  note     TEXT    NOT NULL DEFAULT '',
  sort     INTEGER NOT NULL DEFAULT 0
);
`

type item struct {
	Category string  `json:"category"`
	Name     string  `json:"name"`
	Brand    string  `json:"brand"`
	Spec     string  `json:"spec"`
	Unit     string  `json:"unit"`
	Qty      float64 `json:"qty"`
	Price    float64 `json:"price"`
	Note     string  `json:"note"`
	Sort     int     `json:"sort"`
}

// version 由构建时注入：make 会传 -ldflags "-X main.version=1.0.0"。
var version = "dev"

func main() {
	showVer := flag.Bool("version", false, "打印版本号后退出")
	addr := flag.String("addr", ":8787", "监听地址")
	dbPath := flag.String("db", "data/plan.db", "SQLite 文件路径")
	base := flag.String("base", "", "子路径部署前缀，例如 /floorplan（也可留空，交给反向代理剥前缀）")
	noOpen := flag.Bool("no-open", false, "不自动打开浏览器（服务器 / 后台运行时用）")
	flag.Parse()
	if *showVer {
		// 只打版本号，方便脚本解析（make / CI / 用户 -version 对照）
		fmt.Println("floorplan3d " + version)
		return
	}

	db, err := openDB(*dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/plan", func(w http.ResponseWriter, r *http.Request) { planHandler(w, r, db) })
	mux.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) { itemsHandler(w, r, db) })
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	// embed FS 以模块根为根，URL 里的 /static/xxx 正好对应内嵌路径 static/xxx，直接服务即可。
	mux.Handle("/static/", cacheStatic(http.FileServer(http.FS(assets))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		b, _ := assets.ReadFile("index.html")
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.Header().Set("cache-control", "no-store")
		w.Write(b)
	})

	prefix := normalizeBase(*base)
	srv := &http.Server{Addr: *addr, Handler: mount(mux, prefix), ReadHeaderTimeout: 10 * time.Second}
	url := *addr
	if strings.HasPrefix(url, ":") {
		url = "localhost" + url
	}
	log.Printf("户型装修设计 %s: http://%s%s/   数据库: %s", version, url, prefix, *dbPath)
	if !*noOpen {
		openBrowser("http://" + url + prefix + "/") // 失败无所谓，用户自己点日志里的地址
	}
	log.Fatal(srv.ListenAndServe())
}

// normalizeBase 把 "floorplan"、"/floorplan/" 之类的写法统一成 "/floorplan"，空串表示根路径。
func normalizeBase(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "/")
	if s == "" {
		return ""
	}
	return "/" + s
}

// mount 在子路径下挂载整套路由；访问不带尾斜杠的 /floorplan 时 301 到 /floorplan/，
// 否则页面里的相对路径会以根目录为基准，静态资源和 api 全部 404。
func mount(h http.Handler, base string) http.Handler {
	if base == "" {
		return h
	}
	inner := http.StripPrefix(base, h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == base {
			target := base + "/"
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		if !strings.HasPrefix(r.URL.Path, base+"/") {
			http.NotFound(w, r)
			return
		}
		inner.ServeHTTP(w, r)
	})
}

// cacheStatic 给本地化的 three.js 加一天缓存：文件内嵌在二进制里，换个版本重新编译即可，
// 但浏览器可能留旧缓存，所以只给 1 小时而不是一年。
func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("cache-control", "public, max-age=3600")
		h.ServeHTTP(w, r)
	})
}

// openBrowser 双击启动时省掉"自己去浏览器粘地址"这一步。
// 无桌面环境（服务器）时命令不存在，直接忽略错误。
func openBrowser(url string) {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	} else if runtime.GOOS == "windows" {
		cmd = "rundll32"
		url = "url.dll,FileProtocolHandler," + url
	}
	_ = exec.Command(cmd, url).Start()
}

func openDB(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite 单写者，串行化最省心
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

/* ---------------- 方案 ---------------- */

func planHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	switch r.Method {
	case http.MethodGet:
		var data string
		err := db.QueryRow(`SELECT data FROM plans WHERE id='default'`).Scan(&data)
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "application/json; charset=utf-8")
		io.WriteString(w, data)
	case http.MethodPut:
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
		if err != nil || !json.Valid(body) {
			http.Error(w, "方案 JSON 无效", http.StatusBadRequest)
			return
		}
		_, err = db.Exec(`INSERT INTO plans(id,name,data,updated_at) VALUES('default','默认方案',?,?)
			ON CONFLICT(id) DO UPDATE SET data=excluded.data, updated_at=excluded.updated_at`,
			string(body), time.Now().Format(time.RFC3339))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

/* ---------------- 用料清单 ---------------- */

func itemsHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT category,name,brand,spec,unit,qty,price,note,sort FROM items ORDER BY sort, id`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		list := []item{}
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.Category, &it.Name, &it.Brand, &it.Spec, &it.Unit, &it.Qty, &it.Price, &it.Note, &it.Sort); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			list = append(list, it)
		}
		writeJSON(w, list)
	case http.MethodPut:
		var list []item
		if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&list); err != nil {
			http.Error(w, "清单 JSON 无效", http.StatusBadRequest)
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`DELETE FROM items`); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// 清空自增序列，避免 id 无上限增长
		if _, err := tx.Exec(`DELETE FROM sqlite_sequence WHERE name='items'`); err != nil && !strings.Contains(err.Error(), "no such table") {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		stmt, err := tx.Prepare(`INSERT INTO items(category,name,brand,spec,unit,qty,price,note,sort) VALUES(?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()
		for i, it := range list {
			if _, err := stmt.Exec(it.Category, it.Name, it.Brand, it.Spec, it.Unit, it.Qty, it.Price, it.Note, i); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
