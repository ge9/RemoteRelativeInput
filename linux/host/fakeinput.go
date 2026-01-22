// fake input. currently supports X11 and wlroots. TODO: support libei (GNOME Mutter)
package host

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/MatthiasKunnen/go-wayland/wayland/client"
	"github.com/TKMAX777/RemoteRelativeInput/gowayland"
	"github.com/bnema/libei-go-bindings"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
	"golang.org/x/sys/unix"
)

type FakeInput interface {
	MouseMoveRel(dx, dy int16)
	MouseMoveAbs(dx, dy int16)
	MouseButtonMapped(btn uint8, btnmap MouseButtonMap, isUp int) //1 = keyup(released)
	KeyDown(ki keyinfo)
	KeyUp(ki keyinfo)
	Wheel(delta int)
}
type MouseButtonMap struct {
	Left    uint8
	Right   uint8
	Middle  uint8
	Forward uint8
	Back    uint8
}

func NewFakeInput(X *xgb.Conn, f func(int) int) FakeInput {
	wi, err := NewWaylandInput()
	if err != nil {
		println("seems not wayland, use X11")
		xtest.Init(X)
		println("X11 FakeInput setup ok")
		return XFakeInput{X: X}
	}
	err = wi.SetupWaylandInput()
	if err != nil {
		println("wayland but not wlroots, use libei")
		oo, _ := NewLibeiInput()
		println("libei FakeInput setup ok")
		return oo
	}
	println("wlroots FakeInput setup ok")
	return wi
}

//X11

type XFakeInput struct {
	X *xgb.Conn
}

func (xfi XFakeInput) MouseMoveRel(dx, dy int16) {
	xtest.FakeInput(xfi.X, xproto.MotionNotify, 1, 0, 0, dx, dy, 0) //1 = True = Relative
}
func (xfi XFakeInput) MouseMoveAbs(dx, dy int16) {
	xtest.FakeInput(xfi.X, xproto.MotionNotify, 0, 0, 0, dx, dy, 0) //0 = False = Absolute. TODO: test this code
}
func (xfi XFakeInput) MouseButtonMapped(btn uint8, btnmap MouseButtonMap, isUp int) {
	var button byte
	switch btn {
	case uint8(btnmap.Left):
		button = 1
	case uint8(btnmap.Right):
		button = 3
	case uint8(btnmap.Middle):
		button = 2
	case uint8(btnmap.Forward):
		button = 9
	case uint8(btnmap.Back):
		button = 8

	default:
		fmt.Printf("mouse button %d is not supported", btn)
	}
	xtest.FakeInput(xfi.X, xproto.ButtonPress+byte(isUp), button, 0, 0, 0, 0, 0)
}
func (xfi XFakeInput) KeyDown(ki keyinfo) {
	xtest.FakeInput(xfi.X, xproto.KeyPress, byte(ki.kc), 0, 0, 0, 0, 0)
}
func (xfi XFakeInput) KeyUp(ki keyinfo) {
	xtest.FakeInput(xfi.X, xproto.KeyRelease, byte(ki.kc), 0, 0, 0, 0, 0)
}
func (xfi XFakeInput) Wheel(delta int) {
	var p byte = 5
	if delta < 0 {
		p = 4
	}
	xtest.FakeInput(xfi.X, xproto.ButtonPress, p, 0, 0, 0, 0, 0)
	xtest.FakeInput(xfi.X, xproto.ButtonRelease, p, 0, 0, 0, 0, 0)
}

//Wayland (wlroots)

type WaylandInput struct {
	display          *client.Display
	registry         *client.Registry
	seat             *client.Seat
	pointerManager   *gowayland.ZwlrVirtualPointerManagerV1
	pointer          *gowayland.ZwlrVirtualPointerV1
	keyboardManager  *gowayland.ZwpVirtualKeyboardManagerV1
	keyboard         *gowayland.ZwpVirtualKeyboardV1
	currentDepressed uint32 //hotkeys
}

func NewWaylandInput() (*WaylandInput, error) {
	display, err := client.Connect("")
	wi := &WaylandInput{display: display}
	return wi, err
}
func (wi *WaylandInput) SetupWaylandInput() error {
	wi.registry, _ = wi.display.GetRegistry()
	wi.registry.SetGlobalHandler(wi.onGlobal)
	wi.display.Roundtrip()
	if wi.pointerManager == nil || wi.keyboardManager == nil { //TODO: we may need better error handling
		return fmt.Errorf("maybe not wlroots")
	}
	wi.pointer, _ = wi.pointerManager.CreateVirtualPointer(wi.seat)
	wi.keyboard, _ = wi.keyboardManager.CreateVirtualKeyboard(wi.seat)
	wi.display.Roundtrip()
	wi.SetupKeymapFromSystem()
	return nil
}

