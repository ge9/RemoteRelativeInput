package host

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/TKMAX777/RemoteRelativeInput/keymap"
	"github.com/TKMAX777/RemoteRelativeInput/remote_send"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

func StartServer() {
	//var xdot = linuxapi.NewXdotool(display)
	scanner := bufio.NewScanner(os.Stdin)

	X, _ := xgb.NewConn() //use DISPLAY and XAUTHORITY env
	setup := xproto.Setup(X)

	min := setup.MinKeycode
	max := setup.MaxKeycode
	//works for "core keyboard device" ? (xkbcomp without "-i")
	reply, _ := xproto.GetKeyboardMapping(
		X,
		min,
		byte(max-min+1),
	).Reply()
	keysymsPerKeycode := int(reply.KeysymsPerKeycode)

	// for kc := min; kc < max; kc++ {
	// 	base := int(kc-min) * keysymsPerKeycode
	// 	for i := 0; i < keysymsPerKeycode; i++ {
	// 		println(reply.Keysyms[base+i], kc)
	// 	}
	// }
	result := make(map[uint8]xproto.Keycode)
	//FIXME: hard-coded range from windows_key_map.go
	for winKC := uint8(0x01); winKC <= 0xFE; winKC++ {
		key, e := keymap.GetWindowsKeyDetail(uint32(winKC))
		if e != nil {
			continue
		}
		keysym, ok := keymap.X11Keys[key.EventInput]
		if !ok {
			continue
		}

		for kc := min; kc < max; kc++ {
			base := int(kc-min) * keysymsPerKeycode
			for i := 0; i < keysymsPerKeycode; i++ {
				if keysym == 0xee01 { //customized keys, OEM_102. FIXME: This code is Japan-centric and also assume default xkb mapping.
					result[winKC] = 132
				} else if uint32(reply.Keysyms[base+i]) == keysym {
					result[winKC] = kc
					goto found
				}
			}
		}
	found:
	}

	xtest.Init(X)
	for scanner.Scan() {
		if scanner.Text() == "CLOSE" {
			os.Exit(0)
		}

		var augs = strings.Split(scanner.Text(), " ")
		if len(augs) < 4 {
			continue
		}

		eventType, err := strconv.ParseUint(augs[0], 10, 32)
		if err != nil {
			continue
		}
		eventInput, err := strconv.ParseUint(augs[1], 10, 32)
		if err != nil {
			continue
		}
		eventValue1, err := strconv.ParseInt(augs[2], 10, 32)
		if err != nil {
			continue
		}
		eventValue2, err := strconv.ParseInt(augs[3], 10, 32)
		if err != nil {
			continue
		}

		switch keymap.EV_TYPE(eventType) {
		case keymap.EV_TYPE_MOUSE_MOVE:
			switch uint32(eventInput) {
			case uint32(remote_send.MouseMoveTypeRelative):
				xtest.FakeInput(X, xproto.MotionNotify, 1, 0, 0, int16(eventValue1), int16(eventValue2), 0) //1 = True = Relative
			case uint32(remote_send.MouseMoveTypeAbsolute):
				xtest.FakeInput(X, xproto.MotionNotify, 0, 0, 0, int16(eventValue1), int16(eventValue2), 0) //0 = False = Positive. TODO: test this code
			}
		case keymap.EV_TYPE_MOUSE:
			var button byte
			switch eventInput {
			case 0x01:
				button = 1 // Left
			case 0x04:
				button = 2 // Middle
			case 0x02:
				button = 3 // Right
			default:
				continue
			}

			switch uint32(eventValue1) {
			case uint32(remote_send.KeyDown):
				xtest.FakeInput(X, xproto.ButtonPress, button, 0, 0, 0, 0, 0)
			case uint32(remote_send.KeyUp):
				xtest.FakeInput(X, xproto.ButtonRelease, button, 0, 0, 0, 0, 0)
			}
		case keymap.EV_TYPE_WHEEL:
			switch uint32(eventValue1) {
			case uint32(remote_send.KeyDown):
				// down = button 5
				xtest.FakeInput(X, xproto.ButtonPress, 5, 0, 0, 0, 0, 0)
				xtest.FakeInput(X, xproto.ButtonRelease, 5, 0, 0, 0, 0, 0)
			case uint32(remote_send.KeyUp):
				// up = button 4
				xtest.FakeInput(X, xproto.ButtonPress, 4, 0, 0, 0, 0, 0)
				xtest.FakeInput(X, xproto.ButtonRelease, 4, 0, 0, 0, 0, 0)
			}
		case keymap.EV_TYPE_KEY:
			key, err := keymap.GetWindowsKeyDetail(uint32(eventInput))
			if err != nil || key.EventInput == "" {
				println("unsupported key:", uint32(eventInput))
				continue
			}

			switch uint32(eventValue1) {
			case uint32(remote_send.KeyDown):
				//xdot.KeyDown(key.EventInput)
				xtest.FakeInput(X, xproto.KeyPress, byte(result[uint8(eventInput)]), 0, 0, 0, 0, 0)
			case uint32(remote_send.KeyUp):
				//xdot.KeyUp(key.EventInput)
				xtest.FakeInput(X, xproto.KeyRelease, byte(result[uint8(eventInput)]), 0, 0, 0, 0, 0)
			}
		}
	}
}
