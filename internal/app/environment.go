package app

// 本文件实现「环境」相关的全部逻辑：当前环境读取、下拉框标签与下标缓存、
// 全局/环境变量表合并，以及环境管理抽屉的编辑与持久化。

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
	"gorester/internal/model"
)

// ---------------------------------------------------------------------------
// 环境管理抽屉
// ---------------------------------------------------------------------------
const (
	envNameWidth = 90.0
)

// openEnvDrawer 打开环境抽屉并同步编辑快照。
func (s *projectSession) openEnvDrawer() {
	s.syncEnvSnapshots()
	s.envDrawerOpen = true
}

// 环境设置窗体样式
func (s *projectSession) envDrawer(c ui.C) {
	ui.Drawer(c, &s.envDrawerOpen, i18n.T("dialog.envManage"), ui.DrawerSpec{Size: 440},
		func() {
			ui.Scroll(c).Fill().Pad(16).Children(func() {
				ui.Column(c).Gap(12).Children(func() {
					// —— 环境选择 / 增删 ——
					ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
						envs := s.project.Environments
						sel := ui.Select(c, &s.envIndex, s.envIndices(), s.envLabelFn).Grow(1).MaxWidth(220).Label(i18n.T("dialog.selectEnv"))
						if sel.Changed() {
							s.syncEnvSnapshots()
						}
						ui.Spacer(c)
						if ui.Button(c, i18n.T("dialog.addEnv")).Icon(iconPlus).Ghost().Small().Clicked() {
							s.project.Environments = append(s.project.Environments, &model.Environment{Name: i18n.T("dialog.newEnv"), Vars: map[string]string{}, Headers: map[string]string{}, TimeoutMs: model.DefaultTimeoutMs})
							s.envIndex = len(s.project.Environments) - 1
							s.syncEnvSnapshots()
						}
						if len(envs) > 1 && ui.Button(c, i18n.T("dialog.deleteEnv")).Icon(iconTrash).Ghost().Small().Clicked() {
							s.project.Environments = append(envs[:s.envIndex], envs[s.envIndex+1:]...)
							if s.envIndex >= len(s.project.Environments) {
								s.envIndex = len(s.project.Environments) - 1
							}
							s.syncEnvSnapshots()
						}
					})

					// —— 当前环境基础配置 ——
					if env := s.currentEnv(); env != nil {
						ui.Text(c, i18n.T("dialog.basic")).Weight(gfx.WeightSemibold)
						// 表单文字放在输入框左侧：TextField 的 Label 参数只是无障碍标签，并不绘制出来。
						ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
							ui.Label(c, i18n.T("dialog.envName")).Width(envNameWidth).MarginLeft(20)
							ui.TextField(c, &env.Name, i18n.T("dialog.envName"), "").Grow(1)
						})
						ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
							ui.Label(c, i18n.T("dialog.baseURL")).Width(envNameWidth).MarginLeft(20)
							ui.TextField(c, &env.BaseURL, i18n.T("dialog.baseURL"), i18n.T("dialog.baseURLPlaceholder")).Grow(1)
						})
						ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
							ui.Label(c, i18n.T("dialog.timeout")).Width(envNameWidth).MarginLeft(20)
							ui.TextField(c, &s.envTimeoutEdit, i18n.T("dialog.timeout"), "30").Tooltip(i18n.T("dialog.timeout.tip")).Grow(1)
						})

						ui.Divider(c)
						ui.Text(c, i18n.T("dialog.envVars")).Weight(gfx.WeightSemibold)
						s.kvEditor(c, &s.envVarsEdit, kvColumns{valueLabel: i18n.T("common.value")})
						ui.Divider(c)
						ui.Text(c, i18n.T("dialog.envHeaders")).Weight(gfx.WeightSemibold)
						s.kvEditor(c, &s.envHeadersEdit, kvColumns{valueLabel: i18n.T("common.value")})
					}

					ui.Divider(c)
					ui.Text(c, i18n.T("dialog.globalVars")).Weight(gfx.WeightSemibold)
					s.kvEditor(c, &s.globalVarsEdit, kvColumns{valueLabel: i18n.T("common.value")})
					ui.Divider(c)
					ui.Text(c, i18n.T("dialog.globalHeaders")).Weight(gfx.WeightSemibold)
					s.kvEditor(c, &s.globalHeadersEdit, kvColumns{valueLabel: i18n.T("common.value")})
				})
			})
		},
		func() {
			if ui.Button(c, i18n.T("common.done")).Clicked() {
				s.commitEnvSnapshots()
				s.envDrawerOpen = false
			}
		},
	)
}

