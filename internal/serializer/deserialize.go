package serializer

import (
	"encoding/binary"
	"math"

	"github.com/kanengo/akasa/internal/unsafex"
)

type Deserializer struct {
	buf   []byte
	index int
}

func NewDeserializer(buf []byte) *Deserializer {
	return &Deserializer{buf: buf}
}

func (d *Deserializer) check(n int) {
	if len(d.buf[d.index:]) < n {
		panic(makeSerializerError("deserializer: not enough space to deserialize"))
	}
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

func (d *Deserializer) Bytes() (val []byte) {
	size := d.Uint32()
	if int32(size) == -1 {
		return nil
	}

	val = d.buf[d.index : d.index+int(size)]
	d.index += int(size)

	return
}
