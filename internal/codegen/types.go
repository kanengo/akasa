package codegen

import (
	"fmt"
	"go/types"
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
	sizes          typeutil.Map
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

func newTypeSet(pkg *packages.Package) *typeSet {
	return &typeSet{
		pkg:            pkg,
		imported:       []importPkg{},
		importedByPath: make(map[string]importPkg),
		importedByName: make(map[string]importPkg),
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

func isInvalid(t types.Type) bool {
	return t.String() == "invalid type"
}

func isAkasarRouter(t types.Type) bool {
	return isAkasarType(t, "Router", 1)
}

func isContext(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}

	return named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
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
