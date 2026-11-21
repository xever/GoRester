// GoRester 是一个使用 Go + Viva 编写的 HTTP 接口管理与测试工具（类似
// Postman / Apifox）。这里是程序入口，实际逻辑都在 internal/app 里。
package main

import "gorester/internal/app"

func main() {
	app.Run()
}