func (wi *WaylandInput) onGlobal(e client.RegistryGlobalEvent) {
	wi.display.Roundtrip()
	switch e.Interface {
	case "wl_seat":
		wi.seat = client.NewSeat(wi.display.Context())
		wi.registry.Bind(e.Name, e.Interface, e.Version, wi.seat)
	case "zwlr_virtual_pointer_manager_v1":
		wi.pointerManager = gowayland.NewZwlrVirtualPointerManagerV1(wi.display.Context())
		wi.registry.Bind(e.Name, e.Interface, e.Version, wi.pointerManager)
	case "zwp_virtual_keyboard_manager_v1":
		wi.keyboardManager = gowayland.NewZwpVirtualKeyboardManagerV1(wi.display.Context())
		wi.registry.Bind(e.Name, e.Interface, e.Version, wi.keyboardManager)
	}
}

func (wi *WaylandInput) SetupKeymapFromSystem() error {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}
	xauth := os.Getenv("XAUTHORITY")
	if len(xauth) == 0 {
		home := os.Getenv("HOME")
		xauth = home + "/.Xauthority"
	}
	cmd := exec.Command("xkbcomp", display, "-")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}

	keymapStr := out.String()

	return wi.SetupKeymap(keymapStr)
}

func (wi *WaylandInput) SetupKeymap(keymapStr string) error {
	file := createMemFile(keymapStr)
	defer file.Close()

	info, _ := file.Stat()
	size := uint32(info.Size())

	// format: 1 (XKB_KEYMAP_FORMAT_TEXT_V1)
	wi.keyboard.Keymap(1, int(file.Fd()), size)
	wi.display.Roundtrip()
	return nil
}

func createMemFile(content string) *os.File {
	fd, _ := unix.MemfdCreate("keymap", 0)
	file := os.NewFile(uintptr(fd), "keymap")
	file.WriteString(content)
	return file
}

func (wi *WaylandInput) MouseMoveRel(dx, dy int16) {
	wi.pointer.Motion(uint32(0), float64(dx), float64(dy))
	wi.pointer.Frame()

}
func (wi *WaylandInput) MouseMoveAbs(dx, dy int16) {
	println("absolute mouse move is currently not supported in Wayland")
}

// Using 0 instead of time.Now() basically works, but any click event sent while some context menu is opening will be ignored (confirmed in labwc).
func (wi *WaylandInput) MouseButtonMapped(btn uint8, btnmap MouseButtonMap, isUp int) {
	var button2 uint32
	switch btn {
	case uint8(btnmap.Left):
		button2 = 0x110
	case uint8(btnmap.Right):
		button2 = 0x111
	case uint8(btnmap.Middle):
		button2 = 0x112
	case uint8(btnmap.Forward):
		button2 = 0x113
	case uint8(btnmap.Back):
		button2 = 0x114

	default:
		fmt.Printf("mouse button %d is not supported", btn)
	}
	wi.pointer.Button(uint32(time.Now().UnixMilli()), button2, uint32(^isUp&0x1))
	wi.pointer.Frame()
}

func (wi *WaylandInput) KeyDown(ki keyinfo) {
	wi.keyboard.Key(0, ki.kc-8, 1)
	if ki.modif == MODIF_AUTO_RELEASE {
		wi.keyboard.Key(0, ki.kc-8, 1)
		wi.keyboard.Key(0, ki.kc-8, 0)
	} else if ki.modif != 0 {
		wi.currentDepressed |= ki.modif
		wi.keyboard.Modifiers(wi.currentDepressed, 0, 0, 0)
	}
}

func (wi *WaylandInput) KeyUp(ki keyinfo) {
	wi.keyboard.Key(0, ki.kc-8, 0)
	if ki.modif != 0 {
		wi.currentDepressed &^= ki.modif
		wi.keyboard.Modifiers(wi.currentDepressed, 0, 0, 0)
	}
}

func (wi *WaylandInput) Wheel(delta int) {
	wi.pointer.AxisSource(0)
	wi.pointer.Axis(0, 0, float64(delta/12)) //12 seems correct
	wi.pointer.Frame()

	wi.pointer.AxisSource(0)
	wi.pointer.AxisStop(0, 0)
	wi.pointer.Frame()
}

// Wayland (GNOME Mutter)

