package host

import (
	"bufio"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/TKMAX777/RemoteRelativeInput/keymap"
	"github.com/TKMAX777/RemoteRelativeInput/remote_send"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

const MODIF_AUTO_RELEASE = 0x100000

type keyinfo struct {
	kc    xproto.Keycode
	modif uint32
}

func StartServer() {
	X, _ := xgb.NewConn() //use DISPLAY and XAUTHORITY env. This is also used in Wayland (for xkb).
	result := getKeyMap(X)
	id := func(x int) int {
		return x
	}
	fi := NewFakeInput(X, id)
	scanner := bufio.NewScanner(os.Stdin)
	mbm := MouseButtonMap{
		1, 2, 4, 8, 16,
	}
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
				fi.MouseMoveRel(int16(eventValue1), int16(eventValue2))
			case uint32(remote_send.MouseMoveTypeAbsolute):
				fi.MouseMoveAbs(int16(eventValue1), int16(eventValue2))
			}
		case keymap.EV_TYPE_MOUSE:
			fi.MouseButtonMapped(uint8(eventInput), mbm, int(eventValue1))
		case keymap.EV_TYPE_WHEEL:
			if eventValue1 == 1 {
				eventInput = -eventInput
			}
			fi.Wheel(int(eventInput))
		case keymap.EV_TYPE_KEY:
			ki := result[uint8(eventInput)]
			if ki.kc == 0 {
				println("unsupported key:", uint32(eventInput))
				continue
			}
			switch uint32(eventValue1) {
			case uint32(remote_send.KeyDown):
				println(eventInput)
				println(ki.kc)
				fi.KeyDown(ki)
			case uint32(remote_send.KeyUp):
				fi.KeyUp(ki)
			}
		}
	}
}

func getKeyMap(X *xgb.Conn) map[uint8]keyinfo {
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

	shiftOnlyKeys := []uint32{0x007c}   //fix for OEM_102 (bar/underscore) in jp106, ignore AltGr keys
	autoReleaseKeys := []uint32{0xff2a} //fix for Zenkaku_Hankaku in jp106
	keymap_override := map[uint8]uint32{0xDE: 0x005e}

	for kc := min; kc < max; kc++ {
		base := int(kc-min) * keysymsPerKeycode
		for i := 0; i < keysymsPerKeycode; i++ { //i=0, 1, 2, 3 is (typically) for none, Shift, AltGr, Shift+AltGr
			println(reply.Keysyms[base+i], kc, i)
		}
	}
	result := make(map[uint8]keyinfo)
	for winKC := uint8(0x01); winKC <= 0xFE; winKC++ {
		key, e := keymap.GetWindowsKeyDetail(uint32(winKC))
		keysym := keymap_override[winKC]
		if keysym == 0 { //use default
			if e != nil {
				continue
			}
			keysym0, ok := keymap.X11Keys[key.Constant]
			if !ok {
				continue
			}
			keysym = keysym0
		}

		for kc := min; kc < max; kc++ {
			base := int(kc-min) * keysymsPerKeycode
			for i := 0; i < keysymsPerKeycode; i++ {
				if slices.Contains(shiftOnlyKeys, keysym) && i >= 2 {
					continue
				}
				if uint32(reply.Keysyms[base+i]) == keysym {
					//hotkeys
					switch key.Constant {
					case "VK_CONTROL":
						result[winKC] = keyinfo{kc, 4}
					case "VK_CAPITAL":
						result[winKC] = keyinfo{kc, 2}
					case "VK_SHIFT":
						result[winKC] = keyinfo{kc, 1}
					case "VK_MENU":
						result[winKC] = keyinfo{kc, 8}
					case "VK_NUMLOCK":
						result[winKC] = keyinfo{kc, 16}
					default:
						result[winKC] = keyinfo{kc, 0}
						//this is a bit hacky but we use modifier variable also as a mark for "auto-release" keys
						if slices.Contains(autoReleaseKeys, keysym) {
							result[winKC] = keyinfo{kc, MODIF_AUTO_RELEASE}
						}
					}
					goto found
				}
			}
		}
	found:
	}
	return result
}
