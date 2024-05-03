package codegen

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"

	"google.golang.org/protobuf/proto"

	"github.com/kanengo/akasar/internal/unsafex"
)

type deserializerError struct {
	err error
}

func (e deserializerError) Error() string {
	if e.err == nil {
		return "deserializer:"
	}

	return "deserializer:" + e.err.Error()
}

func makeDeserializerError(format string, args ...interface{}) serializerError {
	return serializerError{err: fmt.Errorf(format, args...)}
}

type Deserializer struct {
	buf   []byte
	index int
}

func NewDeserializer(buf []byte) *Deserializer {
	return &Deserializer{buf: buf}
}

func (d *Deserializer) check(n int) {
	if len(d.buf[d.index:]) < n {
		panic(makeDeserializerError("deserializer: not enough space to deserialize"))
	}
}

func (d *Deserializer) Error() error {
	var list []error
	for {
		tag := d.Uint8()
		if tag == endOfErrors {
			break
		} else if tag == serializedErrorVal {
			val := d.Any()
			if e, ok := val.(error); ok {
				list = append(list, e)
				continue
			}
			panic(fmt.Sprintf("received type %T which is not an error", val))
		} else if tag == serializedErrorPtr {
			val := d.Any()
			if e, ok := pointee(val).(error); ok {
				list = append(list, e)
				continue
			}
			panic(fmt.Sprintf("received type %T which is not a pointer to error", val))
		} else if tag == emulatedError {
			msg := d.String()
			list = append(list, errors.New(msg))
		}
	}

	if len(list) == 1 {
		return list[0]
	}

	return errors.Join(list...)
}

func (d *Deserializer) Any() any {
	key := d.String()

	typesMu.Lock()
	defer typesMu.Unlock()

	t, ok := types[key]
	if !ok {
		panic(fmt.Sprintf("received value for non-registered type %q", key))
	}

	var ptr reflect.Value
	if t.Kind() == reflect.Pointer {
		ptr = reflect.New(t.Elem())
	} else {
		ptr = reflect.New(t)
	}

	am, ok := ptr.Interface().(AutoMarshal)
	if !ok {
		panic(fmt.Sprintf("received value for non-serializable type %v", t))
	}
	am.AkasarUnmarshal(d)

	result := ptr
	if t.Kind() != reflect.Pointer {
		result = ptr.Elem()
	}

	return result.Interface()
}

func (d *Deserializer) Uint64() (val uint64) {
	size := 8
	d.check(size)

	val = binary.LittleEndian.Uint64(d.buf[d.index : d.index+size])
	d.index += size

	return
}

func (d *Deserializer) Uint32() (val uint32) {
	size := 4
	d.check(size)

	val = binary.LittleEndian.Uint32(d.buf[d.index : d.index+size])
	d.index += size

	return
}

func (d *Deserializer) Uint16() (val uint16) {
	size := 2
	d.check(size)

	val = binary.LittleEndian.Uint16(d.buf[d.index : d.index+size])
	d.index += size

	return
}

func (d *Deserializer) Uint8() (val uint8) {
	size := 1
	d.check(size)

	val = d.buf[d.index]
	d.index += size

	return
}

func (d *Deserializer) Uint() (val uint) {
	val = uint(d.Uint64())

	return
}

func (d *Deserializer) Byte() (val byte) {
	val = d.Uint8()
	return
}

func (d *Deserializer) Int64() (val int64) {
	val = int64(d.Uint64())
	return
}

func (d *Deserializer) Int32() (val int32) {
	val = int32(d.Uint32())
	return
}

func (d *Deserializer) Int16() (val int16) {
	val = int16(d.Uint16())
	return
}

func (d *Deserializer) Int8() (val int8) {
	val = int8(d.Uint8())
	return
}

func (d *Deserializer) Int() (val int) {
	val = int(d.Uint64())

	return
}

func (d *Deserializer) Bool() (val bool) {
	b := d.Uint8()
	if b == 1 {
		val = true
	} else {
		val = false
	}

	return
}

func (d *Deserializer) Float32() (val float32) {
	val = math.Float32frombits(d.Uint32())

	return
}

func (d *Deserializer) Float64() (val float64) {
	val = math.Float64frombits(d.Uint64())

	return
}

func (d *Deserializer) Complex64() (val complex64) {
	val = complex(d.Float32(), d.Float32())
	return
}

func (d *Deserializer) Complex128() (val complex128) {
	val = complex(d.Float64(), d.Float64())
	return
}

func (d *Deserializer) String() (val string) {
	size := int(d.Uint32())

	val = unsafex.BytesToString(d.buf[d.index : d.index+size])
	d.index += size

	return val
}

func (d *Deserializer) UnmarshalProto(value proto.Message) {
	if err := proto.Unmarshal(d.Bytes(), value); err != nil {
		panic(makeDeserializerError("error decoding to proto %T: %w", value, err))
	}
}

func (d *Deserializer) Bytes() (val []byte) {
	size := d.Uint32()
	if int32(size) == -1 {
		return nil
	}

	val = d.buf[d.index : d.index+int(size)]
	d.index += int(size)

	return
}

func (d *Deserializer) Len() int {
	n := int(d.Int32())
	if n < -1 {
		panic(makeDeserializerError("length can't be smaller than -1"))
	}

	return n
}
