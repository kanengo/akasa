package codegen

import (
	"fmt"
	"github.com/kanengo/akasar/runtime/pool"
	"testing"
)

func TestSerializer(t *testing.T) {
	var a int = 10
	var b int8 = 8
	var c int16 = 16
	var d int32 = -32
	var e int64 = 64

	var s string = "hello, serializer"

	var f1 float32 = -32.32
	var f2 float64 = 64.64

	var bl = true

	var bn []byte = make([]byte, 0)

	var bs = []byte{1, 2, 3, 4, 5, 6, 7}

	size := 8 + 1 + 2 + 4 + 8 + 4 + len(s) + 4 + 8 + 1 + 4 + 4 + 7

	sere := NewSerializer(size)
	fmt.Printf("buf cap:%d len:%d\n", cap(*sere.buf), len(*sere.buf))

	sere.Int(a)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Int8(b)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Int16(c)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Int32(d)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Int64(e)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.String(s)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Float32(f1)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Float64(f2)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Bool(bl)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	sere.Bytes(bn)
	fmt.Printf("bytes nil buf len:%d\n", len(*sere.buf))
	sere.Bytes(bs)
	fmt.Printf("buf len:%d\n", len(*sere.buf))
	_ = pool.FreePowerOfTwoSizeBytes(sere.Data())

	fmt.Println("==========================deserializer============================")

	dese := NewDeserializer(*sere.buf)

	fmt.Printf("deserialize data:%v\n", dese.Int())
	fmt.Printf("deserialize data:%v\n", dese.Int8())
	fmt.Printf("deserialize data:%v\n", dese.Int16())
	fmt.Printf("deserialize data:%v\n", dese.Int32())
	fmt.Printf("deserialize data:%v\n", dese.Int64())
	fmt.Printf("deserialize data:%v\n", dese.String())
	fmt.Printf("deserialize data:%v\n", dese.Float32())
	fmt.Printf("deserialize data:%v\n", dese.Float64())
	fmt.Printf("deserialize data:%v\n", dese.Bool())
	fmt.Printf("deserialize data:%v\n", dese.Bytes() == nil)
	fmt.Printf("deserialize data:%v\n", dese.Bytes())

}
