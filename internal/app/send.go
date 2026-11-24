package app

// 本文件实现「发送请求」的完整流程：从当前编辑的接口/用例与环境构造请求选项、
// 把请求投递到后台 Effect、以及读取响应 Content-Type。

import (
	"strings"

	"github.com/vivaui/viva/ui"

	"gorester/internal/httpclient"
	"gorester/internal/i18n"
	"gorester/internal/model"
)

// buildOptions 根据当前编辑的接口/用例 + 环境，构造发送请求所需的选项。
func (s *projectSession) buildOptions() httpclient.RequestOptions {
	opts := httpclient.RequestOptions{Vars: s.requestVars()}

	// 环境 base_url。
	if env := s.currentEnv(); env != nil {
		opts.BaseURL = env.BaseURL
	}

	// 全局 + 环境 Header 作为基础 Header。
	merged := map[string]string{}
	for k, v := range s.project.GlobalHeaders {
		merged[k] = v
	}
	if env := s.currentEnv(); env != nil {
		for k, v := range env.Headers {
			merged[k] = v
		}
	}

	// 当前标签页对应的请求定义。
	if tab := s.currentTab(); tab != nil {
		switch tab.kind {
		case tabInterface:
			if tab.iface != nil {
				fillOptsFromInterface(&opts, tab.iface)
			}
		case tabCase:
			if tab.case_ != nil {
				opts.Method = tab.case_.Method
				opts.Path = tab.case_.Path
				opts.Params = cloneKVs(tab.case_.Params)
				opts.Headers = cloneKVs(tab.case_.Headers)
				opts.Cookies = cloneKVs(tab.case_.Cookies)
				opts.Body = cloneBody(tab.case_.Body)
				opts.Auth = tab.case_.Auth
			}
		}
	}

	// 全局 / 环境 Header 放在接口自己的 Header 之后，接口自己的优先。
	// 用新切片合并，避免 append 破坏接口原来的 Header 底层数组。
	mergedHeaders := append([]model.KeyValue(nil), opts.Headers...)
	for k, v := range merged {
		exists := false
		for _, h := range mergedHeaders {
			if strings.EqualFold(h.Key, k) {
				exists = true
				break
			}
		}
		if !exists {
			mergedHeaders = append(mergedHeaders, model.KeyValue{Key: k, Value: v, Enabled: true})
		}
	}
	opts.Headers = mergedHeaders
	return opts
}

// fillOptsFromInterface 把接口的定义（深拷贝）填入选项，避免后台请求与编辑共享数据。
func fillOptsFromInterface(opts *httpclient.RequestOptions, iface *model.Interface) {
	opts.Method = iface.Method
	opts.Path = iface.Path
	opts.Params = cloneKVs(iface.Params)
	opts.Headers = cloneKVs(iface.Headers)
	opts.Cookies = cloneKVs(iface.Cookies)
	opts.Body = cloneBody(iface.Body)
	opts.Auth = iface.Auth
}

// cloneBody 深拷贝请求体（Raw 字符串不可变，只拷贝两个键值对切片）。
func cloneBody(b model.Body) model.Body {
	b.FormData = cloneKVs(b.FormData)
	b.URLEncoded = cloneKVs(b.URLEncoded)
	return b
}

// doSend 根据当前编辑内容 + 环境发起请求（异步）。
func (s *projectSession) doSend() {
	// 发送前先把代码编辑器里未落盘的编辑写回请求体。
	s.flushBodyEditor()
	opts := s.buildOptions()
	if opts.Method == "" {
		s.app.toaster.Show(i18n.T("toast.cannotSend"), i18n.T("toast.selectMethod"), ui.ToastError)
		return
	}
	// 记录本次实际发送的请求（用于「请求」标签展示）。
	s.lastReq = s.client.Describe(opts)
	s.sendMu.Lock()
	s.sendOpts = opts
	s.sendMu.Unlock()
	s.sending = true
	s.sendKey++
}

// responseContentType 从响应头里提取 Content-Type。
func (s *projectSession) responseContentType() string {
	if s.response == nil {
		return ""
	}
	for _, h := range s.response.Headers {
		if strings.EqualFold(h.Key, "Content-Type") {
			return h.Value
		}
	}
	return ""
}
