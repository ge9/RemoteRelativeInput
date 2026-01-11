package keymap

// this is basically combination of windows_key_map.go and /usr/include/X11/keysymdef.h.
// assumes US (and sometimes JP) keyboard.
var X11Keys = map[string]uint32{
	"VK_BACK":       0xff08,
	"VK_TAB":        0xff09,
	"VK_CLEAR":      0xff0b,
	"VK_RETURN":     0xff0d,
	"VK_SHIFT":      0xffe1, /* Left shift */
	"VK_CONTROL":    0xffe3, /* Left control */
	"VK_MENU":       0xffe9, /* Left alt */
	"VK_PAUSE":      0xff0d,
	"VK_CAPITAL":    0xffe5,
	"VK_ESCAPE":     0xff1b,
	"VK_SPACE":      0x0020,
	"VK_OEM_3":      0x0060, // "`~" in US, "@`" in JP (grave works for both)
	"VK_OEM_PLUS":   0x002b, // "=+" in US, ";+" in JP (plus works for both)
	"VK_OEM_1":      0x003a, // ";:" in US,  ":*" in JP (colon works for both)
	"VK_OEM_COMMA":  0x002c,
	"VK_OEM_MINUS":  0x002d,
	"VK_OEM_7":      0x0027, // "'"" in US, "^~" in JP
	"VK_OEM_PERIOD": 0x002e,
	"VK_OEM_2":      0x002f, // "/?" in US and JP
	"VK_OEM_5":      0x007c, // "\|" in US and JP.
	"VK_OEM_102":    0x005f, // "\_" in JP. 0x005f is underscore.
	"VK_OEM_4":      0x005b, // "[{" in US and JP
	"VK_OEM_6":      0x005d, // "]}" in US and JP
	"VK_PRIOR":      0xff55, // PageUp
	"VK_NEXT":       0xff56, // PageDown
	"VK_END":        0xff57,
	"VK_HOME":       0xff58,
	"VK_LEFT":       0xff51,
	"VK_UP":         0xff52,
	"VK_RIGHT":      0xff53,
	"VK_DOWN":       0xff54,
	"VK_INSERT":     0xff63,
	"VK_DELETE":     0xffff,
	"VK_OEM_AUTO":   0xff2a, //Zenkaku_Hankaku
	"0 key":         0x0030,
	"1 key":         0x0031,
	"2 key":         0x0032,
	"3 key":         0x0033,
	"4 key":         0x0034,
	"5 key":         0x0035,
	"6 key":         0x0036,
	"7 key":         0x0037,
	"8 key":         0x0038,
	"9 key":         0x0039,
	"A key":         0x0061,
	"B key":         0x0062,
	"C key":         0x0063,
	"D key":         0x0064,
	"E key":         0x0065,
	"F key":         0x0066,
	"G key":         0x0067,
	"H key":         0x0068,
	"I key":         0x0069,
	"J key":         0x006a,
	"K key":         0x006b,
	"L key":         0x006c,
	"M key":         0x006d,
	"N key":         0x006e,
	"O key":         0x006f,
	"P key":         0x0070,
	"Q key":         0x0071,
	"R key":         0x0072,
	"S key":         0x0073,
	"T key":         0x0074,
	"U key":         0x0075,
	"V key":         0x0076,
	"W key":         0x0077,
	"X key":         0x0078,
	"Y key":         0x0079,
	"Z key":         0x007a,
	"VK_F1":         0xffbe,
	"VK_F2":         0xffbf,
	"VK_F3":         0xffc0,
	"VK_F4":         0xffc1,
	"VK_F5":         0xffc2,
	"VK_F6":         0xffc3,
	"VK_F7":         0xffc4,
	"VK_F8":         0xffc5,
	"VK_F9":         0xffc6,
	"VK_F10":        0xffc7,
	"VK_F11":        0xffc8,
	"VK_F12":        0xffc9,
	"VK_F13":        0xffca,
	"VK_F14":        0xffcb,
	"VK_F15":        0xffcc,
	"VK_F16":        0xffcd,
	"VK_F17":        0xffce,
	"VK_F18":        0xffcf,
	"VK_F19":        0xffd0,
	"VK_F20":        0xffd1,
	"VK_F21":        0xffd2,
	"VK_F22":        0xffd3,
	"VK_F23":        0xffd4,
	"VK_F24":        0xffd5,
}
