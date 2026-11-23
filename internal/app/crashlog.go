package app

// 本文件负责把崩溃信息写到 ~/Documents/gorester/log/app.log。
//
// 两类崩溃都会被记录：
//  1. Viva 在原生边界捕获到的 panic（窗口过程、绘制回调、拖拽等），
//     经 app.SetFaultHandler 通知；
//  2. 未恢复的 panic / 致命错误，经 runtime/debug.SetCrashOutput 写入。

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	vivaapp "github.com/vivaui/viva/app"

	"gorester/internal/storage"
)

// crashLogFile 保持崩溃日志文件的句柄，供 SetFaultHandler 写入。
var crashLogFile *os.File

// setupCrashLog 打开日志文件并安装崩溃处理。失败时静默返回（日志是尽力而为）。
func setupCrashLog() {
	dir, err := storage.LogDir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	crashLogFile = f

	// 未恢复的 panic / 致命错误：由 Go runtime 写到文件（它会复制文件描述符，
	// 所以这里保留自己的句柄用于 SetFaultHandler 也没关系）。
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		crashLogFile = nil
		f.Close()
		return
	}

	// Viva 捕获到的 panic（不会让进程退出的那种）。
	vivaapp.SetFaultHandler(func(fault vivaapp.Fault) {
		writeCrashLog(fmt.Appendf(nil, "[fault] %s: %s\n%s\n", fault.Where, fault.Message, fault.Stack))
	})
}

// writeCrashLog 把一段内容带时间戳追加到日志文件。
func writeCrashLog(data []byte) {
	if crashLogFile == nil {
		return
	}
	fmt.Fprintf(crashLogFile, "\n===== %s =====\n", time.Now().Format("2006-01-02 15:04:05"))
	crashLogFile.Write(data)
	crashLogFile.Sync()
}
