package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	Radix "github.com/fzzy/radix/extra/pool"
	"github.com/go-martini/martini"
	_ "github.com/go-sql-driver/mysql"
	"github.com/martini-contrib/render"
	"github.com/martini-contrib/sessions"
	goCache "github.com/pmylund/go-cache"
	"github.com/walf443/stopwatch"
)

var db *sql.DB
var (
	UserLockThreshold int
	IPBanThreshold    int
)
var gocache = goCache.New(30*time.Second, 10*time.Second)
var radix *Radix.Pool

func init() {
	var dsn string
	if getEnv("ISU4_DB_SOCK", "") == "" {
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local",
			getEnv("ISU4_DB_USER", "isucon"),
			getEnv("ISU4_DB_PASSWORD", "isucon"),
			getEnv("ISU4_DB_HOST", "localhost"),
			getEnv("ISU4_DB_PORT", "3306"),
			getEnv("ISU4_DB_NAME", "isu4_qualifier"),
		)
	} else {
		dsn = fmt.Sprintf(
			"%s:%s@unix(%s)/%s?parseTime=true&loc=Local",
			getEnv("ISU4_DB_USER", "isucon"),
			getEnv("ISU4_DB_PASSWORD", "isucon"),
			getEnv("ISU4_DB_SOCK", ""),
			getEnv("ISU4_DB_NAME", "isu4_qualifier"),
		)
		//dsn := "root@unix(/var/lib/mysql/mysql.sock)/isu4_qualifier?parseTime=true&loc=Local"
	}
	log.Printf("%s", dsn)

	var err error

	db, err = sql.Open("mysql", dsn)
	if err != nil {
		panic(err)
	}

	UserLockThreshold, err = strconv.Atoi(getEnv("ISU4_USER_LOCK_THRESHOLD", "3"))
	if err != nil {
		panic(err)
	}

	IPBanThreshold, err = strconv.Atoi(getEnv("ISU4_IP_BAN_THRESHOLD", "10"))
	if err != nil {
		panic(err)
	}
}

var port = flag.Uint("port", 0, "port to listen")
var redisport = flag.Uint("redis", 0, "port to redis")

