package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	"github.com/LTXWorld/greenLight_copy/internal/data"
	"github.com/LTXWorld/greenLight_copy/internal/jsonlog"
	"github.com/LTXWorld/greenLight_copy/internal/mailer"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// 在之后的开发中我们将在build time伴随着git自动地生成这个版本号
var (
	// 表示执行时间，嵌入到二进制包中
	buildTime string
	version   string
)

// 自定义config结构体类型，监听的端口号，当前运行环境，数据库连接池，通过命令行交互
// 加入对于连接池的配置属性来自定义连接池信息
type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	// Add a new limiter struct containing fields for the requests-per-second and burst values
	// and a boolean which we can use to enable/disable rate limiting
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	// Add a new smtp struct containing fields for SMTP server config
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	// Add a cors struct and trustedOrigins field with the type []string
	cors struct {
		trustedOrigins []string
	}
}

// 为HTTP的处理器，辅助代码，中间件保存依赖
type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
}

func main() {
	// 声明config类型的实例
	var cfg config

	// 通过命令行flag交互读取config中的端口值等信息赋值给cfg中的各属性，例如默认端口值为4060
	flag.IntVar(&cfg.port, "port", 4066, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	// Read the 数据源店均从命令行的db-dsn command-line标签到config 结构体中
	//flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")
	//cfg.db.dsn = "postgres://greenlight:iutaol123@localhost/greenlight?sslmode=disable"
	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")
	fmt.Println("Using Database DSN:", cfg.db.dsn)

	// 配置连接池
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	// 从命令行读取关于速率的配置
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	// Read the SMTP server config settings into the config struct,using the Mailtrap settings as the default
	flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "25e5b5841c2992", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "52dac9cb14d90c", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "lutao123050104@gmail.com", "SMTP sender")

	// Use the flag.Func() to process the -cors-trusted-origins command line flag
	// use the strings.Fields将flag value根据空白字符进行分割开
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	// 为version创建一个flag
	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	// if the version flag value is true,打印出版本号以及其他动态信息
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		// Print out the contents of the buildTime variable
		fmt.Printf("Build time:\t%s\n", buildTime)
		os.Exit(0)
	}

	// 使用jsonlog自定义初始化一个日志向标准输出流写信息，将日志封装为json类型
	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	// 调用openDB方法创建连接池
	db, err := openDB(cfg)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	defer db.Close()

	logger.PrintInfo("database connection pool established", nil)

	// 在JSON中发布一个新的version变量在expvar handler中表示我们app的版本
	expvar.NewString("version").Set(version)
	// 发布goroutine的数量
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	// 发布数据库连接池的状态
	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))
	// 发布当前Unix时间戳
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

	// 声明一个app实例，保存依赖
	app := &application{
		config: cfg,
		logger: logger,
		//Use the NewModels function to initialize a Models struct, passing the connection pool as a parameter
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	// Call app.serve() to start the server
	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}
}

// openDB 返回一个sql.DB连接池，与box中不太一样
func openDB(cfg config) (*sql.DB, error) {
	// sql.Open create an empty connection pool
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	// 设置最大连接数为cfg中属性（来自于flag命令行传输)
	db.SetMaxOpenConns(cfg.db.maxOpenConns)

	// 设置最大空闲链接数
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	// 使用time.ParseDuration函数将空闲超时从string转换为time.Duration类型
	// 然后再据此设置最大空闲连接生命周期
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxIdleTime(duration)

	// 创建上下文具有5秒的生命周期, 如果PingContext5s内无法成功，会返回错误
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用PingContext来创建一个链接检查错误
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	// 添加打印用于检查dsn的值
	fmt.Println("Database DSN:", cfg.db.dsn)

	return db, nil
}
