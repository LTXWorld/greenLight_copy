package main

import (
	"errors"
	"expvar"
	"fmt"
	"github.com/LTXWorld/greenLight_copy/internal/data"
	"github.com/LTXWorld/greenLight_copy/internal/validator"
	"github.com/felixge/httpsnoop"
	"github.com/tomasen/realip"
	"golang.org/x/time/rate"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	// 定义一个client结构体包括limiter和最后出现时间
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	// Declare a mutex and a map to hold the clients' IP addresses and rate limiters&time
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Launch a background goroutine which removes old entries from the clients map every minute
	go func() {
		for {
			time.Sleep(time.Minute)
			// 后台Goroutine删除时会不会影响正在运行的后面的其他逻辑？
			mu.Lock()

			// Loop through all clients. If they haven't been seen within the last three minutes
			// delete the corresponding entry
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip) // 从clients map中删除指定ip的entry
				}
			}
			mu.Unlock()
		}
	}()

	//// Initialize a new rate limiter allows an average of 2 requests per second
	//// with a maximum of 4 requests in a single 'burst'
	//limiter := rate.NewLimiter(2, 4)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only carry out the check if rate limiting is enabled
		if app.config.limiter.enabled {
			// host,port,error,从请求地址中提取IP地址，由于设置了反向代理，使用realip.FromRequest
			// 从请求头中获取客户端的真实IP地址
			ip := realip.FromRequest(r)

			mu.Lock() // 下面这段代码互斥进行，不能多个请求同时访问map

			// 检查ip是否已经存在于这个map中(ip-client),对map的一种断言判断
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					// 不再硬编码，而是使用main config内的
					limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst)}
			}

			clients[ip].lastSeen = time.Now()
			// 每当调用Allow都会消耗一个令牌，如果没有剩余令牌就会返回false，Allow底层有锁保持互斥
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				app.rateLimitExceededResponse(w, r)
				return
			}
			mu.Unlock()
		}

		next.ServeHTTP(w, r)
	})
}

// 通过用户传来的JSON请求中的Authorization头字段验证用户信息，并将用户信息加入到请求上下文中
func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 向 HTTP 响应中添加一个 Vary: Authorization 响应头，
		// 它的主要作用是向任何缓存服务器或代理服务器说明，响应的内容可能会因为请求中 Authorization 请求头的不同而变化
		// 只有当请求的 Authorization 头的值相同，缓存才可以重复使用相同的响应。否则，缓存服务器应该认为它们是不同的请求
		w.Header().Add("Vary", "Authorization")

		// 从请求的验证头中获取对应值
		authorizationHeader := r.Header.Get("Authorization")

		// 如果没有Authorization字段，将匿名用户加入到请求上下文中并不执行下面任何代码
		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		// "Bearer <token>"格式
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidCredentialsResponse(w, r)
			return
		}

		// Extract the actual authentication token from the header parts
		token := headerParts[1]

		v := validator.New()

		// 验证token是否有效
		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// 根据有效的token从数据库中进行检索用户信息
		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidCredentialsResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}
		// 将用户信息加入到新的请求上下文中
		r = app.contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

// 判断用户是否匿名
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// retrieve the user information from the request context
		user := app.contextGetUser(r)

		// 如果是匿名用户，返回401错误信息
		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// 判断用户是否非匿名且已经激活
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	//
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		// 检查用户是否已经激活
		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

	// Wrap fn with the requireAuthenticatedUser() middleware before returning it
	// 这样的实现做到了先验证是否匿名，再验证是否激活
	return app.requireAuthenticatedUser(fn)
}

// 检查所给的权限是否在当前用户的权限列表中
func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		// 检查所给的权限是否在当前用户的权限列表中
		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}

	return app.requireActivatedUser(fn)
}

// 使浏览器允许跨域请求的接收
// app有一个来自于命令行设置的信任列表，其他源根据自己的源来判断是否匹配这个信任列表，并填充响应体
func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add the "Vary: Origin" header.
		w.Header().Add("Vary", "Origin")

		// 针对预检请求添加响应头
		w.Header().Add("Vary", "Access-Control-Request-Method")

		// Get the value of the request's Origin header
		origin := r.Header.Get("Origin")

		// Only run this if there's an Origin request header present and at least one trusted
		// origin is configured
		if origin != "" && len(app.config.cors.trustedOrigins) != 0 {
			// 循环去寻找origin中是否在其中之一
			for i := range app.config.cors.trustedOrigins {
				if origin == app.config.cors.trustedOrigins[i] {
					w.Header().Set("Access-Control-Allow-Origin", origin)

					// 检查请求中是否有OPTIONS方法并且包含Access-Control-Request-Method字段POST,DELETE
					// 如果有，就证明这个跨域请求是预检请求
					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						// 设置对于预检请求必要的响应头字段
						w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
						// 	提前响应预检请求并返回 200 OK 状态码
						w.WriteHeader(http.StatusOK)
						return
					}
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) metrics(next http.Handler) http.Handler {
	// 当中间件链第一次构建时初始化新的expvar变量
	totalRequestsReceived := expvar.NewInt("total_requests_received")
	totalResponseSent := expvar.NewInt("total_responses_sent")
	totalProcessingTimeMicroseconds := expvar.NewInt("total_processing_time_μs")
	// 声明一个新的map来保存每个响应状态码的数量
	totalResponseSentByStatus := expvar.NewMap("total_responses_sent_by_status")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		totalRequestsReceived.Add(1)

		// 调用httpsnoop.CatureMetrics，并传入next下一个处理器，最终返回Metrics结构体
		metrics := httpsnoop.CaptureMetrics(next, w, r)

		// 在中间件回溯中，增加响应
		totalResponseSent.Add(1)

		// 获取请求流转时长
		totalProcessingTimeMicroseconds.Add(metrics.Duration.Microseconds())

		// 最终map中存的是"200":n次,使用strconv将int转为string
		totalResponseSentByStatus.Add(strconv.Itoa(metrics.Code), 1)
	})
}
