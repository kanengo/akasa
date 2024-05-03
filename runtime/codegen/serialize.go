package codegen

import (
	"encoding/binary"
	"fmt"
	"math"

	"google.golang.org/protobuf/proto"

	"github.com/kanengo/akasar/internal/umath"

	"github.com/kanengo/akasar/internal/unsafex"
)

type serializerError struct {
	err error
}

func (e serializerError) Error() string {
	if e.err == nil {
		return "serializer:"
	}

	return "serializer:" + e.err.Error()
}

func makeSerializerError(format string, args ...interface{}) serializerError {
	return serializerError{err: fmt.Errorf(format, args...)}
}

type Serializer struct {
	buf []byte
}

func NewSerializer(size ...int) *Serializer {
	if len(size) > 0 {
		n := umath.FindNearestPow2(size[0])
		enc := &Serializer{buf: make([]byte, 0, n)}
		return enc
	}

	return &Serializer{buf: make([]byte, 0, 128)}
}

func (s *Serializer) grow(bytesNeeded int) {
	l := len(s.buf)
	c := cap(s.buf)

	if l+bytesNeeded <= c {
		return
	}

	newSize := umath.FindNearestPow2(c + 1)
	//fmt.Printf("new cap size:%d\n", newSize)
	buf := make([]byte, 0, newSize)
	buf = append(buf, s.buf...)

	s.buf = buf
}

func (s *Serializer) Uint64(val uint64) {
	s.grow(8)
	s.buf = binary.LittleEndian.AppendUint64(s.buf, val)
}

func (s *Serializer) Uint32(val uint32) {
	s.grow(4)
	s.buf = binary.LittleEndian.AppendUint32(s.buf, val)
}

func (s *Serializer) Uint16(val uint16) {
	s.grow(2)
	s.buf = binary.LittleEndian.AppendUint16(s.buf, val)
}

func (s *Serializer) Uint8(val uint8) {
	s.grow(1)
	s.buf = append(s.buf, val)
}

func (s *Serializer) Uint(val uint) {
	s.Uint64(uint64(val))
}

func (s *Serializer) Int(val int) {
	s.Uint64(uint64(val))
}

func (s *Serializer) Int64(val int64) {
	s.Uint64(uint64(val))
}

func (s *Serializer) Int32(val int32) {
	s.Uint32(uint32(val))
}

func (s *Serializer) Int16(val int16) {
	s.Uint16(uint16(val))
}

func (s *Serializer) Int8(val int8) {
	s.Uint8(uint8(val))
}

func (s *Serializer) Byte(val byte) {
	s.grow(1)
	s.buf = append(s.buf, val)
}

func (s *Serializer) String(val string) {
	n := len(val)
	if n > math.MaxInt32 {
		panic(makeSerializerError("unable to encode string; length doesn't fit in 4 bytes"))
	}
	s.Uint32(uint32(n))
	if n == 0 {
		return
	}
	s.grow(n)
	s.buf = append(s.buf, unsafex.StringToBytes(val)...)
}

func (s *Serializer) MarshalProto(value proto.Message) {
	bs, err := proto.Marshal(value)
	if err != nil {
		panic(makeSerializerError("error encoding to proto %T: %w", value, err))
	}
	s.Bytes(bs)
}

func (s *Serializer) Bytes(val []byte) {
	if val == nil {
		s.Int32(-1)
		return
	}
	n := len(val)
	if n > math.MaxInt32 {
		panic(makeSerializerError("unable to encode bytes; length doesn't fit in 4 bytes"))
	}
	s.Uint32(uint32(n))
	if n == 0 {
		return
	}
	s.grow(n)
	s.buf = append(s.buf, val...)
}

func (s *Serializer) Bool(b bool) {
	if b {
		s.Uint8(1)
	} else {
		s.Uint8(0)
	}
}

func (s *Serializer) Float32(val float32) {
	s.Uint32(math.Float32bits(val))
}

func (s *Serializer) Float64(val float64) {
	s.Uint64(math.Float64bits(val))
}

func (s *Serializer) Complex64(val complex64) {
	s.Float32(real(val))
	s.Float32(imag(val))
}

func (s *Serializer) Complex128(val complex128) {
	s.Float64(real(val))
	s.Float64(imag(val))
}

func (s *Serializer) Data() []byte {
	return s.buf
}

const (
	endOfErrors        uint8 = 0
	serializedErrorVal uint8 = 1
	serializedErrorPtr uint8 = 2
	emulatedError      uint8 = 3
)

func (s *Serializer) Error(err error) {
	seen := map[error]struct{}{}

	var dfs func(err error)
	dfs = func(e error) {
		if err == nil {
			return
		}
		if _, ok := seen[e]; ok {
			return
		}
		seen[e] = struct{}{}

		if am, ok := err.(AutoMarshal); ok {
			s.Uint8(serializedErrorVal)
			s.Any(am)
			return
		}

		if am, ok := pointerTo(err).(AutoMarshal); ok {
			s.Uint8(serializedErrorPtr)
			s.Any(am)
			return
		}

		s.Uint8(emulatedError)
		s.String(err.Error())

	}
	dfs(err)
	s.Uint8(endOfErrors)
}

func (s *Serializer) Any(value AutoMarshal) {
	s.String(typKey(value))
	value.AkasarMarshal(s)
}

func (s *Serializer) Len(l int) {
	if l < -1 {
		panic(makeSerializerError("unable to encode a negative length %d", l))
	}
	if l > math.MaxUint32 {
		panic(makeSerializerError("length can't be represented in 4 bytes"))
	}

	s.Int32(int32(l))
}
