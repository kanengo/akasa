package logging

import (
	"fmt"
	"testing"
	"time"

	"github.com/kanengo/akasar/runtime/protos"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

func TestJsonPrinter_Format(t *testing.T) {
	e := &protos.LogEntry{
		App:        "",
		Version:    gonanoid.Must(16),
		Component:  "github.com/kanengo/akasar/component/user",
		Node:       gonanoid.Must(16),
		TimeMicros: time.Now().UnixMicro(),
		Level:      "info",
		File:       "E:\\Codes\\github\\akasar\\runtime\\logging\\pretty.go",
		Line:       42,
		Msg:        "test json printer",
		Attrs:      []string{"id", "123123213", "name", "leeka\nhello"},
	}

	p := PrettyPrinter{}
	fmt.Println(p.Format(e))
}
