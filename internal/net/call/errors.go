package call

import (
	"fmt"

	"github.com/kanengo/akasar/runtime/codegen"
)

type transportError int

const (
	CommunicationError transportError = iota
	Unreachable
)

func (e transportError) Error() string {
	switch e {
	case CommunicationError:
		return "communication error"
	case Unreachable:
		return "unreachable"
	default:
		return fmt.Sprintf("unknown error %d", e)
	}
}

func decodeError(msg []byte) (err error, ok bool) {
	defer func() {
		if x := codegen.CatchPanics(recover()); x != nil {
			err = x
		}
	}()

	dec := codegen.NewDeserializer(msg)
	err = dec.Error()
	ok = true

	return
}

func encodeError(err error) []byte {
	e := codegen.NewSerializer()
	e.Error(err)

	return e.Data()
}
