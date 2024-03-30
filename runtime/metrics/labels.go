package metrics

import (
	"fmt"
	"reflect"
	"unicode"
	"unicode/utf8"
)

func unExport(s string) string {
	if s == "" {
		return ""
	}

	r, n := utf8.DecodeRuneInString(s)

	return string(unicode.ToLower(r)) + s[n:]
}

// typeCheckLabels 检查L类型是否符合label的约定
// L 必须是结构体,结构体字段必须是预先声明(string,error) 或未定义的(*T, struct, []int)，或者是未定义类型的别名
// 具体支持的类型见下面的switch
func typeCheckLabels[L comparable]() error {
	var x L
	t := reflect.TypeOf(x)

	if t.Kind() != reflect.Struct {
		return fmt.Errorf("metric labels: type %T is not a struct", x)
	}

	names := map[string]struct{}{}
	for i := 0; i < t.NumField(); i++ {
		fi := t.Field(i)

		if fi.Type.PkgPath() != "" {
			return fmt.Errorf("metic labels: field %q of type %T has unsupported type %v",
				fi.Name, x, fi.Type.Name())
		}

		switch fi.Type.Kind() {
		case reflect.String,
			reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		default:
			return fmt.Errorf("metic labels: field %q of type %T has unsupported type %v",
				fi.Name, x, fi.Type.Name())
		}

		if !fi.IsExported() {
			return fmt.Errorf("metric labels: filed %q of type %T is unexported", fi.Name, x)
		}

		name := unExport(fi.Name)
		if alias, ok := fi.Tag.Lookup("akasar"); ok {
			name = alias
		}

		if _, ok := names[name]; ok {
			return fmt.Errorf("metric labels: type %T has duplicate field %q", x, fi.Name)
		}
		names[name] = struct{}{}
	}

	return nil
}

// labelExtractor 通过label结构体L提取labels
type labelExtractor[L comparable] struct {
	fields []field
}

type field struct {
	f    reflect.StructField
	name string //字段名, 或者别名
}

// newLabelExtractor
func newLabelExtractor[L comparable]() *labelExtractor[L] {
	var x L
	t := reflect.TypeOf(x)

	fields := make([]field, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		fi := t.Field(i)

		if alias, ok := fi.Tag.Lookup("akasar"); ok {
			fields[i] = field{
				f:    fi,
				name: alias,
			}
		} else {
			fields[i] = field{
				f:    fi,
				name: unExport(fi.Name),
			}
		}
	}

	return &labelExtractor[L]{fields}
}

func (l *labelExtractor[L]) Extract(labels L) map[string]string {
	v := reflect.ValueOf(labels)
	var extracted map[string]string
	if len(l.fields) == 0 {
		return extracted
	}
	extracted = make(map[string]string, len(l.fields))

	for _, field := range l.fields {
		extracted[field.name] = fmt.Sprint(v.FieldByIndex(field.f.Index).Interface())
	}

	return extracted
}
