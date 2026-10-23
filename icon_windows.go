//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	iconU32        = syscall.NewLazyDLL("user32.dll")
	iconK32        = syscall.NewLazyDLL("kernel32.dll")
	pFindWindowW   = iconU32.NewProc("FindWindowW")
	pSendMessageW  = iconU32.NewProc("SendMessageW")
	pLoadImageW    = iconU32.NewProc("LoadImageW")
	pSetClassLong  = iconU32.NewProc("SetClassLongPtrW")
	pGetModuleHand = iconK32.NewProc("GetModuleHandleW")
)

const (
	wmSetIcon        = 0x0080
	iconSmall        = 0
	iconBig          = 1
	imageIcon        = 1
	gclpHIconIndex   = 14 // GCLP_HICON = -14
	gclpHIconSmIndex = 34 // GCLP_HICONSM = -34
)

// Windows 任务栏、标题栏、Alt-Tab 用的是「窗口图标」，而不是 exe 文件图标；
// rsrc 只把图标嵌进了文件资源，Wails 创建的窗口并不会自动用它。
// 这里在窗口出现后，从 exe 内嵌的图标资源加载 HICON，同时设置窗口消息图标与窗口类图标。
func applyWindowIcon(title string) {
	go func() {
		t, _ := syscall.UTF16PtrFromString(title)
		var hwnd uintptr
		for i := 0; i < 100; i++ {
			hwnd, _, _ = pFindWindowW.Call(0, uintptr(unsafe.Pointer(t)))
			if hwnd != 0 {
				break
			}
			time.Sleep(150 * time.Millisecond)
		}
		if hwnd == 0 {
			return
		}
		hInst, _, _ := pGetModuleHand.Call(0)
		load := func(id, w, h uintptr) uintptr {
			hIcon, _, _ := pLoadImageW.Call(hInst, id, imageIcon, w, h, 0)
			return hIcon
		}
		// rsrc 默认把图标写为资源 ID 1，多试几个常见 ID 兜底
		var big, small uintptr
		for _, id := range []uintptr{1, 2, 100, 101, 32512} {
			if big == 0 {
				big = load(id, 32, 32)
			}
			if small == 0 {
				small = load(id, 16, 16)
			}
			if big != 0 && small != 0 {
				break
			}
		}
		// GCLP 索引是负数，先转二进制补码再传
		neg := func(n uintptr) uintptr { return ^(n - 1) }
		if big != 0 {
			pSendMessageW.Call(hwnd, wmSetIcon, iconBig, big)
			pSetClassLong.Call(hwnd, neg(gclpHIconIndex), big)
		}
		if small != 0 {
			pSendMessageW.Call(hwnd, wmSetIcon, iconSmall, small)
			pSetClassLong.Call(hwnd, neg(gclpHIconSmIndex), small)
		}
	}()
}
