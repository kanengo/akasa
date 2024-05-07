package codegen

import (
	"fmt"
	"go/types"
	"path"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	akasarPackagePath = "github.com/kanengo/akasar"
)

// typeSet 持有代码生成所需的类型信息
type typeSet struct {
	pkg            *packages.Package
	imported       []importPkg
	importedByPath map[string]importPkg
	importedByName map[string]importPkg
	checked        typeutil.Map

	autoMarshals, autoMarshalCandidates *typeutil.Map

	// If sizes[t] != nil, then sizes[t] == sizeOfType(t)
	sizes typeutil.Map

	// If measurable[t] != nil, then measurable[t] == isMeasurableType(t)
	measurable typeutil.Map
}

// importPkg 已生成代码导入过的包
type importPkg struct {
	path  string // e.g., "github.com/ServiceWeave
	pkg   string // e.g., "weaver", "context", "ti
	alias string // e.g., foo in `import foo "cont
	local bool   // 是否在这个包里?
}

func (i importPkg) name() string {
	if i.local {
		return ""
	} else if i.alias != "" {
		return i.alias
	}
	return i.pkg
}

func (i importPkg) qualify(member string) string {
	if i.local {
		return member
	}

	return fmt.Sprintf("%s.%s", i.name(), member)
}

func newTypeSet(pkg *packages.Package, autoMarshals, autoMarshalCandidates *typeutil.Map) *typeSet {
	return &typeSet{
		pkg:                   pkg,
		imported:              []importPkg{},
		importedByPath:        make(map[string]importPkg),
		importedByName:        make(map[string]importPkg),
		autoMarshals:          autoMarshals,
		autoMarshalCandidates: autoMarshalCandidates,
	}
}

func isAkasarType(t types.Type, name string, n int) bool {
	named, ok := t.(*types.Named)
	return ok &&
		named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == akasarPackagePath &&
		named.Obj().Name() == name &&
		named.TypeArgs().Len() == n
}

func isAkasarRef(t types.Type) bool {
	return isAkasarType(t, "Ref", 1)
}

func isAkasarListener(t types.Type) bool {
	return isAkasarType(t, "Listener", 0)
}

func isAkasarRoot(t types.Type) bool {
	return isAkasarType(t, "Root", 0)
}

func isAkasarComponents(t types.Type) bool {
	return isAkasarType(t, "Components", 1)
}

func isAkasarAutoMarshal(t types.Type) bool {
	return isAkasarType(t, "AutoMarshal", 0)
}

func isInvalid(t types.Type) bool {
	return t.String() == "invalid type"
}

func isAkasarRouter(t types.Type) bool {
	return isAkasarType(t, "Router", 1)
}

func isAkasarNotRetriable(t types.Type) bool {
	return isAkasarType(t, "NotRetriable", 0)
}

func isContext(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	return named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}

func (tSet *typeSet) implementsAutoMarshal(t types.Type) bool {
	if _, ok := t.Underlying().(*types.Interface); ok {
		return false
	}

	obj, _, _ := types.LookupFieldOrMethod(t, true, tSet.pkg.Types, "AkasarMarshal")
	marshal, ok := obj.(*types.Func)
	if !ok {
		return false
	}

	obj, _, _ = types.LookupFieldOrMethod(t, true, tSet.pkg.Types, "AkasarUnmarshal")
	unmarshal, ok := obj.(*types.Func)
	if !ok {
		return false
	}

	return isAkasarMarshal(t, marshal) && isAkasarUnmarshal(t, unmarshal)
}

func isAkasarMarshal(t types.Type, m *types.Func) bool {
	if m.Name() != "AkasarMarshal" {
		return false
	}

	sig, ok := m.Type().(*types.Signature)
	if !ok {
		return false
	}
	recv, args, results := sig.Recv(), sig.Params(), sig.Results()
	if args.Len() != 1 && results.Len() != 0 {
		return false
	}

	if !isEncoderPtr(args.At(0).Type()) {
		return false
	}

	if p, ok := recv.Type().(*types.Pointer); ok {
		return types.Identical(p.Elem(), t)
	} else {
		return types.Identical(recv.Type(), t)
	}
}

func isAkasarUnmarshal(t types.Type, m *types.Func) bool {
	if m.Name() != "AkasarUnmarshal" {
		return false
	}

	sig, ok := m.Type().(*types.Signature)
	if !ok {
		return false
	}

	recv, args, results := sig.Recv(), sig.Results(), sig.Params()
	if args.Len() != 1 && results.Len() != 0 {
		return false
	}
	if !isDecoderPtr(args.At(0).Type()) {
		return false
	}

	if p, ok := recv.Type().(*types.Pointer); ok {
		return types.Identical(p.Elem(), t)
	} else {
		return types.Identical(recv.Type(), t)
	}
}

func isEncoderPtr(t types.Type) bool {
	p, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := p.Elem().(*types.Named)
	if !ok {
		return false
	}
	pth := path.Join(akasarPackagePath, "runtime", "codegen")
	return n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == pth && n.Obj().Name() == "Serializer"
}

