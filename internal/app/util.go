package app

// 本文件放与界面/业务无关的通用小工具：键值切片深拷贝、HTTP 状态码描述、
// 字节数格式化。它们被请求、响应、弹窗等多处复用，因此单独拎出来。

import (
	"fmt"

	"gorester/internal/model"
)

// cloneKVs 深拷贝一个键值对切片，避免用例与接口共享底层数组。
func cloneKVs(src []model.KeyValue) []model.KeyValue {
	if src == nil {
		return nil
	}
	out := make([]model.KeyValue, len(src))
	copy(out, src)
	return out
}

// httpStatusText 返回状态码的常见描述。
func httpStatusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	case 504:
		return "Gateway Timeout"
	}
	return ""
}

// humanBytes 把字节数格式化成可读文本。
func humanBytes(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