// syncEnvSnapshots 把当前环境与全局的变量 / Header 复制到可编辑的切片。
func (s *projectSession) syncEnvSnapshots() {
	if env := s.currentEnv(); env != nil {
		s.envVarsEdit = mapToKV(env.Vars)
		s.envHeadersEdit = mapToKV(env.Headers)
		s.envTimeoutEdit = strconv.Itoa(env.TimeoutMs / 1000) // 以秒展示
	}
	s.globalVarsEdit = mapToKV(s.project.GlobalVars)
	s.globalHeadersEdit = mapToKV(s.project.GlobalHeaders)
}

// commitEnvSnapshots 把编辑快照写回环境与全局变量。
func (s *projectSession) commitEnvSnapshots() {
	if env := s.currentEnv(); env != nil {
		env.Vars = kvToMap(s.envVarsEdit)
		env.Headers = kvToMap(s.envHeadersEdit)
		if sec, err := strconv.Atoi(strings.TrimSpace(s.envTimeoutEdit)); err == nil && sec > 0 {
			env.TimeoutMs = sec * 1000
		}
	}
	s.project.GlobalVars = kvToMap(s.globalVarsEdit)
	s.project.GlobalHeaders = kvToMap(s.globalHeadersEdit)
	// 环境超时变化时同步到复用客户端。
	if env := s.currentEnv(); env != nil && env.TimeoutMs > 0 {
		s.client.SetTimeout(timeDurationMs(env.TimeoutMs))
	}
	s.persist()
}

// currentEnv 返回当前选中的环境（越界时回退到第一个）。
func (s *projectSession) currentEnv() *model.Environment {
	envs := s.project.Environments
	if len(envs) == 0 {
		return nil
	}
	if s.envIndex < 0 || s.envIndex >= len(envs) {
		s.envIndex = 0
	}
	return envs[s.envIndex]
}

// envNameAt 返回第 i 个环境的名称（供环境下拉框标签使用）。
func (s *projectSession) envNameAt(i int) string {
	if i < 0 || i >= len(s.project.Environments) {
		return ""
	}
	return s.project.Environments[i].Name
}

// envIndices 返回环境下拉框的下标切片（环境数量变化时才重新分配）。
func (s *projectSession) envIndices() []int {
	n := len(s.project.Environments)
	if len(s.envIdxCache) != n {
		s.envIdxCache = make([]int, n)
		for i := range s.envIdxCache {
			s.envIdxCache[i] = i
		}
	}
	return s.envIdxCache
}

// requestVars 组装变量表：全局变量 + 环境变量（环境变量优先级更高）。
func (s *projectSession) requestVars() map[string]string {
	vars := map[string]string{}
	for k, v := range s.project.GlobalVars {
		vars[k] = v
	}
	if env := s.currentEnv(); env != nil {
		for k, v := range env.Vars {
			vars[k] = v
		}
	}
	return vars
}

// ---------------------------------------------------------------------------
// map / 切片互转（环境编辑用）
// ---------------------------------------------------------------------------

// mapToKV 把 map[string]string 转成有序的 []model.KeyValue（键名排序，便于编辑）。
func mapToKV(m map[string]string) []model.KeyValue {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]model.KeyValue, 0, len(keys))
	for _, k := range keys {
		out = append(out, model.KeyValue{Key: k, Value: m[k], Enabled: true})
	}
	return out
}

// kvToMap 把 []model.KeyValue 转回 map[string]string（忽略空键）。
func kvToMap(kvs []model.KeyValue) map[string]string {
	m := map[string]string{}
	for _, kv := range kvs {
		if kv.Key != "" {
			m[kv.Key] = kv.Value
		}
	}
	return m
}

// timeDurationMs 把毫秒转成 time.Duration。
func timeDurationMs(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
