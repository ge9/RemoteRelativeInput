package host

import (
	"bufio"
	"flag"
	"fmt"
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
	kc    uint32
	modif uint32
}

func StartServer() {
	X, _ := xgb.NewConn() //use DISPLAY and XAUTHORITY env. This is also used in Wayland (for xkb).

	var (
		clientLinux        bool
		shiftOnlyFlag      string
		autoReleaseFlag    string
		keymapOverrideFlag string
	)

	flag.BoolVar(&clientLinux, "linux", false,
		"the client is Linux")
	flag.StringVar(&shiftOnlyFlag, "shiftonly", "",
		"comma-separated uint32 keycodes (e.g. 0x007c,111)")
	flag.StringVar(&autoReleaseFlag, "autorelease", "",
		"comma-separated uint32 keycodes (e.g. 0xff2a,222)")
	flag.StringVar(&keymapOverrideFlag, "keymap-override", "",
		"comma-separated key mappings uint8:uint32 (e.g. 0xDE:0x005e,111:222)")

	flag.Parse()
	shiftOnlyKeys := parseUint32List(shiftOnlyFlag)
	autoReleaseKeys := parseUint32List(autoReleaseFlag)
	keymap_override := parseKeymapOverride(keymapOverrideFlag)

	fmt.Printf("shiftOnlyKeys = %#v\n", shiftOnlyKeys)
	fmt.Printf("autoReleaseKeys = %#v\n", autoReleaseKeys)
	fmt.Printf("keymap_override = %#v\n", keymap_override)
	mbm := MouseButtonMap{
		1, 3, 2, 9, 8,
	}
	result := getKeyMapLinux(X, shiftOnlyKeys, keymap_override)
	if !clientLinux { //Windows
		result = getKeyMapWin(X, shiftOnlyKeys, autoReleaseKeys, keymap_override)
		mbm = MouseButtonMap{
			1, 2, 4, 8, 16,
		}
	}
	id := func(x int) int {
		return x
	}
	fi := NewFakeInput(X, id)
	scanner := bufio.NewScanner(os.Stdin)
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
				fi.KeyDown(ki)
			case uint32(remote_send.KeyUp):
				fi.KeyUp(ki)
			}
		}
	}
}
func getModifMap(X *xgb.Conn) map[uint32]uint32 {
	modif_map := make(map[uint32]uint32)
	reply2, _ := xproto.GetModifierMapping(
		X,
	).Reply()
	kpm := int(reply2.KeycodesPerModifier)
	for modIdx := 0; modIdx < 8; modIdx++ { //there are 8 modifiers. reference: "Key masks" in /usr/include/X11/X.h
		base := modIdx * kpm
		for i := 0; i < kpm; i++ {
			kc := reply2.Keycodes[base+i]
			if kc != 0 {
				modif_map[uint32(kc)] = 1 << modIdx
			}
		}
	}
	return modif_map
}
func getKeyMapWin(X *xgb.Conn, shiftOnlyKeys []uint32, autoReleaseKeys []uint32, keymap_override map[uint8]uint32) map[uint8]keyinfo {
	setup := xproto.Setup(X)
	min := uint32(setup.MinKeycode)
	max := uint32(setup.MaxKeycode)
	//works for "core keyboard device" ? (xkbcomp without "-i")
	reply, _ := xproto.GetKeyboardMapping(
		X,
		xproto.Keycode(min),
		byte(max-min+1),
	).Reply()

	keysymsPerKeycode := int(reply.KeysymsPerKeycode)
	modif_map := getModifMap(X)
	// shiftOnlyKeys := []uint32{0x007c}   //fix for OEM_102 (bar/underscore) in jp106, ignore AltGr keys
	// autoReleaseKeys := []uint32{0xff2a} //fix for Zenkaku_Hankaku in jp106
	// keymap_override := map[uint8]uint32{0xDE: 0x005e}

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
					if slices.Contains(autoReleaseKeys, keysym) {
						result[winKC] = keyinfo{kc, MODIF_AUTO_RELEASE}
					} else {
						modbit := modif_map[kc] //0 if not a modifier
						result[winKC] = keyinfo{kc, modbit}
					}
					goto found
				}
			}
		}
	found:
	}
	return result
}

// basically identity function
func getKeyMapLinux(X *xgb.Conn, autoReleaseKeys []uint32, keymap_override map[uint8]uint32) map[uint8]keyinfo {
	modif_map := getModifMap(X)
	result := make(map[uint8]keyinfo)
	for kc := uint8(1); kc < 0xFF; kc++ {
		kc0 := uint32(kc)
		kc1 := keymap_override[kc]
		if kc1 != 0 {
			kc0 = kc1
		}
		//this is a bit hacky but we use the modifier variable also for "auto-release" flag
		if slices.Contains(autoReleaseKeys, kc0) {
			result[kc] = keyinfo{kc0, MODIF_AUTO_RELEASE}
		} else {
			modbit := modif_map[kc0] //0 if not a modifier
			result[kc] = keyinfo{kc0, modbit}
		}
	}
	return result
}

func parseUint32List(s string) []uint32 {
	var out []uint32
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseUint(part, 0, 32)
		if err != nil {
			panic(fmt.Errorf("invalid uint32 value %q: %w", part, err))
		}
		out = append(out, uint32(v))
	}
	return out
}
func parseKeymapOverride(s string) map[uint8]uint32 {
	out := make(map[uint8]uint32)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			panic(fmt.Errorf("invalid keymap override %q (expected a:b)", part))
		}

		k, err := strconv.ParseUint(kv[0], 0, 8)
		if err != nil {
			panic(fmt.Errorf("invalid key (uint8) %q: %w", kv[0], err))
		}

		v, err := strconv.ParseUint(kv[1], 0, 32)
		if err != nil {
			panic(fmt.Errorf("invalid value (uint32) %q: %w", kv[1], err))
		}

		out[uint8(k)] = uint32(v)
	}
	return out
}
