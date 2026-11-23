# GoRester

一个使用 **Go + Viva** 编写的类 Postman / Apifox 的 HTTP 接口管理与测试工具。
原生桌面应用（macOS / Windows / Linux），无 WebView、无浏览器引擎。

## 为什么做这个
1. 之前常用的某跨平台客户端内存占用极大，还有内存泄漏
2. 他们跨平台都是electron平台的，尝试原生UI
3. 尝试AI agent编程，学习go 语言
4. 为什么没有用rust+tauri ？觉得go有意思 简单好学
5. 开发过程还是发现基于webview的组件丰富好做 :( 😝 


## 功能

- 主页以方格卡片展示项目，左上角「新建」创建项目。
- 每个项目一个标签页，可关闭；左右分栏：左侧接口目录树，右侧主工作区。
- 接口目录树：搜索、按方法过滤、多级文件夹、右键新建接口/文件夹、方法字母徽标、接口下可展开用例。
- 主工作区：工作区标签页（项目概览 + 接口 + 用例）、环境下拉、URL 构造栏（方法 / 路径 / 发送 / 保存 / 保存为用例）。
- 请求参数配置：Params / Body / Cookie / Header / Auth；Body 支持 none / form-data / x-www-form-urlencoded / json / xml / raw，json/xml 支持格式化。
- 响应结果：Body / Cookie / Header，Pretty/Raw 切换，带行号与语法高亮，支持下载、复制、搜索；状态栏显示耗时与状态码。
- 环境管理：Base URL、环境变量、环境 Header、全局变量、全局 Header、默认耗时。
- 变量使用双花括号包裹便命名，如`{{name}}`
- OpenAPI 3.0 导入 / 导出。

## 数据存储

所有数据保存在 `~/Documents/gorester/` 目录下，一个项目一个文件夹，JSON 格式。

## 目录结构

```
gorester/
├── cmd/
│   └── gorester/          # 程序入口（main）
├── internal/              # 内部实现，按职责分包
│   ├── app/               # 应用层：窗口、主页、项目工作区、界面组件
│   ├── model/             # 数据模型（Project / Interface / Case …）
│   ├── storage/           # JSON 持久化到 ~/Documents/gorester/
│   ├── httpclient/        # HTTP 客户端与请求执行
│   ├── openapi/           # OpenAPI 3.0 导入导出
│   └── highlight/         # JSON/XML 格式化与语法高亮
├── resource/              # 图标资源（内嵌进二进制）
│   ├── icon.ico           #   Windows 图标
│   ├── icon.icns          #   macOS 图标
│   └── resource.go        #   内嵌 + 解码运行时窗口图标
├── scripts/               # 构建 / 打包脚本（build / run / test / package-win）
├── dist/                  # 成品（编译产物，不入库）
├── docs/                  # 文档（本 README、需求说明）
├── viva.toml              # viva 打包清单（应用名、图标、版本等）
├── go.mod                 # Go 模块定义
├── go.sum                 # 依赖校验
└── .gitignore
```

## 运行

```bash
./scripts/run.sh          # 直接以源码运行
```

或：

```bash
./scripts/build.sh        # 构建到 dist/gorester
./dist/gorester           # 运行成品
```

需要 Go 1.26+ 与各平台对应工具链（macOS 需要 Xcode 命令行工具）。

## 打包

```bash
./scripts/package-win.sh  # 打包 Windows x64：exe + 图标，输出 zip
./scripts/package-mac.sh  # 打包 macOS：.app + 图标（自动用 .icns）
```

Windows 目标走 purego（无需 mingw-w64 交叉编译器）。图标和版本信息由 `viva.toml`
的 `[app] icon` 写入；运行时窗口图标由内嵌的 `resource/icon.ico` 解码提供。
两个脚本会自动按平台选用 `resource/icon.ico`（Windows）或 `resource/icon.icns`（macOS）。

## 测试与检查

```bash
./scripts/test.sh         # gofmt / go vet / go test
```

测试使用 Viva 的无窗口驱动（`app.Driver`）渲染真实绘制器与焦点规则，不打开窗口。
