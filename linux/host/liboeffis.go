package host

/*
#cgo pkg-config: liboeffis-1.0
#include <libei-1.0/liboeffis.h>
*/
import "C"
import (
	"fmt"
	"time"
)

type EISSession struct {
	FD      int
	context *C.struct_oeffis
}

func GetEISSession() (*EISSession, error) {
	ctx := C.oeffis_new(nil)
	if ctx == nil {
		return nil, fmt.Errorf("failed to create oeffis context")
	}

	C.oeffis_create_session(ctx, C.OEFFIS_DEVICE_POINTER|C.OEFFIS_DEVICE_KEYBOARD)

	fmt.Println("requesting mouse/keyboard permission with a dialog...")

	for {
		C.oeffis_dispatch(ctx)
		ev := C.oeffis_get_event(ctx)

		switch ev {
		case C.OEFFIS_EVENT_CONNECTED_TO_EIS:
			fd := int(C.oeffis_get_eis_fd(ctx))
			return &EISSession{FD: fd, context: ctx}, nil

		case C.OEFFIS_EVENT_DISCONNECTED, C.OEFFIS_EVENT_CLOSED:
			C.oeffis_unref(ctx)
			return nil, fmt.Errorf("portal connection failed or closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *EISSession) Close() {
	if s.context != nil {
		C.oeffis_unref(s.context)
		s.context = nil
	}
}