type LibEIInput struct {
	client *libei.Context
	//we need a map here maybe
	devices map[libei.DeviceCapability]*libei.Device
	session *EISSession
	mu      sync.Mutex
	ready   chan struct{}
	once    sync.Once
}

func NewLibeiInput() (*LibEIInput, error) {
	session, err := GetEISSession()
	if err != nil {
		return nil, err
	}

	client, err := libei.NewSender(nil)
	if err != nil {
		session.Close()
		return nil, err
	}

	if err := client.SetupFD(session.FD); err != nil {
		session.Close()
		return nil, err
	}

	impl := &LibEIInput{
		client:  client,
		session: session,
		ready:   make(chan struct{}),
	}

	go impl.runEventLoop()

	fmt.Println("Waiting for devices ready")
	<-impl.ready
	return impl, nil
}

func (f *LibEIInput) runEventLoop() {
	f.devices = make(map[libei.DeviceCapability]*libei.Device)

	for {
		f.mu.Lock()
		f.client.Dispatch()
		ev, _ := f.client.GetEvent()
		f.mu.Unlock()

		if ev == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		switch ev.Type() {
		case libei.EventSeatAdded:
			f.mu.Lock()
			ev.Seat().BindCapabilities(
				libei.DeviceCapPointer,
				libei.DeviceCapPointerAbsolute,
				libei.DeviceCapKeyboard,
				libei.DeviceCapButton,
				libei.DeviceCapScroll,
			)
			f.mu.Unlock()

		case libei.EventDeviceAdded:
			f.mu.Lock()
			dev := ev.Device()
			dev.StartEmulating(0)

			caps := []libei.DeviceCapability{
				libei.DeviceCapPointer,
				libei.DeviceCapKeyboard,
				libei.DeviceCapButton,
				libei.DeviceCapScroll,
				libei.DeviceCapPointerAbsolute,
			}
			for _, c := range caps {
				if dev.HasCapability(c) {
					f.devices[c] = dev
					fmt.Printf("Device registered ok: %s (Capability: %v)\n", dev.Name(), c)
				}
			}
			f.mu.Unlock()

			// treat as ready if at least one device is ready (is this OK...?)
			f.once.Do(func() { close(f.ready) })
		}
		ev.Release()
	}
}

func (f *LibEIInput) getDevice(cap libei.DeviceCapability) *libei.Device {
	if dev, ok := f.devices[cap]; ok {
		return dev
	}
	return nil
}
func (f *LibEIInput) MouseMoveRel(dx, dy int16) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dev := f.getDevice(libei.DeviceCapPointer)
	if dev == nil {
		return
	}

	dev.PointerMotion(float64(dx), float64(dy))
	dev.Frame(uint64(f.client.Now()))
}

func (f *LibEIInput) MouseMoveAbs(dx, dy int16) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dev := f.getDevice(libei.DeviceCapPointerAbsolute)

	dev.PointerMotionAbsolute(float64(dx), float64(dy))
	dev.Frame(uint64(f.client.Now()))
}

func (f *LibEIInput) MouseButtonMapped(btn uint8, btnmap MouseButtonMap, isUp int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dev := f.getDevice(libei.DeviceCapButton)
	var button uint32
	switch btn {
	case uint8(btnmap.Left):
		button = libei.ButtonLeft
	case uint8(btnmap.Right):
		button = libei.ButtonRight
	case uint8(btnmap.Middle):
		button = libei.ButtonMiddle
	case uint8(btnmap.Forward):
		button = libei.ButtonForward
	case uint8(btnmap.Back):
		button = libei.ButtonBack

	default:
		fmt.Printf("mouse button %d is not supported", btn)
	}
	dev.Button(button, isUp == 0)
	dev.Frame(uint64(f.client.Now()))
}

func (f *LibEIInput) KeyDown(ki keyinfo) {
	f.sendKey(ki, true)
	if ki.modif == MODIF_AUTO_RELEASE {
		f.sendKey(ki, false)
	}
}

func (f *LibEIInput) KeyUp(ki keyinfo) {
	f.sendKey(ki, false)
}

func (f *LibEIInput) sendKey(ki keyinfo, pressed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dev := f.getDevice(libei.DeviceCapKeyboard)
	if dev == nil {
		return
	}

	dev.KeyboardKey(uint32(ki.kc)-8, pressed)
	dev.Frame(uint64(f.client.Now()))
}

func (f *LibEIInput) Wheel(delta int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dev := f.getDevice(libei.DeviceCapScroll)

	dev.ScrollDelta(0, float64(delta/12))
	dev.Frame(uint64(f.client.Now()))
}

func (f *LibEIInput) Close() {
	f.session.Close()
}
