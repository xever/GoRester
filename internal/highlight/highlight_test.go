package highlight

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// TestJSON 验证 JSON 高亮能生成正确的行数。
func TestJSON(t *testing.T) {
	theme := ui.LightTheme()
	lines, ok := BuildResponseLines([]byte(`{"a":1,"b":[true,null]}`), "application/json", theme)
	if !ok {
		t.Fatal("合法 JSON 应被高亮")
	}
	if len(lines) < 3 {
		t.Fatalf("格式化后的 JSON 应有多行，实际 %d", len(lines))
	}
}