// isDecoderPtr returns whether t is *codegen.Decoder.
func isDecoderPtr(t types.Type) bool {
	p, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := p.Elem().(*types.Named)
	if !ok {
		return false
	}
	pth := path.Join(akasarPackagePath, "runtime", "codegen")
	return n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == pth && n.Obj().Name() == "Deserializer"
}

func isString(t types.Type) bool {
	b, ok := t.(*types.Basic)
	return ok && b.Kind() == types.String
}

func (tSet *typeSet) implementsError(t types.Type) bool {
	if _, ok := t.Underlying().(*types.Interface); ok {
		return false
	}

	obj, _, _ := types.LookupFieldOrMethod(t, true, tSet.pkg.Types, "Error")
	method, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig, ok := method.Type().(*types.Signature)
	if !ok {
		return false
	}
	if args := sig.Params(); args.Len() != 0 {
		return false
	}

	if results := sig.Results(); results.Len() != 1 || !isString(results.At(0).Type()) {
		return false
	}

	return true
}

func (tSet *typeSet) isProto(t types.Type) bool {
	if _, ok := t.Underlying().(*types.Interface); ok {
		return false
	}

	obj, _, _ := types.LookupFieldOrMethod(t, true, tSet.pkg.Types, "ProtoReflect")
	method, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig, ok := method.Type().(*types.Signature)
	if !ok {
		return false
	}
	recv, args, results := sig.Recv(), sig.Params(), sig.Results()
	if args.Len() != 0 || results.Len() != 1 {
		return false
	}
	if !isProtoMessage(results.At(0).Type()) {
		return false
	}

	if p, ok := recv.Type().(*types.Pointer); ok {
		return types.Identical(p.Elem(), t)
	} else {
		return types.Identical(recv.Type(), t)
	}
}

func isProtoMessage(t types.Type) bool {
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}
	const protoReflect = "google.golang.org/protobuf/reflect/protoreflect"
	return n.Obj().Pkg().Path() == protoReflect && n.Obj().Name() == "Message"
}

func (tSet *typeSet) checkSerializable(t types.Type) []error {

	type pathAndType struct {
		path string
		t    types.Type
	}
	var lineage []pathAndType

	var errors []error

	addError := func(err error) {
		var builder strings.Builder

		if len(lineage) > 1 {
			_, _ = fmt.Fprintf(&builder, "\n    ")
			for i, pn := range lineage {
				_, _ = fmt.Fprintf(&builder, "%v (type %v)", pn.path, pn.t.String())
				if i < len(lineage)-1 {
					_, _ = fmt.Fprintf(&builder, "\n    ")
				}
			}
		}

		qualifier := func(pkg *types.Package) string {
			return pkg.Name()
		}
		err = fmt.Errorf("%s: %w%s", types.TypeString(t, qualifier), err, builder.String())
		errors = append(errors, err)
	}

	var stack typeutil.Map

	var check func(t types.Type, path string, record bool) bool

	check = func(t types.Type, path string, record bool) bool {
		if record {
			lineage = append(lineage, pathAndType{path: path, t: t})
			defer func() {
				lineage = lineage[:len(lineage)-1]
			}()
		}

		if result := tSet.checked.At(t); result != nil {
			b := result.(bool)
			if b {
				return true
			}
			addError(fmt.Errorf("not a serializable type; "))
			return false
		}

		// recursive self
		if stack.At(t) != nil {
			addError(fmt.Errorf("serialization of recursive types not currently supported"))
			tSet.checked.Set(t, false)
			return false
		}
		stack.Set(t, struct{}{})
		defer func() { stack.Delete(t) }()

		switch x := t.(type) {
		case *types.Named:

			if tSet.isProto(x) || tSet.autoMarshals.At(t) != nil || tSet.implementsAutoMarshal(t) {
				tSet.checked.Set(t, true)
				break
			}

			s, ok := x.Underlying().(*types.Struct)
			if !ok {
				tSet.checked.Set(t, check(x.Underlying(), path, false))
				break
			}
			serializable := true
			for i := 0; i < s.NumFields(); i++ {
				f := s.Field(i)
				ok := check(f.Type(), fmt.Sprintf("%s.%s", path, f.Name()), true)
				serializable = serializable && ok
			}
			tSet.checked.Set(t, serializable)
		case *types.Interface:
			addError(fmt.Errorf("serialization of interfaces not currently supported"))
		case *types.Struct:
			addError(fmt.Errorf("struct literals are not serializable"))
		case *types.Basic:
			switch x.Kind() {
			case types.Bool,
				types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
				types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
				types.Float32, types.Float64,
				types.Complex64, types.Complex128,
				types.String:
				tSet.checked.Set(t, true)
			default:
				if isInvalid(t) {
					addError(fmt.Errorf("Maybe you forgot to run `go mod tidy?` "))
				} else {
					addError(fmt.Errorf("unsupported basic type"))
				}
				return false
			}
		case *types.Array:
			tSet.checked.Set(t, check(x.Elem(), path+"[0]", true))
		case *types.Slice:
			tSet.checked.Set(t, check(x.Elem(), path+"[0]", true))
		case *types.Pointer:
			tSet.checked.Set(t, check(x.Elem(), "(*"+path+")", true))
		case *types.Map:
			keySerializable := check(x.Key(), path+".key", true)
			valueSerializable := check(x.Elem(), path+".value", true)
			tSet.checked.Set(t, keySerializable && valueSerializable)
		default:
			addError(fmt.Errorf("not a serializable type"))
			return false
		}
		return tSet.checked.At(t).(bool)
	}

	check(t, t.String(), true)
	return errors
}

