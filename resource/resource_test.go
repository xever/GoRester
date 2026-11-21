package resource

import "testing"

// TestWindowIconRGBA 验证能从 .ico 解码出运行时窗口图标。
func TestWindowIconRGBA(t *testing.T) {
	size, rgba := WindowIconRGBA()
	if size <= 0 {
		t.Fatal("应能从 icon.ico 解码出图标")
	}
	if len(rgba) != size*size*4 {
		t.Fatalf("RGBA 长度 = %d，期望 %d（%d×%d×4）", len(rgba), size*size*4, size, size)
	}
}

// TestEmbedded 验证两个图标文件都已内嵌。
func TestEmbedded(t *testing.T) {
	if len(IconICO()) == 0 {
		t.Fatal("icon.ico 未被内嵌")
	}
	if len(IconICNS()) == 0 {
		t.Fatal("icon.icns 未被内嵌")
	}
}
