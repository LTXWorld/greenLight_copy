package validator

import "regexp"

var (
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

// Validator 定义新的检验类型包括了检验错误map
type Validator struct {
	Errors map[string]string
}

// New 用来创建新的Validator实例包含空的错误map
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

// Valid 如果错误map中没有内容，就证明没错
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError 添加错误信息并避免重复的错误类型
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists { // 检查Errors中是否已经存在该键（错误类型），如果不存在才添加
		v.Errors[key] = message
	}
}

// Check 检查ok的条件是否正确，并根据key返回message信息
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// In returns true if a specific value is in a list of strings
func In(value string, list ...string) bool {
	for i := range list {
		if value == list[i] {
			return true
		}
	}
	return false
}

// Matches 判断一个string类型是否匹配具体的正则表达式
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// Unique 判断一个string切片中的string是否各不相同
func Unique(values []string) bool {
	uniqueValues := make(map[string]bool)

	for _, value := range values {
		uniqueValues[value] = true
	}

	return len(values) == len(uniqueValues)
}