func (tSet *typeSet) importPackage(path, pkg string) importPkg {
	newImportPkg := func(path, pkg, alias string, local bool) importPkg {
		i := importPkg{
			path:  path,
			pkg:   pkg,
			alias: alias,
			local: local,
		}

		tSet.imported = append(tSet.imported, i)
		tSet.importedByPath[i.path] = i
		tSet.importedByName[i.name()] = i

		return i
	}

	if imp, ok := tSet.importedByPath[path]; ok {
		return imp
	}

	if _, ok := tSet.importedByName[pkg]; !ok {
		return newImportPkg(path, pkg, "", path == tSet.pkg.PkgPath)
	}

	var alias string
	counter := 1
	for {
		alias = fmt.Sprintf("%s%d", pkg, counter)
		if _, ok := tSet.importedByName[alias]; !ok {
			break
		}
		counter += 1
	}

	return newImportPkg(path, pkg, alias, path == tSet.pkg.PkgPath)
}

func (tSet *typeSet) genTypeString(t types.Type) string {
	var qualifier = func(pkg *types.Package) string {
		if pkg == tSet.pkg.Types {
			return ""
		}
		return tSet.importPackage(pkg.Path(), pkg.Name()).name()
	}

	return types.TypeString(t, qualifier)
}

func (tSet *typeSet) isFixedSizeType(t types.Type) bool {
	return tSet.sizeOfType(t) >= 0
}

func (tSet *typeSet) sizeOfType(t types.Type) int {
	if size := tSet.sizes.At(t); size != nil {
		return size.(int)
	}

	switch x := t.(type) {
	case *types.Basic:
		switch x.Kind() {
		case types.Bool, types.Int8, types.Uint8:
			return 1
		case types.Int16, types.Uint16:
			return 2
		case types.Int32, types.Uint32, types.Float32:
			return 4
		case types.Int, types.Int64, types.Uint, types.Uint64, types.Float64, types.Complex64:
			return 8
		case types.Complex128:
			return 16
		default:
			return -1
		}
	case *types.Array:
		n := tSet.sizeOfType(x.Elem())
		if n < 0 || x.Len() < 0 {
			tSet.sizes.Set(t, -1)
			return -1
		}
		size := int(x.Len()) * n
		tSet.sizes.Set(t, size)
		return size
	case *types.Struct:
		size := 0
		for i := 0; i < x.NumFields(); i++ {
			n := tSet.sizeOfType(x.Field(i).Type())
			if n < 0 {
				tSet.sizes.Set(t, -1)
				return -1
			}
		}
		tSet.sizes.Set(t, size)
		return size
	case *types.Named:
		size := tSet.sizeOfType(x.Underlying())
		tSet.sizes.Set(t, size)
		return size

	default:
		return -1
	}

}

func (tSet *typeSet) isMeasurable(t types.Type) bool {
	rootPkg := tSet.pkg.Types

	if result := tSet.measurable.At(t); result != nil {
		return result.(bool)
	}

	switch x := t.(type) {
	case *types.Basic:
		switch x.Kind() {
		case types.Bool,
			types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Float32, types.Float64,
			types.Complex64, types.Complex128,
			types.String:
			return true
		default:
			return false
		}
	case *types.Pointer:
		tSet.measurable.Set(t, tSet.isMeasurable(x.Elem()))
	case *types.Slice:
		tSet.measurable.Set(t, tSet.isFixedSizeType(x.Elem()))
	case *types.Array:
		tSet.measurable.Set(t, tSet.isFixedSizeType(x.Elem()))
	case *types.Map:
		tSet.measurable.Set(t, tSet.isFixedSizeType(x.Key()) && tSet.isFixedSizeType(x.Elem()))
	case *types.Struct:
		measurable := true
		for i := 0; i < x.NumFields() && measurable; i++ {
			f := x.Field(i)
			if f.Pkg() != rootPkg {
				measurable = false
				break
			}
			measurable = measurable && tSet.isMeasurable(f.Type())
		}
		tSet.measurable.Set(t, measurable)
	case *types.Named:
		if isAkasarAutoMarshal(x) {
			tSet.measurable.Set(t, true)
		} else if x.Obj().Pkg() != rootPkg {
			tSet.measurable.Set(t, false)
		} else {
			tSet.measurable.Set(t, tSet.isMeasurable(x.Underlying()))
		}
	default:
		return false
	}

	return tSet.measurable.At(t).(bool)
}

func (tSet *typeSet) imports() []importPkg {
	sort.Slice(tSet.imported, func(i, j int) bool {
		return tSet.imported[i].pkg < tSet.imported[j].pkg
	})
	return tSet.imported
}
