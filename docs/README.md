# GoRester

一个使用 **Go + Viva** 编写的类 Postman / Apifox 的 HTTP 接口管理与测试工具。
原生桌面应用（macOS / Windows / Linux），无 WebView、无浏览器引擎。

## 目录结构

```
gorester/
├── src/          # 源码（Go 源文件）
├── scripts/      # 构建脚本（build / run / test）
├── dist/         # 成品（编译产物，不入库）
├── docs/         # 文档（本 README、需求说明）
├── go.mod        # Go 模块定义
├── go.sum        # 依赖校验
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

## 测试与检查

```bash
./scripts/test.sh         # gofmt / go vet / go test
```

测试使用 Viva 的无窗口驱动（`app.Driver`）渲染真实绘制器与焦点规则，不打开窗口。

## 功能

- 主页以方格卡片展示项目，左上角「新建」创建项目。
- 每个项目一个标签页，可关闭；左右分栏：左侧接口目录树，右侧主工作区。
- 接口目录树：搜索、按方法过滤、多级文件夹、右键新建接口/文件夹、方法字母徽标、接口下可展开用例。
- 主工作区：工作区标签页（项目概览 + 接口 + 用例）、环境下拉、URL 构造栏（方法 / 路径 / 发送 / 保存 / 保存为用例）。
- 请求参数配置：Params / Body / Cookie / Header / Auth；Body 支持 none / form-data / x-www-form-urlencoded / json / xml / raw，json/xml 支持格式化。
- 响应结果：Body / Cookie / Header，Pretty/Raw 切换，带行号与语法高亮，支持下载、复制、搜索；状态栏显示耗时与状态码。
- 环境管理：Base URL、环境变量、环境 Header、全局变量、全局 Header、默认耗时。
- OpenAPI 3.0 导入 / 导出。

## 数据存储

所有数据保存在 `~/gorester/` 目录下，一个项目一个文件夹，JSON 格式。

## 源码说明

| 文件（`src/` 下） | 说明 |
| --- | --- |
| `main.go` | 入口 + 顶层状态与标签栏 |
| `app.go` | 会话状态、目录树扁平化、环境工具 |
| `home.go` | 主页 |
| `sidebar.go` | 左侧目录树 |
| `workspace.go` | 主工作区（请求/响应/状态栏） |
| `dialogs.go` | 弹窗与抽屉（新建文件夹、保存、用例、环境） |
| `keyvalue.go` | 键值对表格编辑器 |
| `model.go` | 数据模型 |
| `storage.go` | JSON 持久化 |
| `httpclient.go` | HTTP 客户端与请求执行 |
| `openapi.go` | OpenAPI 3.0 导入导出 |
| `highlight.go` | JSON/XML 格式化与语法高亮 |
| `icons.go` | 矢量图标 |
