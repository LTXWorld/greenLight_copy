package main

import (
	"context"
	"github.com/LTXWorld/greenLight_copy/internal/data"
	"net/http"
)

// 自定义上下文key类型
type contextKey string

const userContextKey = contextKey("user")

// 返回请求的新副本，将 user 数据存储到请求的上下文中
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	// 根据父上下文r.Context创建了一个新的上下文，包含了键值对信息，键是userContextKey,值是user
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

// 通过请求上下文中的信息获取对应键的值（即User信息）
func (app *application) contextGetUser(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in request context")
	}

	return user
}
