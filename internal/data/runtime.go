package data

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidRuntimeFormat 是一个UnmarshalJSON方法会发生的错误类型
var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

// Runtime 本质上还是int32类型的，序列化为JSON时转为string类型，反序列化为Go时转回int32类型
type Runtime int32

// MarshalJSON 为Runtime类型实现接口（重写该方法）实现了自定义JSON序列化格式
func (r Runtime) MarshalJSON() ([]byte, error) {
	// 产生一个string显示我们要的电影时长格式
	jsonValue := fmt.Sprintf("%d mins", r)

	// 使用strconv.Quote()函数将string包裹在双引号中，以符合JSON string的格式
	quotedJSONValue := strconv.Quote(jsonValue)

	// 将这样一个string转为字节切片返回
	return []byte(quotedJSONValue), nil
}

// UnmarshalJSON 为Runtime类型实现反序列化接口，自定义使传来的string类型反序列化为Runtime(int32)类型
func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {
	// 传来的电影时长runtime应该是"<runtime> mins"这样的格式，先试着去除双引号
	unquotedJSONValue, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	// 如果没有错误，就提取number部分
	parts := strings.Split(unquotedJSONValue, " ")

	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRuntimeFormat
	}

	// 否则，进行转换为int类型
	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	// Convert the int32 to a Runtime type(本质上也是int32)
	*r = Runtime(i)

	return nil
}