func main() {
	m := martini.Classic()
	flag.Parse()

	var err error
	if *redisport == 0 {
		radix, err = Radix.NewPool("unix", "/tmp/redis.sock", 10)
	} else {
		radix, err = Radix.NewPool("tcp", fmt.Sprintf(":%d", *redisport), 10)
	}

	store := sessions.NewCookieStore([]byte("secret-isucon"))
	m.Use(sessions.Sessions("isucon_go_session", store))

	m.Use(martini.Static("../public"))
	m.Use(render.Renderer(render.Options{
		Layout: "layout",
	}))

	m.Get("/", func(string, session sessions.Session) string {
		flash := getFlash(session, "notice")
		if flash == "" {
			return `<!DOCTYPE html><html><head><meta charset="UTF-8"><link rel="stylesheet" href="/stylesheets/bootstrap.min.css"><link rel="stylesheet" href="/stylesheets/bootflat.min.css"><link rel="stylesheet" href="/stylesheets/isucon-bank.css"><title>isucon4</title></head><body><div class="container"><h1 id="topbar"><a href="/"><img src="/images/isucon-bank.png" alt="いすこん銀行 オンラインバンキングサービス"></a></h1><div id="be-careful-phising" class="panel panel-danger"><div class="panel-heading"><span class="hikaru-mozi">偽画面にご注意ください！</span></div><div class="panel-body"><p>偽のログイン画面を表示しお客様の情報を盗み取ろうとする犯罪が多発しています。</p><p>ログイン直後にダウンロード中や、見知らぬウィンドウが開いた場合、<br>すでにウィルスに感染している場合がございます。即座に取引を中止してください。</p><p>また、残高照会のみなど、必要のない場面で乱数表の入力を求められても、<br>絶対に入力しないでください。</p></div></div><div class="page-header"><h1>ログイン</h1></div><div class="container"><form class="form-horizontal" role="form" action="/login" method="POST"><div class="form-group"><label for="input-username" class="col-sm-3 control-label">お客様ご契約ID</label><div class="col-sm-9"><input id="input-username" type="text" class="form-control" placeholder="半角英数字" name="login"></div></div><div class="form-group"><label for="input-password" class="col-sm-3 control-label">パスワード</label><div class="col-sm-9"><input type="password" class="form-control" id="input-password" name="password" placeholder="半角英数字・記号（２文字以上）"></div></div><div class="form-group"><div class="col-sm-offset-3 col-sm-9"><button type="submit" class="btn btn-primary btn-lg btn-block">ログイン</button></div></div></form></div></div></body></html>`
		} else {
			return `<!DOCTYPE html><html><head><meta charset="UTF-8"><link rel="stylesheet" href="/stylesheets/bootstrap.min.css"><link rel="stylesheet" href="/stylesheets/bootflat.min.css"><link rel="stylesheet" href="/stylesheets/isucon-bank.css"><title>isucon4</title></head><body><div class="container"><h1 id="topbar"><a href="/"><img src="/images/isucon-bank.png" alt="いすこん銀行 オンラインバンキングサービス"></a></h1><div id="be-careful-phising" class="panel panel-danger"><div class="panel-heading"><span class="hikaru-mozi">偽画面にご注意ください！</span></div><div class="panel-body"><p>偽のログイン画面を表示しお客様の情報を盗み取ろうとする犯罪が多発しています。</p><p>ログイン直後にダウンロード中や、見知らぬウィンドウが開いた場合、<br>すでにウィルスに感染している場合がございます。即座に取引を中止してください。</p><p>また、残高照会のみなど、必要のない場面で乱数表の入力を求められても、<br>絶対に入力しないでください。</p></div></div><div class="page-header"><h1>ログイン</h1></div><div id="notice-message" class="alert alert-danger" role="alert">` + flash + `</div><div class="container"><form class="form-horizontal" role="form" action="/login" method="POST"><div class="form-group"><label for="input-username" class="col-sm-3 control-label">お客様ご契約ID</label><div class="col-sm-9"><input id="input-username" type="text" class="form-control" placeholder="半角英数字" name="login"></div></div><div class="form-group"><label for="input-password" class="col-sm-3 control-label">パスワード</label><div class="col-sm-9"><input type="password" class="form-control" id="input-password" name="password" placeholder="半角英数字・記号（２文字以上）"></div></div><div class="form-group"><div class="col-sm-offset-3 col-sm-9"><button type="submit" class="btn btn-primary btn-lg btn-block">ログイン</button></div></div></form></div></div></body></html>`
		}
	})

	m.Post("/login", func(req *http.Request, r render.Render, session sessions.Session) {
		stopwatch.Reset("POST /login")
		user, err := attemptLogin(req)
		stopwatch.Watch("after attemptLogin")

		notice := ""
		if err != nil || user == nil {
			switch err {
			case ErrBannedIP:
				notice = "You're banned."
			case ErrLockedUser:
				notice = "This account is locked."
			default:
				notice = "Wrong username or password"
			}

			session.Set("notice", notice)
			r.Redirect("/")
			return
		}

		stopwatch.Watch("before session set")
		session.Set("user_id", strconv.Itoa(user.ID))
		stopwatch.Watch("after session set")
		r.Redirect("/mypage")
	})

	m.Get("/mypage", func(r render.Render, session sessions.Session) {
		currentUser := getCurrentUser(session.Get("user_id"))

		if currentUser == nil {
			session.Set("notice", "You must be logged in")
			r.Redirect("/")
			return
		}

		currentUser.getLastLogin()
		r.HTML(200, "mypage", currentUser)
	})

	m.Get("/report", func(r render.Render) {
		r.JSON(200, map[string][]string{
			"banned_ips":   bannedIPs(),
			"locked_users": lockedUsers(),
		})
	})

	m.Get("/init", func(r render.Render, session sessions.Session) {
		initialize()
	})

	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, os.Interrupt)
	signal.Notify(sigchan, syscall.SIGTERM)
	signal.Notify(sigchan, syscall.SIGINT)

	var l net.Listener
	if *port == 0 {
		ferr := os.Remove("/dev/shm/server.sock")
		if ferr != nil {
			if !os.IsNotExist(ferr) {
				panic(ferr.Error())
			}
		}
		l, err = net.Listen("unix", "/dev/shm/server.sock")
		os.Chmod("/dev/shm/server.sock", 0777)
	} else {
		l, err = net.ListenTCP("tcp", &net.TCPAddr{Port: int(*port)})
	}
	if err != nil {
		panic(err.Error())
	}
	go func() {
		log.Println(http.Serve(l, m))
	}()

	<-sigchan
}

func initialize() {
	log.Println("initialize start")

	gocache.Flush()

	setFailureCacheFromDB()

	log.Println("initialize end")
}

func setFailureCacheFromDB() {
	rows, err := db.Query("SELECT DISTINCT ip FROM login_log")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var ip string
		rows.Scan(&ip)
		cnt, err := getFailureCountFromDB(ip)
		if err != nil {
			log.Fatal(err)
		}
		setFailureCount(ip, cnt)
	}
}
