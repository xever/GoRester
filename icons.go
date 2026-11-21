package main

// 本文件手工定义应用用到的矢量图标。
//
// Viva 不内置图标集，图标是编译进程序的 gfx.Icon 值（Lucide 风格的 24x24
// 描边图标）。这里用 gfx.PathCommand 逐条画出每个图标，避免引入额外依赖。

import "github.com/vivaui/viva/gfx"

// lucide 用一个统一的风格包住一组路径命令：24x24 视口、2 描边、圆角端点。
func lucide(commands ...gfx.PathCommand) gfx.Icon {
	return gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: commands}
}

// 画一个圆（用于 search、globe 等图标的外圈）。
func iconCircle(cx, cy, r float32) []gfx.PathCommand {
	const k = 0.5523
	return []gfx.PathCommand{
		gfx.MoveTo(cx+r, cy),
		gfx.CubicTo(cx+r, cy+r*k, cx+r*k, cy+r, cx, cy+r),
		gfx.CubicTo(cx-r*k, cy+r, cx-r, cy+r*k, cx-r, cy),
		gfx.CubicTo(cx-r, cy-r*k, cx-r*k, cy-r, cx, cy-r),
		gfx.CubicTo(cx+r*k, cy-r, cx+r, cy-r*k, cx+r, cy),
		gfx.ClosePath(),
	}
}

// 画一个圆角矩形（用于 file、copy 等图标的轮廓）。
func iconRect(x, y, w, h, r float32) []gfx.PathCommand {
	return []gfx.PathCommand{
		gfx.MoveTo(x+r, y),
		gfx.LineTo(x+w-r, y),
		gfx.CubicTo(x+w, y, x+w, y+r, x+w, y+r),
		gfx.LineTo(x+w, y+h),
		gfx.LineTo(x, y+h),
		gfx.LineTo(x, y+r),
		gfx.CubicTo(x, y+r, x, y, x+r, y),
		gfx.ClosePath(),
	}
}

