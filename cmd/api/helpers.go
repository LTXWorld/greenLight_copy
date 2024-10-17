package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LTXWorld/greenLight_copy/internal/validator"
	"github.com/julienschmidt/httprouter"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// 从当前请求上下文中获取id参数的值int
func (app *application) readIDParam(r *http.Request) (int64, error) {
	// 路由器解析请求时，任何的插值URL参数都将存储在上下文中
	// 通过此方法获取包含这些参数名称和值的切片
	params := httprouter.ParamsFromContext(r.Context())

	// 使用ByName从键值对中获取到id对应的值，需要将string转换为int
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

// 定义一个封装类型，为了将json中的data们封装为一个对象。
type envelop map[string]interface{}

// 用来将数据写成JSON格式返回给用户，包括了状态码，要传输的被封装过的数据，http头部的map包括任何想要在这个响应中添加的http头部
func (app *application) writeJSON(w http.ResponseWriter, status int, data envelop, headers http.Header) error {
	// Encode the data to JSON，使用MarshalIndent增加空格，使格式更好看
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	// 在写响应前我们不会遇到错误，现在可以添加任何想要添加的http头部
	// 即使对一个空的map进行迭代也不会报错
	for key, value := range headers {
		w.Header()[key] = value
	}

	// 设置"Content-Type:application/json"响应头，如果不设置默认就是text/plain
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// 将JSON作为响应体,JSON仅仅就是一个text
	w.Write(js)

	return nil
}

// 读取JSON格式的请求体并返回其中可能发生的所有关于JSON的错误情况的信息
func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// Use http.MaxBytesReader() 去限制请求体的大小1MB
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	// 初始化json.Decoder，调用DisallowUnknownFields方法在反序列化之前，防止请求体中的数据存在无法映射的属性
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	// 反序列化请求体到目标位置
	err := dec.Decode(dst)
	if err != nil {
		// 对错误进行分类
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		// 使用errors.As函数检查错误类型
		// JSON格式不正确时，少括号多引号等`{"name": "John Doe", "age": 30,}`
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		// errors.Is用于判断错误是否匹配
		// JSON在解析过程中意外结束——数据不完整中途截断`{"name": "John Doe", "age": 30`
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		// JSON数据的类型不正确，字段类型与Go结构体定义类型不匹配，可以捕捉具体的不匹配类型
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		// JSON数据体为空
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		// 如果请求体中包含结构体中没有的属性，decode将会返回json:unknown field <name>，对这个错误进行捕获
		// 并从错误中提取出字段名称，插入到自定义的错误消息中
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		// 如果请求体大小超过了1MB
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		// 反序列化时保存目标不是非空指针,这是不应发生且我们没有准备好妥善处理的错误，故使用Panic。
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		// 其他情况返回错误信息即可
		default:
			return err
		}
	}

	// 上面只是第一次序列化，因为每次调用decode只会读取当前第一个JSON值，如果后面还有JSON并且是垃圾内容，程序不会报错
	// 再次调用decode(),看后面是否还有JSON信息,目标位置设置为匿名的空结构体
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

// 从请求值中返回一个字符串值，如果没有匹配到key返回设置的默认值
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	// Extract the value for a given key from the query string
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	return s
}

// 读取一个字符串值，然后在逗号字符处将其拆分为一个切片
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	// Extract the value from the query string
	csv := qs.Get(key)

	if csv == "" {
		return defaultValue
	}

	//
	return strings.Split(csv, ",")
}

// 从query字符串中读取一个字符串值，将其转换为整数返回，如果转换不成，那么记录Validator错误
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	// Try to convert the value to an int
	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	//
	return i
}

// 用来包装关于goroutine的panic recover逻辑,并使用WaitGroup进行处理后台goroutine的关闭
func (app *application) background(fn func()) {
	// Increment the WaitGroup counter
	app.wg.Add(1)

	// Launch a background goroutine
	go func() {
		defer app.wg.Done()
		// Recover any panic
		defer func() {
			if err := recover(); err != nil {
				app.logger.PrintError(fmt.Errorf("%s", err), nil)
			}
		}()

		// Execute the arbitrary function that we passed as the p
		fn()
	}()
}
