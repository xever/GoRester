// Package resource 内嵌应用的图标资源（Windows 的 .ico 与 macOS 的 .icns），
// 并把它解码成运行时窗口图标所需的原始 RGBA 像素。
package resource

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// 用 go:embed 把图标文件打进二进制，打包后无需再带外部文件。

//go:embed icon.ico
var iconICO []byte

//go:embed icon.icns
var iconICNS []byte

// IconICO 返回 Windows 图标（.ico）的原始字节。
func IconICO() []byte { return iconICO }

// IconICNS 返回 macOS 图标（.icns）的原始字节。
func IconICNS() []byte { return iconICNS }

// WindowIconRGBA 解码 .ico 里最大尺寸的一帧，返回 (size, 直通 RGBA 像素)。
// 像素按从上到下的行序排列，每像素 4 字节（R、G、B、A），正是 app.WindowIcon
// 需要的格式。解码失败时返回 (0, nil)，调用方据此跳过图标设置。
func WindowIconRGBA() (size int, rgba []byte) {
	img, err := decodeICO(iconICO)
	if err != nil {
		return 0, nil
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rgba = make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// 用 NRGBA（直通 alpha，非预乘），符合 WindowIcon 的要求。
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			i := (y*w + x) * 4
			rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = c.R, c.G, c.B, c.A
		}
	}
	return w, rgba
}

// decodeICO 解析 .ico 目录，找到最大尺寸的一帧并解码为图片。
func decodeICO(data []byte) (image.Image, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("ico 文件过短")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count <= 0 || 6+count*16 > len(data) {
		return nil, fmt.Errorf("ico 目录无效")
	}

	bestArea, bestOffset, bestLen := 0, 0, 0
	for i := 0; i < count; i++ {
		e := data[6+i*16:]
		w, h := int(e[0]), int(e[1])
		if w == 0 { // 0 表示 256
			w = 256
		}
		if h == 0 {
			h = 256
		}
		length := int(binary.LittleEndian.Uint32(e[8:12]))
		offset := int(binary.LittleEndian.Uint32(e[12:16]))
		if w*h > bestArea {
			bestArea = w * h
			bestOffset, bestLen = offset, length
		}
	}

	if bestOffset+bestLen > len(data) {
		return nil, fmt.Errorf("ico 帧越界")
	}
	frame := data[bestOffset : bestOffset+bestLen]

	// 现代 .ico 通常是 PNG 压缩帧。
	if len(frame) >= 8 && bytes.Equal(frame[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return png.Decode(bytes.NewReader(frame))
	}
	return nil, fmt.Errorf("ico 帧不是 PNG 格式")
}
