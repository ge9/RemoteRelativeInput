// *This doesn't work in XWayland. Use C implementation instead.*
package client

import (
	"fmt"
	"log"
	"os"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

var (
	ExitKeycode = uint8(74)   // F8
	WindowSize  = uint16(200) // window size
	BluePixel   = uint32(0x0000FF)
	StayOnTop   = true
	HideCursor  = true
)

func StartClient() {
	X, err := xgb.NewConn()
	if err != nil {
		log.Fatalf("failed to connect to X: %v", err)
	}
	defer X.Close()

	setup := xproto.Setup(X)
	screen := setup.DefaultScreen(X)
	root := screen.Root

	wid, _ := xproto.NewWindowId(X)
	mask := uint32(xproto.CwBackPixel | xproto.CwEventMask)
	values := []uint32{
		BluePixel,
		xproto.EventMaskExposure | xproto.EventMaskButtonPress,
	}

	xproto.CreateWindow(X, screen.RootDepth, wid, root,
		100, 100, WindowSize, WindowSize, 0,
		xproto.WindowClassInputOutput, screen.RootVisual, mask, values)

	title := "Input Capture Box"
	xproto.ChangeProperty(X, xproto.PropModeReplace, wid, xproto.AtomWmName, xproto.AtomString, 8, uint32(len(title)), []byte(title))
	wmProtocols := getAtom(X, "WM_PROTOCOLS")
	wmDeleteWindow := getAtom(X, "WM_DELETE_WINDOW")

	xproto.ChangeProperty(X, xproto.PropModeReplace, wid, wmProtocols, xproto.AtomAtom, 32, 1,
		[]byte{
			byte(wmDeleteWindow & 0xff),
			byte((wmDeleteWindow >> 8) & 0xff),
			byte((wmDeleteWindow >> 16) & 0xff),
			byte((wmDeleteWindow >> 24) & 0xff),
		})
	if StayOnTop { //not working
		setAlwaysOnTop(X, root, wid)
	}

	xproto.MapWindow(X, wid)
	blankCursor := xproto.Cursor(0)
	if HideCursor {
		blankCursor = createBlankCursor(X, root)
	}
	isCapturing := false

	centerX := int16(screen.WidthInPixels / 2)
	centerY := int16(screen.HeightInPixels / 2)

	grabInput := func() bool {
		kbReply, _ := xproto.GrabKeyboard(X, false, root, xproto.TimeCurrentTime,
			xproto.GrabModeAsync, xproto.GrabModeAsync).Reply()
		if kbReply.Status != xproto.GrabStatusSuccess {
			return false
		}
		pReply, _ := xproto.GrabPointer(X, false, root,
			xproto.EventMaskPointerMotion|xproto.EventMaskButtonPress|xproto.EventMaskButtonRelease,
			xproto.GrabModeAsync, xproto.GrabModeAsync,
			root, blankCursor, xproto.TimeCurrentTime).Reply()

		if pReply.Status != xproto.GrabStatusSuccess {
			xproto.UngrabKeyboard(X, xproto.TimeCurrentTime)
			return false
		}

		// set initial pointer location
		xproto.WarpPointer(X, xproto.WindowNone, root, 0, 0, 0, 0, centerX, centerY)
		return true
	}

	// start capture at start
	if grabInput() {
		isCapturing = true
		fmt.Fprintf(os.Stderr, "Capture started. (F8 to release)\n")
	}

	for {
		ev, err := X.WaitForEvent()
		if err != nil {
			continue
		}

		switch e := ev.(type) {
		case xproto.ClientMessageEvent:
			if e.Type == wmProtocols {
				data := e.Data.Data32
				if len(data) > 0 && xproto.Atom(data[0]) == wmDeleteWindow {
					fmt.Println("CLOSE")
					return
				}
			}
		case xproto.KeyPressEvent:
			if isCapturing {
				if uint8(e.Detail) == ExitKeycode {
					xproto.UngrabPointer(X, xproto.TimeCurrentTime)
					xproto.UngrabKeyboard(X, xproto.TimeCurrentTime)
					isCapturing = false
					fmt.Fprintf(os.Stderr, "Capture released. (Click the blue window to resume)\n")
				} else {
					fmt.Printf("3 %d 0 0\n", uint8(e.Detail))
				}
			}

		case xproto.KeyReleaseEvent:
			if isCapturing {
				fmt.Printf("3 %d 1 0\n", uint8(e.Detail))
			}

		case xproto.ButtonPressEvent:
			if !isCapturing {
				// resume capture on click
				if e.Event == wid {
					if grabInput() {
						isCapturing = true
						fmt.Fprintf(os.Stderr, "Capture resumed.\n")
					}
				}
			} else {
				if e.Detail == 4 { // Scroll Up
					fmt.Printf("2 120 1 0\n") //120 is the default value in Windows
				} else if e.Detail == 5 { // Scroll Down
					fmt.Printf("2 120 0 0\n")
				} else {
					fmt.Printf("0 %d 0 0\n", uint8(e.Detail))
				}
			}

		case xproto.ButtonReleaseEvent:
			if isCapturing && e.Detail != 4 && e.Detail != 5 {
				fmt.Printf("0 %d 1 0\n", uint8(e.Detail))
			}

		case xproto.MotionNotifyEvent:
			if isCapturing {
				dx := e.RootX - centerX
				dy := e.RootY - centerY

				// reset pointer location
				// this won't create dead loop because dx, dy is 0 for WarpPointer
				if dx != 0 || dy != 0 {
					fmt.Printf("1 0 %d %d\n", dx, dy)
					xproto.WarpPointer(X, xproto.WindowNone, root, 0, 0, 0, 0, centerX, centerY)
				}
			}
		}
	}
}

func setAlwaysOnTop(X *xgb.Conn, root xproto.Window, wid xproto.Window) {
	stateAtom := getAtom(X, "_NET_WM_STATE")
	aboveAtom := getAtom(X, "_NET_WM_STATE_ABOVE")

	ev := xproto.ClientMessageEvent{
		Format: 32,
		Window: wid,
		Type:   stateAtom,
		Data: xproto.ClientMessageDataUnionData32New([]uint32{
			1, // _NET_WM_STATE_ADD
			uint32(aboveAtom),
			0, 0, 0,
		}),
	}

	xproto.SendEvent(X, false, root, xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify, string(ev.Bytes()))
}

func createBlankCursor(X *xgb.Conn, root xproto.Window) xproto.Cursor {
	pix, _ := xproto.NewPixmapId(X)
	// empty 1x1 pix map
	xproto.CreatePixmap(X, 1, pix, xproto.Drawable(root), 1, 1)

	cid, _ := xproto.NewCursorId(X)
	xproto.CreateCursor(X, cid, pix, pix, 0, 0, 0, 0, 0, 0, 0, 0)
	return cid
}

func getAtom(X *xgb.Conn, name string) xproto.Atom {
	reply, _ := xproto.InternAtom(X, false, uint16(len(name)), name).Reply()
	if reply == nil {
		return 0
	}
	return reply.Atom
}