var (
	// iconPlus 加号：新建。
	iconPlus = lucide(
		gfx.MoveTo(12, 5), gfx.LineTo(12, 19),
		gfx.MoveTo(5, 12), gfx.LineTo(19, 12),
	)

	// iconSearch 放大镜：搜索。
	iconSearch = lucide(append(iconCircle(11, 11, 8),
		gfx.MoveTo(21, 21), gfx.LineTo(16.65, 16.65))...)

	// iconFilter 漏斗：过滤。
	iconFilter = lucide(
		gfx.MoveTo(22, 3), gfx.LineTo(2, 3),
		gfx.LineTo(10, 12.46), gfx.LineTo(10, 19), gfx.LineTo(14, 21), gfx.LineTo(14, 12.46),
		gfx.LineTo(22, 3),
	)

	// iconClose 叉号：关闭标签页 / 弹窗。
	iconClose = lucide(
		gfx.MoveTo(18, 6), gfx.LineTo(6, 18),
		gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	)

	// iconFolder 文件夹。
	iconFolder = lucide(
		gfx.MoveTo(4, 20), gfx.LineTo(20, 20), gfx.CubicTo(21.1, 20, 22, 19.1, 22, 18), gfx.LineTo(22, 8),
		gfx.CubicTo(22, 6.9, 21.1, 6, 20, 6), gfx.LineTo(12.1, 6), gfx.LineTo(10.5, 3.9),
		gfx.CubicTo(10.1, 3.4, 9.6, 3.1, 9, 3.1), gfx.LineTo(4, 3.1),
		gfx.CubicTo(2.9, 3.1, 2, 4, 2, 5.1), gfx.LineTo(2, 18), gfx.CubicTo(2, 19.1, 2.9, 20, 4, 20), gfx.ClosePath(),
	)

	// iconSend 发送（纸飞机）。
	iconSend = lucide(
		gfx.MoveTo(22, 2), gfx.LineTo(11, 13),
		gfx.MoveTo(22, 2), gfx.LineTo(15, 22), gfx.LineTo(11, 13), gfx.LineTo(2, 9), gfx.ClosePath(),
	)

	// iconSave 保存（软盘）。
	iconSave = lucide(
		gfx.MoveTo(19, 21), gfx.LineTo(5, 21), gfx.CubicTo(4.46, 21, 4, 20.54, 4, 20), gfx.LineTo(4, 4),
		gfx.CubicTo(4, 3.45, 4.45, 3, 5, 3), gfx.LineTo(16, 3), gfx.LineTo(20, 7), gfx.LineTo(20, 20),
		gfx.CubicTo(20, 20.54, 19.54, 21, 19, 21), gfx.ClosePath(),
		gfx.MoveTo(17, 21), gfx.LineTo(17, 12), gfx.LineTo(7, 12), gfx.LineTo(7, 21),
		gfx.MoveTo(7, 3), gfx.LineTo(7, 7), gfx.LineTo(15, 7),
	)

	// iconChevronDown 下箭头：下拉菜单提示。
	iconChevronDown = lucide(
		gfx.MoveTo(6, 9), gfx.LineTo(12, 15), gfx.LineTo(18, 9),
	)

	// iconMore 三个点：更多操作。
	iconMore = lucide(
		gfx.MoveTo(5, 12), gfx.LineTo(5.01, 12),
		gfx.MoveTo(12, 12), gfx.LineTo(12.01, 12),
		gfx.MoveTo(19, 12), gfx.LineTo(19.01, 12),
	)

	// iconDownload 下载。
	iconDownload = lucide(
		gfx.MoveTo(21, 15), gfx.LineTo(21, 19), gfx.CubicTo(21, 19.53, 20.53, 20, 20, 20), gfx.LineTo(4, 20),
		gfx.CubicTo(3.46, 20, 3, 19.53, 3, 19), gfx.LineTo(3, 15),
		gfx.MoveTo(7, 10), gfx.LineTo(12, 15), gfx.LineTo(17, 10),
		gfx.MoveTo(12, 15), gfx.LineTo(12, 3),
	)

	// iconUpload 上传（导出）。
	iconUpload = lucide(
		gfx.MoveTo(21, 15), gfx.LineTo(21, 19), gfx.CubicTo(21, 19.53, 20.53, 20, 20, 20), gfx.LineTo(4, 20),
		gfx.CubicTo(3.46, 20, 3, 19.53, 3, 19), gfx.LineTo(3, 15),
		gfx.MoveTo(17, 8), gfx.LineTo(12, 3), gfx.LineTo(7, 8),
		gfx.MoveTo(12, 3), gfx.LineTo(12, 15),
	)

	// iconCopy 复制。
	iconCopy = lucide(append(iconRect(9, 9, 13, 13, 2),
		gfx.MoveTo(5, 15), gfx.LineTo(4, 15), gfx.CubicTo(3.46, 15, 3, 14.54, 3, 14), gfx.LineTo(3, 4),
		gfx.CubicTo(3, 3.45, 3.45, 3, 4, 3), gfx.LineTo(14, 3), gfx.CubicTo(14.54, 3, 15, 3.46, 15, 4), gfx.LineTo(15, 5))...)

	// iconTrash 删除（垃圾桶）。
	iconTrash = lucide(
		gfx.MoveTo(3, 6), gfx.LineTo(21, 6),
		gfx.MoveTo(19, 6), gfx.LineTo(19, 20), gfx.CubicTo(19, 20.53, 18.53, 21, 18, 21), gfx.LineTo(6, 21),
		gfx.CubicTo(5.47, 21, 5, 20.53, 5, 20), gfx.LineTo(5, 6),
		gfx.MoveTo(8, 6), gfx.LineTo(8, 4), gfx.CubicTo(8, 3.47, 8.47, 3, 9, 3), gfx.LineTo(15, 3),
		gfx.CubicTo(15.53, 3, 16, 3.47, 16, 4), gfx.LineTo(16, 6),
		gfx.MoveTo(10, 11), gfx.LineTo(10, 17),
		gfx.MoveTo(14, 11), gfx.LineTo(14, 17),
	)

	// iconSettings 环境设置（用「滑杆」图标代替齿轮，线条更简单）。
	iconSettings = lucide(
		gfx.MoveTo(21, 4), gfx.LineTo(14, 4), gfx.MoveTo(10, 4), gfx.LineTo(3, 4),
		gfx.MoveTo(21, 12), gfx.LineTo(12, 12), gfx.MoveTo(8, 12), gfx.LineTo(3, 12),
		gfx.MoveTo(21, 20), gfx.LineTo(16, 20), gfx.MoveTo(12, 20), gfx.LineTo(3, 20),
		gfx.MoveTo(14, 2), gfx.LineTo(14, 6),
		gfx.MoveTo(8, 10), gfx.LineTo(8, 14),
		gfx.MoveTo(16, 18), gfx.LineTo(16, 22),
	)

	// iconFile 接口文件。
	iconFile = lucide(
		gfx.MoveTo(15, 2), gfx.LineTo(6, 2), gfx.CubicTo(5.47, 2, 5, 2.47, 5, 3), gfx.LineTo(5, 21),
		gfx.CubicTo(5, 21.53, 5.47, 22, 6, 22), gfx.LineTo(18, 22), gfx.CubicTo(18.53, 22, 19, 21.53, 19, 21), gfx.LineTo(19, 6), gfx.ClosePath(),
		gfx.MoveTo(14, 2), gfx.LineTo(14, 7), gfx.LineTo(19, 7),
	)

	// iconBookmark 书签：用例。
	iconBookmark = lucide(
		gfx.MoveTo(19, 21), gfx.LineTo(12, 17), gfx.LineTo(5, 21), gfx.LineTo(5, 5), gfx.CubicTo(5, 4.47, 5.47, 4, 6, 4),
		gfx.LineTo(18, 4), gfx.CubicTo(18.53, 4, 19, 4.47, 19, 5), gfx.ClosePath(),
	)

	// iconGlobe 地球：环境。
	iconGlobe = lucide(append(iconCircle(12, 12, 10),
		gfx.MoveTo(2, 12), gfx.LineTo(22, 12),
		gfx.MoveTo(12, 2), gfx.CubicTo(14.5, 4.5, 15.5, 8, 15.5, 12), gfx.CubicTo(15.5, 16, 14.5, 19.5, 12, 22),
		gfx.CubicTo(9.5, 19.5, 8.5, 16, 8.5, 12), gfx.CubicTo(8.5, 8, 9.5, 4.5, 12, 2))...)

	// iconHome 主页。
	iconHome = lucide(
		gfx.MoveTo(3, 10.5), gfx.LineTo(12, 3), gfx.LineTo(21, 10.5),
		gfx.MoveTo(5, 9.5), gfx.LineTo(5, 21), gfx.LineTo(19, 21), gfx.LineTo(19, 9.5),
	)
)
