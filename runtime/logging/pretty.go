package logging

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kanengo/akasar/runtime/protos"
)

type PrettyFormatter interface {
	Format(*protos.LogEntry) string
}

// PrettyPrinter pretty prints log entries to json string.
type PrettyPrinter struct {
	mu  sync.Mutex
	b   strings.Builder
	sep string //写入下一个key前的分隔符
}

func NewPrettyPrinter() *PrettyPrinter {
	return &PrettyPrinter{}
}

const (
	componentKey = "component"
	nodeKey      = "node"
)

func (p *PrettyPrinter) Format(entry *protos.LogEntry) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.b.Reset()
	p.sep = ""

	p.b.WriteByte('{')

	// level
	p.appendKey(slog.LevelKey)
	p.appendString(entry.Level)

	// time
	if entry.TimeMicros != 0 {
		t := time.UnixMicro(entry.TimeMicros)
		p.appendKey(slog.TimeKey)
		p.appendTime(t)
	}

	// component
	c := ShortenComponent(entry.Component)
	p.appendKey(componentKey)
	p.appendString(c)

	// node
	p.appendKey(nodeKey)
	p.appendString(Shorten(entry.Node))

	// file and line
	if entry.File != "" && entry.Line != -1 {
		file := filepath.Base(entry.File)
		s := fmt.Sprintf("%s:%d", file, entry.Line)
		p.appendKey(slog.SourceKey)
		p.appendString(s)
	}

	// msg
	p.appendKey(slog.MessageKey)
	p.appendString(entry.Msg)

	// attrs
	for i := 0; i < len(entry.Attrs); i += 2 {
		p.appendKey(entry.Attrs[i])
		p.appendString(entry.Attrs[i+1])
	}

	p.b.WriteByte('}')

	return p.b.String()
}

func (p *PrettyPrinter) appendKey(key string) {
	p.b.WriteString(p.sep)
	p.appendString(key)
	p.b.WriteByte(':')
	p.sep = ","
}

func (p *PrettyPrinter) appendString(str string) {
	p.b.WriteByte('"')
	appendEscapedJSONString(&p.b, str)
	p.b.WriteByte('"')
}

func (p *PrettyPrinter) appendTime(t time.Time) {
	p.appendString(t.Format("2006-01-02 15:04:05.000000"))
}

// Shorten 返回一个s的短前缀
func Shorten(s string) string {
	const n = 8
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func ShortenComponent(component string) string {
	parts := strings.Split(component, "/")
	switch len(parts) {
	case 0:
		return "nil"
	case 1:
		return parts[0]
	default:
		return fmt.Sprintf("%s.%s", parts[len(parts)-2], parts[len(parts)-1])
	}
}

func appendEscapedJSONString(sb *strings.Builder, s string) {
	char := func(b byte) { sb.WriteByte(b) }
	str := func(s string) { sb.WriteString(s) }

	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if safeSet[b] {
				i++
				continue
			}
			if start < i {
				str(s[start:i])
			}
			char('\\')
			switch b {
			case '\\', '"':
				char(b)
			case '\n':
				char('n')
			case '\r':
				char('r')
			case '\t':
				char('t')
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				str(`u00`)
				char(hex[b>>4])
				char(hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				str(s[start:i])
			}
			str(`\ufffd`)
			i += size
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				str(s[start:i])
			}
			str(`\u202`)
			char(hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		str(s[start:])
	}
	return
}

const hex = "0123456789abcdef"

var safeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      true,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      true,
	'=':      true,
	'>':      true,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}
