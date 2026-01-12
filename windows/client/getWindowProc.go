package client

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/TKMAX777/RemoteRelativeInput/debug"
	"github.com/TKMAX777/RemoteRelativeInput/keymap"
	"github.com/TKMAX777/RemoteRelativeInput/remote_send"
	"github.com/TKMAX777/RemoteRelativeInput/winapi"
	"github.com/lxn/win"
)

var (
	imm32                   = syscall.NewLazyDLL("imm32.dll")
	procImmAssociateContext = imm32.NewProc("ImmAssociateContext")
)

func (h Handler) getWindowProc(rdClientHwnd win.HWND) func(hwnd win.HWND, uMsg uint32, wParam uintptr, lParam uintptr) uintptr {
	// get remote desktop client rect
	var rect win.RECT
	if !winapi.GetWindowRect(rdClientHwnd, &rect) {
		fmt.Fprintf(os.Stderr, "getWindowProc: GetWindowRectError")
	}

	var isRelativeMode = true

	// get remote desktop client center position
	var windowCenterPosition = h.getWindowCenterPos(rect)

	var currentPosition = windowCenterPosition
	var _ = currentPosition
	debug.Debugln("ToggleKey: ", h.options.toggleKey)
	debug.Debugln("ToggleType: ", h.options.toggleType)
	debug.Debugln("ExitKey: ", h.options.exitKey)

	toggleKey, err := keymap.GetWindowsKeyDetailFromEventInput(h.options.toggleKey)
	if err != nil {
		toggleKey, _ = keymap.GetWindowsKeyDetailFromEventInput("F8")
	}

	exitKey, err := keymap.GetWindowsKeyDetailFromEventInput(h.options.exitKey)
	if err != nil {
		exitKey, _ = keymap.GetWindowsKeyDetailFromEventInput("F12")
	}

	return func(hwnd win.HWND, uMsg uint32, wParam uintptr, lParam uintptr) uintptr {
		var send = func(evType keymap.EV_TYPE, key uint32, state remote_send.InputType) {
			if key == exitKey.Value {
				switch state {
				case remote_send.KeyDown:
					win.SetTimer(hwnd, 1, 500, 0)
				case remote_send.KeyUp:
					win.KillTimer(hwnd, 1)
				}
			}

			if key == toggleKey.Value {
				// toggle window mode
				switch state {
				case remote_send.KeyDown:
					isRelativeMode = !isRelativeMode || h.options.toggleType == ToggleTypeOnce
				case remote_send.KeyUp:
					if h.options.toggleType == ToggleTypeOnce {
						isRelativeMode = false
					}
				}

				debug.Debugf("Set: isRelativeMode: %t\n", isRelativeMode)

				if isRelativeMode {
					h.initWindowAndCursor(hwnd, rdClientHwnd)
					debug.Debugln("Called: initWindowAndCursor")
				} else {
					var crectAbs win.RECT
					if !winapi.GetWindowRect(rdClientHwnd, &crectAbs) {
						fmt.Fprintf(os.Stderr, "GetWindowRectError")
					}

					// show window title only
					if !win.SetWindowPos(hwnd, 0,
						crectAbs.Left, crectAbs.Top, crectAbs.Right-crectAbs.Left, h.metrics.TitleHeight+h.metrics.FrameWidthY*2,
						win.SWP_SHOWWINDOW,
					) {
						fmt.Fprintf(os.Stderr, "SetWindowPos: failed to set window pos")
					}

					debug.Debugln("Called: SetWindowPos")
					winapi.ShowCursor(true)
					winapi.ClipCursor(nil)
				}
				debug.Debugln("ModeChangeDone")
			}
			if isRelativeMode {
				h.remote.SendInput(evType, key, state)
			}
		}

		switch uMsg {
		case win.WM_CREATE:
			winapi.SetLayeredWindowAttributes(hwnd, 0x0000FF, byte(1), winapi.LWA_COLORKEY)
			h.initWindowAndCursor(hwnd, rdClientHwnd)
			win.UpdateWindow(hwnd)
			registerRawInput(hwnd)
			procImmAssociateContext.Call(uintptr(hwnd), 0) //avoid invoking IME

			return winapi.NULL
		case win.WM_SETFOCUS:
			return winapi.NULL
		case win.WM_DESTROY:
			h.remote.SendExit()
			os.Exit(0)
			return winapi.NULL
		case win.WM_PAINT:
			var ps = new(win.PAINTSTRUCT)
			var hdc = win.BeginPaint(hwnd, ps)
			var hBrush = winapi.CreateSolidBrush(0x000000FF)

			win.SelectObject(hdc, hBrush)
			winapi.ExtFloodFill(hdc, 1, 1, 0xFFFFFF, winapi.FLOODFILLSURFACE)
			win.DeleteObject(hBrush)
			win.EndPaint(hwnd, ps)

			if isRelativeMode {
				winapi.SetLayeredWindowAttributes(hwnd, 0x0000FF, byte(1), winapi.LWA_COLORKEY)

				// get remote desktop client rect
				var rect win.RECT
				if !winapi.GetWindowRect(rdClientHwnd, &rect) {
					fmt.Fprintf(os.Stderr, "getWindowProc: GetWindowRectError")
				}

				// get remote desktop client center position
				windowCenterPosition = h.getWindowCenterPos(rect)
			}
			return winapi.NULL
		case win.WM_TIMER:
			h.remote.SendExit()
			os.Exit(0)
			return winapi.NULL
		case win.WM_INPUT:

			var size uint32
			win.GetRawInputData(
				win.HRAWINPUT(lParam),
				win.RID_INPUT,
				nil,
				&size,
				uint32(unsafe.Sizeof(win.RAWINPUTHEADER{})),
			)

			buf := make([]byte, size)
			if win.GetRawInputData(
				win.HRAWINPUT(lParam),
				win.RID_INPUT,
				unsafe.Pointer(&buf[0]),
				&size,
				uint32(unsafe.Sizeof(win.RAWINPUTHEADER{})),
			) != size {
				return winapi.NULL
			}

			hdr := (*win.RAWINPUTHEADER)(unsafe.Pointer(&buf[0]))

			switch hdr.DwType {

			case win.RIM_TYPEMOUSE:
				raw := (*win.RAWINPUTMOUSE)(unsafe.Pointer(&buf[0]))
				m := raw.Data

				if isRelativeMode {
					dx := int32(m.LLastX)
					dy := int32(m.LLastY)

					if dx != 0 || dy != 0 {
						h.remote.SendRelativeCursor(dx, dy)
					}

					btn := m.UsButtonData
					if btn&win.RI_MOUSE_LEFT_BUTTON_DOWN != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x01, remote_send.KeyDown)
					}
					if btn&win.RI_MOUSE_LEFT_BUTTON_UP != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x01, remote_send.KeyUp)
					}
					if btn&win.RI_MOUSE_RIGHT_BUTTON_DOWN != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x02, remote_send.KeyDown)
					}
					if btn&win.RI_MOUSE_RIGHT_BUTTON_UP != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x02, remote_send.KeyUp)
					}
					if btn&win.RI_MOUSE_MIDDLE_BUTTON_DOWN != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x04, remote_send.KeyDown)
					}
					if btn&win.RI_MOUSE_MIDDLE_BUTTON_UP != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x04, remote_send.KeyUp)
					}
					if btn&win.RI_MOUSE_BUTTON_4_DOWN != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x08, remote_send.KeyDown)
					}
					if btn&win.RI_MOUSE_BUTTON_4_UP != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x08, remote_send.KeyUp)
					}
					if btn&win.RI_MOUSE_BUTTON_5_DOWN != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x10, remote_send.KeyDown)
					}
					if btn&win.RI_MOUSE_BUTTON_5_UP != 0 {
						send(keymap.EV_TYPE_MOUSE, 0x10, remote_send.KeyUp)
					}

					if btn&win.RI_MOUSE_WHEEL != 0 {
						//we have to use ugly Pad_cgo_* fields here...
						delta := uint16(m.Pad_cgo_0[0]) | uint16(m.Pad_cgo_0[1])<<8
						wheel := int16(delta)
						if wheel > 0 {
							send(keymap.EV_TYPE_WHEEL, uint32(wheel), remote_send.KeyUp)
						} else {
							send(keymap.EV_TYPE_WHEEL, uint32(-wheel), remote_send.KeyDown)
						}
					}
				}

			case win.RIM_TYPEKEYBOARD:
				_ = fmt.Errorf("e%v", 4)

				raw := (*win.RAWINPUTKEYBOARD)(unsafe.Pointer(&buf[0]))
				k := raw.Data

				key := uint32(k.VKey)

				if k.Flags&win.RI_KEY_BREAK != 0 {
					send(keymap.EV_TYPE_KEY, key, remote_send.KeyUp)
				} else {
					send(keymap.EV_TYPE_KEY, key, remote_send.KeyDown)
				}
			}

			return winapi.NULL
		default:
			return win.DefWindowProc(hwnd, uMsg, wParam, lParam)
		}
	}
}

func (h Handler) Close() {
	winapi.ClipCursor(nil)
	winapi.ShowCursor(true)
}

func registerRawInput(hwnd win.HWND) error {
	rids := []win.RAWINPUTDEVICE{
		{
			UsUsagePage: 0x01, // Generic Desktop
			UsUsage:     0x02, // Mouse
			DwFlags:     win.RIDEV_INPUTSINK,
			HwndTarget:  hwnd,
		},
		{
			UsUsagePage: 0x01, // Generic Desktop
			UsUsage:     0x06, // Keyboard
			DwFlags:     win.RIDEV_INPUTSINK,
			HwndTarget:  hwnd,
		},
	}
	if !win.RegisterRawInputDevices(
		&rids[0],
		uint32(len(rids)),
		uint32(unsafe.Sizeof(rids[0])),
	) {
		return fmt.Errorf("RegisterRawInputDevices failed")
	}
	return nil
}
