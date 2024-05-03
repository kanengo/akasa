package codegen

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/kanengo/akasar/internal/files"

	"golang.org/x/tools/go/types/typeutil"

	"golang.org/x/exp/maps"

	akasa "github.com/kanengo/akasar"

	"golang.org/x/tools/go/packages"
)

const (
	generatedCodeFile = "akasar_gen.go"

	Usage = `Generate code for a Service Akasar application.
Usage:
  akasar generate [packages]

Description:
  generate code for a Service Akasar application in the provided packages.

Examples:
  # Generate code for the package int the current directory.
  akasar generate

  # Generate code for the package in the ./foo directory.
  akasar generate ./foo

  # Generate code for all packages in all sub directories of current directory.
  akasar generate ./...`
)

type generator struct {
	akasa.Components[int]
	tSet       *typeSet
	fileSet    *token.FileSet
	pkg        *packages.Package
	components []*component

	autoMarshals          *typeutil.Map //types that implement AutoMarshal
	autoMarshalCandidates *typeutil.Map //types that declare themselves AutoMarshal

	sizeFuncNeeded typeutil.Map
	generated      typeutil.Map
}

// component 解释代码文件后的组件类型信息
type component struct {
	intf      *types.Named
	impl      *types.Named
	router    *types.Named
	isRoot    bool
	refs      []*types.Named
	listeners []string
	noRetry   map[string]struct{}
}

func (c *component) fullIntfName() string {
	return fullName(c.intf)
}

func (c *component) intfName() string {
	return c.intf.Obj().Name()
}

func (c *component) implName() string {
	return c.impl.Obj().Name()
}

func (c *component) methods() []*types.Func {
	underlying := c.intf.Underlying().(*types.Interface)
	methods := make([]*types.Func, underlying.NumMethods())

	for i := 0; i < underlying.NumMethods(); i++ {
		methods[i] = underlying.Method(i)
	}

	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name() < methods[j].Name()
	})

	return methods
}

func fullName(t *types.Named) string {
	return path.Join(t.Obj().Pkg().Path(), t.Obj().Name())
}

func newGenerator(pkg *packages.Package, fSet *token.FileSet, autoMarshals *typeutil.Map) (*generator, error) {
	//Abort if there were any uerrors loading the package.
	var errs []error
	for _, err := range pkg.Errors {
		errs = append(errs, err)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	tSet := newTypeSet(pkg, autoMarshals, &typeutil.Map{})
	for _, file := range pkg.Syntax {
		filename := fSet.Position(file.Package).Filename
		if filepath.Base(filename) == generatedCodeFile {
			continue
		}

		ts, err := findAutoMarshals(pkg, file)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, t := range ts {
			tSet.autoMarshalCandidates.Set(t, struct{}{})
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	// 一个嵌入了 akasar.AutoMarshal 的类型并不意味着可以自动marshal它
	for _, t := range tSet.autoMarshalCandidates.Keys() {
		n := t.(*types.Named)
		if err := errors.Join(tSet.checkSerializable(n)...); err != nil {
			errs = append(errs, errorf(fSet, n.Obj().Pos(), "type %v is not serializable\n%w", t, err))
			continue
		}
		tSet.autoMarshals.Set(t, struct{}{})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	components := make(map[string]*component)
	for _, file := range pkg.Syntax {
		filename := fSet.Position(file.Package).Filename
		if filepath.Base(filename) == generatedCodeFile {
			continue
		}

		fileComponents, err := findComponents(pkg, file, tSet)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, c := range fileComponents {
			if existing, ok := components[c.fullIntfName()]; ok {
				errs = append(errs, errorf(pkg.Fset, c.impl.Obj().Pos(),
					"Duplicate implementation for component %s, other declaration: %v",
					c.fullIntfName(), fSet.Position(existing.impl.Obj().Pos()),
				))
				continue
			}

			components[c.fullIntfName()] = c

		}
	}

	for _, file := range pkg.Syntax {
		filename := fSet.Position(file.Package).Filename
		if filepath.Base(filename) == generatedCodeFile {
			continue
		}

		if err := findMethodAttribute(pkg, file, components); err != nil {
			errs = append(errs, err)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &generator{
		pkg:        pkg,
		tSet:       tSet,
		fileSet:    fSet,
		components: maps.Values(components),
	}, nil
}

func findAutoMarshals(pkg *packages.Package, file *ast.File) ([]*types.Named, error) {
	var autoMarshals []*types.Named
	var errs []error

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			//This is not a type declaration.
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				panic(errorf(pkg.Fset, spec.Pos(), "type declaration has non-TypeSpec spec: %v", spec))
			}

			// Extract the type's name
			def, ok := pkg.TypesInfo.Defs[typeSpec.Name]
			if !ok {
				panic(errorf(pkg.Fset, spec.Pos(), "name %v not found", typeSpec.Name))
			}
			n, ok := def.Type().(*types.Named)
			if !ok {
				// For type aliases like `type Int = int`, Int has type int and
				// not type Named. We ignore these.
				continue
			}

			// Check if the type of the expression is struct.
			t, ok := pkg.TypesInfo.Types[typeSpec.Type].Type.(*types.Struct)
			if !ok {
				continue
			}

			autoMarshal := false
			for i := 0; i < t.NumFields(); i++ {
				field := t.Field(i)
				if field.Embedded() && isAkasarAutoMarshal(field.Type()) {
					autoMarshal = true
					break
				}
			}
			if !autoMarshal {
				continue
			}

			if n.TypeParams() != nil {
				errs = append(errs, errorf(pkg.Fset, spec.Pos(),
					"generic struct %v cannot embed akasar.AutoMarhsal",
					formatType(pkg, n),
				))
				continue
			}

			autoMarshals = append(autoMarshals, n)

		}

	}

	return autoMarshals, errors.Join(errs...)
}

func parseNowAkasarGenFile(fSet *token.FileSet, filename string, src []byte) (*ast.File, error) {
	if filepath.Base(filename) == generatedCodeFile {
		return parser.ParseFile(fSet, filename, src, parser.PackageClauseOnly)
	}

	return parser.ParseFile(fSet, filename, src, parser.ParseComments|parser.DeclarationErrors)
}

func errorf(fSet *token.FileSet, pos token.Pos, format string, args ...any) error {
	position := fSet.Position(pos)
	if cwd, err := filepath.Abs("."); err == nil {
		if filename, err := filepath.Rel(cwd, position.Filename); err == nil {
			position.Filename = filename
		}
	}

	prefix := position.String()
	return fmt.Errorf("%s: %w", prefix, fmt.Errorf(format, args...))
}

func formatType(currentPackage *packages.Package, t types.Type) string {
	qualifier := func(pkg *types.Package) string {
		if pkg == currentPackage.Types {
			return ""
		}
		return pkg.Name()
	}

	return types.TypeString(t, qualifier)
}

func findMethodAttribute(pkg *packages.Package, f *ast.File, components map[string]*component) error {
	var errs []error
	// 寻找设置了不可重试的方法
	// var _ akasar.NotRetriable = Component.Method
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR { //变量类型
			continue
		}

		for _, spec := range genDecl.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			typeAndValue, ok := pkg.TypesInfo.Types[valSpec.Type]
			if !ok {
				continue
			}
			t := typeAndValue.Type
			if !isAkasarNotRetriable(t) {
				continue
			}
			for _, val := range valSpec.Values {
				com, method, ok := findComponentMethod(pkg, components, val)
				if !ok {
					errs = append(errs, errorf(pkg.Fset, valSpec.Pos(), "akasar.NotRetriable should only be assigned a value that identifies a method of a component implemented by this package"))
					continue
				}
				if com.noRetry == nil {
					com.noRetry = map[string]struct{}{}
				}
				com.noRetry[method] = struct{}{}
			}

		}
	}

	return errors.Join(errs...)
}

func findComponentMethod(pkg *packages.Package, components map[string]*component, val ast.Expr) (*component, string, bool) {
	sel, ok := val.(*ast.SelectorExpr)
	if !ok {
		return nil, "", false
	}
	cTypeVal, ok := pkg.TypesInfo.Types[sel.X]
	if !ok {
		return nil, "", false
	}
	cType, ok := cTypeVal.Type.(*types.Named)
	if !ok {
		return nil, "", false
	}
	cName := fullName(cType)
	c, ok := components[cName]
	if !ok {
		return nil, "", false
	}
	method := sel.Sel.Name
	for _, m := range c.methods() {
		if m.Name() == method {
			return c, method, true
		}
	}

	return nil, "", false
}

func getListenerNamesFromStructFiled(pkg *packages.Package, f *ast.Field) ([]string, error) {
	// Try to get the listener name from the struct tag.
	if f.Tag != nil {
		tag := reflect.StructTag(strings.TrimPrefix(
			strings.TrimSuffix(f.Tag.Value, "`"), "`"))
		if len(f.Names) > 1 {
			return nil, errorf(pkg.Fset, f.Pos(),
				"Tag %s repeated for multiple fields", tag)
		}
		if name, ok := tag.Lookup("akasar"); ok {
			if !token.IsIdentifier(name) {
				return nil, errorf(pkg.Fset, f.Pos(),
					"Listener tag %s is not a valid Go identifier",
					tag,
				)
			}
			return []string{name}, nil
		}
	}

	// GEt the listener name(s) from the struct filed name(s).
	if f.Names == nil {
		return []string{"Listener"}, nil
	}
	var ret []string
	for _, fName := range f.Names {
		ret = append(ret, fName.Name)
	}

	return ret, nil
}

func validateMethods(pkg *packages.Package, tSet *typeSet, intf *types.Named) error {
	var errs []error
	underlying := intf.Underlying().(*types.Interface)
	for i := 0; i < underlying.NumMethods(); i++ {
		m := underlying.Method(i)
		signature, ok := m.Type().(*types.Signature)
		if !ok {
			panic(errorf(pkg.Fset, m.Pos(), "method %s doesn't have a signature.", m.Name()))
		}
		if !m.Exported() {
			errs = append(errs, errorf(pkg.Fset, m.Pos(),
				"Method %s%s %s of akasar component %q is unexported.",
				m.Name(), formatType(pkg, signature.Params()), formatType(pkg, signature.Results()), intf.Obj().Name()),
			)
			continue
		}

		// bad is a helper function for producing helpful error messages.
		bad := func(bad, format string, arg ...any) error {
			err := fmt.Errorf(format, arg...)
			return errorf(pkg.Fset, m.Pos(),
				"Method %s%s %s of akasar component %q has incorrect %s types. %w",
				m.Name(), formatType(pkg, signature.Params()), formatType(pkg, signature.Results()),
				intf.Obj().Name(), bad, err,
			)
		}

		// First argument must be context.Context
		if signature.Params().Len() < 1 || !isContext(signature.Params().At(0).Type()) {
			errs = append(errs, bad("argument", "The first argument must have type context.Context."))
		}

		// All arguments but context.Context must be serializable.
		for i := 1; i < signature.Params().Len(); i++ {
			arg := signature.Params().At(i)
			if err := errors.Join(tSet.checkSerializable(arg.Type())...); err != nil {
				errs = append(errs, bad("argument",
					"Argument %d has type %s, which is not serializable.\n%w",
					i, formatType(pkg, arg.Type()), err,
				))
			}
		}

		// Last result must be error
		if signature.Results().Len() < 1 || signature.Results().At(signature.Results().Len()-1).Type().String() != "error" {
			errs = append(errs, bad("return", "The last return must have type error."))
		}

		// All result but error must be serializable.
		for i := 0; i < signature.Results().Len()-1; i++ {
			res := signature.Results().At(i)
			if err := errors.Join(tSet.checkSerializable(res.Type())...); err != nil {
				errs = append(errs, bad("argument",
					"Return %d has type %s, which is not serializable.\n%w",
					i, formatType(pkg, res.Type()), err,
				))
			}
		}
	}

	return errors.Join(errs...)
}

// extractComponent 尝试从提供的TypeSpec提取出component
func extractComponent(pkg *packages.Package, file *ast.File, tSet *typeSet, spec *ast.TypeSpec) (*component, error) {
	// 检查TypeSpe是否是一个结构体类型.
	s, ok := spec.Type.(*ast.StructType)
	if !ok {
		return nil, nil
	}

	//获取pkg 语法树中的类型信息中的定义的对象信息
	def, ok := pkg.TypesInfo.Defs[spec.Name]
	if !ok {
		panic(errorf(pkg.Fset, spec.Pos(), "name %v not found", spec.Name))
	}

	//是否是已命名对象
	impl, ok := def.Type().(*types.Named)
	if !ok {
		return nil, nil
	}

	var intf *types.Named   //component接口类型
	var router *types.Named //component路由类型(如果有的话)
	var isRoot bool         //是否根组件
	var refs []*types.Named //引用的其他组件类型 Ref[T]
	var listeners []string  //监听器名称列表

	for _, f := range s.Fields.List {
		typeAndValue, ok := pkg.TypesInfo.Types[f.Type]
		if !ok {
			panic(errorf(pkg.Fset, f.Pos(), "type %v not found", f.Type))
		}
		t := typeAndValue.Type

		if isAkasarRef(t) {
			//akasar.Ref[T]
			arg := t.(*types.Named).TypeArgs().At(0)
			if isAkasarRoot(arg) {
				return nil, errorf(pkg.Fset, f.Pos(), "components cannot contain a reference to akasar.Root")
			}
			//T
			named, ok := arg.(*types.Named)
			if !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"akasar.Ref argument %s is not a named type.",
					formatType(pkg, arg),
				)
			}
			refs = append(refs, named)
		} else if isAkasarListener(t) {
			lis, err := getListenerNamesFromStructFiled(pkg, f)
			if err != nil {
				return nil, err
			}
			listeners = append(listeners, lis...)
		}

		if len(f.Names) != 0 {
			//忽略非嵌入的字段
			continue
		}

		switch {
		// The field f is an embedded akasar.Components[T].
		case isAkasarComponents(t):
			// Check that T is a named interface type inside the package
			arg := t.(*types.Named).TypeArgs().At(0)
			named, ok := arg.(*types.Named)
			if !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"akasar.Components argument %s is not a named type.",
					formatType(pkg, arg),
				)
			}
			isRoot = isAkasarRoot(arg)
			if !isRoot && named.Obj().Pkg() != pkg.Types {
				return nil, errorf(pkg.Fset, f.Pos(),
					"akasar.Components argument %s is a type outside the current package.",
					formatType(pkg, named),
				)
			}

			if _, ok := named.Underlying().(*types.Interface); !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"akasar.Components argument %s is not an interface.",
					formatType(pkg, named),
				)
			}
			intf = named

		// The field f is an embedded akasar.WithRouter[T].
		case isAkasarRouter(t):
			arg := t.(*types.Named).TypeArgs().At(0)
			named, ok := arg.(*types.Named)
			if !ok {
				return nil, errorf(pkg.Fset, f.Pos(),
					"akasar.WithRouter argument %s is not a named type",
					formatType(pkg, arg),
				)
			}
			if named.Obj().Pkg() != pkg.Types {
				return nil, errorf(pkg.Fset, f.Pos(),
					"akasar.WithRouter argument %s is a type outside the current package.",
					formatType(pkg, named),
				)
			}
			router = named
		}
	}

	if intf == nil {
		return nil, nil
	}

	// Check that the component implementation implements the component interface
	if !types.Implements(types.NewPointer(impl), intf.Underlying().(*types.Interface)) {
		return nil, errorf(pkg.Fset, spec.Pos(),
			"types %s embeds akasar.Components[%s] but does not implement interface %s.",
			formatType(pkg, impl), formatType(pkg, intf), formatType(pkg, intf),
		)
	}

	// Disallow generic component implementations.
	if spec.TypeParams != nil && spec.TypeParams.NumFields() != 0 {
		return nil, errorf(pkg.Fset, spec.Pos(),
			"component implementation %s is generic.",
			formatType(pkg, impl),
		)
	}

	if err := validateMethods(pkg, tSet, intf); err != nil {
		return nil, err
	}

	// Check that listener names are unique.
	seenLis := map[string]struct{}{}
	for _, lis := range listeners {
		if _, ok := seenLis[lis]; ok {
			return nil, errorf(pkg.Fset, spec.Pos(),
				"component implementation %s declares multiple listeners with name %s.",
				formatType(pkg, impl), lis,
			)
		}
	}

	if err := checkMisTypeInit(pkg, tSet, impl); err != nil {
		fmt.Println(err)
	}

	comp := &component{
		intf:      intf,
		impl:      impl,
		router:    router,
		isRoot:    isRoot,
		refs:      refs,
		listeners: listeners,
	}

	return comp, nil

}

func checkMisTypeInit(pkg *packages.Package, set *typeSet, impl *types.Named) error {
	for i := 0; i < impl.NumMethods(); i++ {
		m := impl.Method(i)
		if m.Name() != "Init" {
			continue
		}

		sig := m.Type().(*types.Signature)
		err := errorf(pkg.Fset, m.Pos(),
			`WARNING: Component %v's Init method has type "%v", not type func(context.Context) error.`,
			impl.Obj().Name(),
			sig,
		)

		if sig.Params().Len() != 1 || !isContext(sig.Params().At(0).Type()) {
			return err
		}

		if sig.Results().Len() != 1 || sig.Results().At(0).Type().String() != "error" {
			return err
		}
		return nil
	}

	return nil
}

func findComponents(pkg *packages.Package, f *ast.File, tSet *typeSet) ([]*component, error) {
	var components []*component
	var errs []error
	for _, d := range f.Decls {
		genDecl, ok := d.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			comp, err := extractComponent(pkg, f, tSet, ts)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if comp != nil {
				components = append(components, comp)
			}
		}
	}

	return components, errors.Join(errs...)
}

type printFn func(format string, args ...any)

func (g *generator) generate() error {
	if len(g.components) == 0 {
		return nil
	}

	sort.Slice(g.components, func(i, j int) bool {
		return g.components[i].intfName() < g.components[j].intfName()
	})

	var body bytes.Buffer
	{
		p := func(format string, args ...any) {
			_, _ = fmt.Fprintln(&body, fmt.Sprintf(format, args...))
		}
		g.generateRegisteredComponents(p)
		g.generateInstanceChecks(p)
		g.generateLocalStubs(p)
		g.generateClientStubs(p)
		g.generateSeverStubs(p)
		g.generateAutoMarshalMethods(p)
		g.generateEncDecMethods(p)

		if g.sizeFuncNeeded.Len() > 0 {
			p(`// Size implementations.`)
			p(``)

			keys := g.sizeFuncNeeded.Keys()
			sort.Slice(keys, func(i, j int) bool {
				x, y := keys[i], keys[j]
				return x.String() < y.String()
			})
			for _, t := range keys {
				g.generateSizeFunction(p, t)
			}
		}
	}

	var header bytes.Buffer
	{
		fn := func(format string, args ...any) {
			_, _ = fmt.Fprintln(&header, fmt.Sprintf(format, args...))
		}
		g.generateImports(fn)
	}

	filename := filepath.Join(g.pkgDir(), generatedCodeFile)
	dst := files.NewWriter(filename)
	defer dst.Cleanup()

	fmtAndWrite := func(buf bytes.Buffer) error {
		b := buf.Bytes()
		var err error
		//formatted, err := format.Source(b)
		//if err != nil {
		//	return fmt.Errorf("format.Source: %w", err)
		//}
		//b = formatted

		_, err = io.Copy(dst, bytes.NewReader(b))
		return err
	}

	if err := fmtAndWrite(header); err != nil {
		return err
	}
	if err := fmtAndWrite(body); err != nil {
		return err
	}

	return dst.Close()
}

// codegen imports and returns the codegen package.
func (g *generator) codegen() importPkg {
	p := fmt.Sprintf("%s/runtime/codegen", akasarPackagePath)

	return g.tSet.importPackage(p, "codegen")
}

func (g *generator) errorsPackage() importPkg {
	return g.tSet.importPackage("errors", "errors")
}

func (g *generator) codes() importPkg {
	return g.tSet.importPackage("go.opentelemetry.io/otel/codes", "codes")
}

func (g *generator) trace() importPkg {
	return g.tSet.importPackage("go.opentelemetry.io/otel/trace", "trace")
}

func (g *generator) akasa() importPkg {
	return g.tSet.importPackage(akasarPackagePath, "akasar")
}

func (g *generator) componentRef(com *component) string {
	if com.isRoot {
		return g.akasa().qualify("Root")
	}

	return com.intfName()
}

func noRetryString(com *component) string {
	list := make([]int, 0, len(com.noRetry))
	for i, m := range com.methods() {
		if _, ok := com.noRetry[m.Name()]; !ok {
			continue
		}
		list = append(list, i)
	}
	slices.Sort(list)
	strs := make([]string, 0, len(list))
	for _, i := range list {
		strs = append(strs, strconv.Itoa(i))
	}

	return strings.Join(strs, ",")
}

// encode returns a statement that encodes the expression e of type t into stub
// of type *codegen.Encoder. For example, encode("enc", "1", int) is
// "enc.Int(1)".
func (g *generator) encode(stub, e string, t types.Type) string {
	f := func(t types.Type) string {
		return fmt.Sprintf("serviceAkasarEnc_%s", sanitize(t))
	}

	// Let enc(stub, e: t) be the statement that encodes e into stub. [t] is
	// shorthand for sanitize(t). under(t) is the underlying type of t.
	//
	// enc(stub, e: basic type t) = stub.[t](e)
	// enc(stub, e: *t) = serviceAkasarEnc[*t](&stub, e)
	// enc(stub, e: [N]t) = serviceAkasarEnc[[N]t](&stub, &e)
	// enc(stub, e: []t) = serviceAkasarEnc[[]t](&stub, e)
	// enc(stub, e: map[k]v) = serviceAkasarEnc[map[k]v](&stub, e)
	// enc(stub, e: struct{...}) = serviceAkasarEnc[struct{...}](&stub, &e)
	// enc(stub, e: type t u) = stub.EncodeProto(&e)           // t implements proto.Message
	// enc(stub, e: type t u) = (e).WeaverMarshal(stub)         // t implements AutoMarshal
	// enc(stub, e: type t u) = stub.EncodeBinaryMarshaler(&e) // t implements BinaryMarshaler
	// enc(stub, e: type t u) = serviceAkasarEnc[t](&stub, &e)       // under(u) = struct{...}
	// enc(stub, e: type t u) = enc(&stub, under(t)(e))        // otherwise

	switch x := t.(type) {
	case *types.Basic:
		switch x.Kind() {
		case types.Bool,
			types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Float32, types.Float64,
			types.Complex64, types.Complex128,
			types.String:
			return fmt.Sprintf("%s.%s(%s)", stub, exported(x.Name()), e)
		default:
			panic(fmt.Sprintf("encode: unexpected expression: %v (type %T)", e, t))
		}
	case *types.Pointer:
		return fmt.Sprintf("%s(%s, %s)", f(x), stub, e)
	case *types.Array:
		return fmt.Sprintf("%s(%s, %s)", f(x), stub, ref(e))
	case *types.Slice:
		return fmt.Sprintf("%s(%s, %s)", f(x), stub, e)
	case *types.Map:
		return fmt.Sprintf("%s(%s, %s)", f(x), stub, e)
	case *types.Struct:
		return fmt.Sprintf("%s(%s, %s)", f(x), stub, ref(e))
	case *types.Named:
		if g.tSet.isProto(x) {
			return fmt.Sprintf("%s.MarshalProto(%s)", stub, ref(e))
		}
		under := x.Underlying()
		if g.tSet.autoMarshals.At(x) != nil || g.tSet.implementsAutoMarshal(x) {
			return fmt.Sprintf("(%s).AkasarMarshal(%s)", e, stub)
		}
		if _, ok := under.(*types.Struct); ok {
			return fmt.Sprintf("%s(%s, %s)", f(x), stub, ref(e))
		}
		return g.encode(stub, fmt.Sprintf("(%s)(%s)", g.tSet.genTypeString(x.Underlying()), e), under)
	default:
		panic(fmt.Sprintf("encode: unexpected expression: %v (type %T)", e, t))
	}
}

func (g *generator) decode(stub, v string, t types.Type) string {
	f := func(t types.Type) string {
		return fmt.Sprintf("serviceAkasarDec_%s", sanitize(t))
	}

	// Let dec(stub, v: t) be the statement that decodes a value of type t from
	// stub into v of type *t. [t] is shorthand for sanitize(t). under(t) is
	// the underlying type of t.
	//
	// dec(stub, v: basic type t) = *v := stub.[t](e)
	// dec(stub, v: *t) = *v := serviceAkasarDec_[*t](&stub)
	// dec(stub, v: [N]t) = serviceAkasarDec_[[N]t](stub, v)
	// dec(stub, v: []t) = v := *v = serviceAkasarDec_[[]t](stub)
	// dec(stub, v: map[k]v) = *v := serviceAkasarDec_[map[k]v](stub)
	// dec(stub, v: struct{...}) = serviceAkasarDec_[struct{...}](stub, &v)
	// dec(stub, v: type t u) = stub.DecodeProto(v)             // t implements proto.Message
	// dec(stub, v: type t u) = (v).WeaverUnmarshal(stub)        // t implements AutoMarshal
	// dec(stub, v: type t u) = stub.DecodeBinaryUnmarshaler(v) // t implements BinaryUnmarshaler
	// dec(stub, v: type t u) = serviceAkasarDec_[t](stub, v)          // under(u) = struct{...}
	// dec(stub, v: type t u) = dec(stub, (*under(t))(v))       // otherwise

	switch x := t.(type) {
	case *types.Basic:
		switch x.Kind() {
		case types.Bool,
			types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Float32, types.Float64,
			types.Complex64, types.Complex128,
			types.String:
			return fmt.Sprintf("%s = %s.%s()", deref(v), stub, exported(x.Name()))
		default:
			panic(fmt.Sprintf("encode: unexpected expression: %v (type %T)", v, t))
		}
	case *types.Pointer:
		return fmt.Sprintf("%s = %s(%s)", deref(v), f(x), stub)
	case *types.Array:
		return fmt.Sprintf("%s(%s,%s)", f(x), stub, v)
	case *types.Slice:
		return fmt.Sprintf("%s = %s(%s)", deref(v), f(x), stub)
	case *types.Map:
		return fmt.Sprintf("%s = %s(%s)", deref(v), f(x), stub)
	case *types.Struct:
		return fmt.Sprintf("%s(%s,%s)", f(x), stub, v)
	case *types.Named:
		if g.tSet.isProto(x) {
			return fmt.Sprintf("%s.UnmarshalProto(%s)", stub, v)
		}
		if g.tSet.autoMarshals.At(x) != nil || g.tSet.implementsAutoMarshal(x) {
			return fmt.Sprintf("(%s).AkasarUnmarshal(%s)", v, stub)
		}
		under := x.Underlying()
		if _, ok := under.(*types.Struct); ok {
			return fmt.Sprintf("%s(%s,%s)", f(x), stub, v)
		}
		return g.decode(stub, fmt.Sprintf("(*%s)(%s)", g.tSet.genTypeString(x.Underlying()), v), under)
	default:
		panic(fmt.Sprintf("encode: unexpected expression: %v (type %T)", v, t))
	}
}

func (g *generator) generateRegisteredComponents(p printFn) {
	if len(g.components) == 0 {
		return
	}

	g.tSet.importPackage("context", "context")
	p(``)
	p(`func init() {`)

	for _, comp := range g.components {
		name := comp.intfName()
		var b strings.Builder

		// 单个方法的方法metric
		emitMetricInitializer := func(m *types.Func, remote bool) {
			_, _ = fmt.Fprintf(&b, ", %sMetrics: %s(%s{Caller: caller, Component: %q, Method: %q, Remote: %v})",
				notExported(m.Name()),
				g.codegen().qualify("MethodMetricsFor"),
				g.codegen().qualify("MethodLabels"),
				comp.fullIntfName(),
				m.Name(),
				remote,
			)
		}

		// single stub
		//E.g.
		// func(impl any, caller string, tracer trace.Tracer) any {
		// 		return foo_local_stub{impl: impl.(Foo), tracer: tracer, ...}
		// }
		b.Reset()
		for _, m := range comp.methods() {
			emitMetricInitializer(m, false)
		}
		localStubFn := fmt.Sprintf(`func(impl any, caller string, tracer %v) any {
			return %sLocalStub{impl: impl.(%s), tracer: tracer%s}
		}`,
			g.trace().qualify("Tracer"),
			notExported(name),
			g.componentRef(comp),
			b.String(),
		)

		//client stub
		//E.g.
		// func(stub *codegen.Stub, caller string, tracer trace.Tracer) any {
		// 		return Foo_stub{stub, stub, ...}
		// }
		b.Reset()
		for _, m := range comp.methods() {
			emitMetricInitializer(m, true)
		}
		clientStubFn := fmt.Sprintf(`func(stub %s, caller string, tracer %v) any {
 			return %sClientStub{stub: stub%s}
		}`,
			g.codegen().qualify("Stub"),
			g.trace().qualify("Tracer"),
			notExported(name),
			b.String(),
		)

		// server stub
		// E.g
		// func(impl any) codegen.Server {
		//     return foo_server_stub{impl: impl.(Foo)}
		// }
		serverStubFn := fmt.Sprintf(`func(impl any) %s {
  			return %sServerStub{impl: impl.(%s)}
		}`,
			g.codegen().qualify("Server"),
			notExported(name),
			g.componentRef(comp),
		)

		reflectImport := g.tSet.importPackage("reflect", "reflect")
		//contextImport := g.tSet.importPackage("context", "context")

		//E.g.
		// codegen.Register(codegen.Registration{
		//		...
		// })
		p(`	%s(%s{`,
			g.codegen().qualify("Register"),
			g.codegen().qualify("Registration"),
		)
		p(`		Name: %q,`, comp.fullIntfName())
		p(`		Iface: %s((*%s)(nil)).Elem(),`,
			reflectImport.qualify("TypeOf"),
			g.componentRef(comp),
		)
		p(`		Impl: %s(%s{}),`,
			reflectImport.qualify("TypeOf"),
			comp.implName(),
		)
		if comp.router != nil {
			p(`		Routed: true,`)
		}
		if len(comp.listeners) > 0 {
			listeners := make([]string, len(comp.listeners))
			for i, lis := range comp.listeners {
				listeners[i] = fmt.Sprintf("%q", lis)
			}
			p(`		Listeners: []string{%s},`, strings.Join(listeners, ","))
		}
		if len(comp.noRetry) > 0 {
			p(`		NoRetry: []int{%s},`, noRetryString(comp))
		}
		p(`		LocalStubFn: %s,`, localStubFn)
		p(`		ClientStubFn: %s,`, clientStubFn)
		p(`		ServerStubFn: %s,`, serverStubFn)
		p(`	})`)
	}

	p(`}`)

}

// generateInstanceChecks 检查实例是否符合有效组件的规则
func (g *generator) generateInstanceChecks(p printFn) {
	p(``)
	p(`// akasar.InstanceOf checks.`)

	for _, c := range g.components {
		p(`var _ %s[%s] = (*%s)(nil)`,
			g.akasa().qualify("InstanceOf"),
			g.componentRef(c),
			c.implName(),
		)
	}

}

func Generate(dir string, pkgs []string) error {
	fSet := token.NewFileSet()

	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedSyntax | packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo,
		Context:    nil,
		Logf:       nil,
		Dir:        dir,
		Env:        nil,
		BuildFlags: []string{"--tags=ignoreAkasarGen"},
		Fset:       fSet,
		ParseFile:  parseNowAkasarGenFile,
		Tests:      false,
		Overlay:    nil,
	}

	pkgList, err := packages.Load(cfg, pkgs...)
	if err != nil {
		return fmt.Errorf("packages.load: %w", err)
	}

	var errs []error

	var autoMarshals typeutil.Map
	for _, pkg := range pkgList {
		g, err := newGenerator(pkg, fSet, &autoMarshals)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := g.generate(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (g *generator) args(sig *types.Signature) string {
	var args strings.Builder
	for i := 1; i < sig.Params().Len(); i++ {
		at := sig.Params().At(i).Type()
		if !sig.Variadic() || i != sig.Params().Len()-1 {
			_, _ = fmt.Fprintf(&args, ", a%d %s", i-1, g.tSet.genTypeString(at))
			continue
		}

		subType := at.(*types.Slice).Elem()
		_, _ = fmt.Fprintf(&args, ", a%d ...%s", i-1, g.tSet.genTypeString(subType))

	}

	return fmt.Sprintf("ctx context.Context%s", args.String())
}

func (g *generator) returns(sig *types.Signature) string {
	var returns strings.Builder

	for i := 0; i < sig.Results().Len()-1; i++ {
		rt := sig.Results().At(i).Type()
		_, _ = fmt.Fprintf(&returns, "r%d %s, ", i, g.tSet.genTypeString(rt))
	}

	return fmt.Sprintf("%serr error", returns.String())
}

func (g *generator) preAllocatable(t types.Type) bool {
	return g.tSet.isMeasurable(t) && g.isAkasarEncoded(t)
}

func (g *generator) isAkasarEncoded(t types.Type) bool {
	if g.tSet.isProto(t) {
		return false
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
			panic(fmt.Sprintf("isAkasarEncoded:unexpected type %v", t))
		}
	case *types.Pointer:
		return g.isAkasarEncoded(x.Elem())
	case *types.Array:
		return g.isAkasarEncoded(x.Elem())
	case *types.Slice:
		return g.isAkasarEncoded(x.Elem())
	case *types.Map:
		return g.isAkasarEncoded(x.Key()) && g.isAkasarEncoded(x.Elem())
	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			f := x.Field(i)
			if !g.isAkasarEncoded(f.Type()) {
				return false
			}
		}
		return true
	case *types.Named:
		if s, ok := x.Underlying().(*types.Struct); ok {
			for i := 0; i < s.NumFields(); i++ {
				f := s.Field(i)
				if !g.isAkasarEncoded(f.Type()) {
					return false
				}
			}
			return true
		}
		return g.isAkasarEncoded(x.Underlying())
	default:
		panic(fmt.Sprintf("isAkasarEncoded:unexpected type %v", t))
	}
}

// findSizeFuncNeeded finds any nested types within the provided type that
// require a akasar generated size function. (Pointer)
func (g *generator) findSizeFuncNeeded(t types.Type) {
	var f func(t types.Type)
	f = func(t types.Type) {
		switch x := t.(type) {
		case *types.Pointer:
			g.sizeFuncNeeded.Set(t, true)
			f(x.Elem())
		case *types.Slice:
			f(x.Elem())
		case *types.Array:
			f(x.Elem())
		case *types.Map:
			f(x.Key())
			f(x.Elem())
		case *types.Struct:
			g.sizeFuncNeeded.Set(t, true)
			for i := 0; i < x.NumFields(); i++ {
				f(x.Field(i).Type())
			}
		case *types.Named:
			if s, ok := x.Underlying().(*types.Struct); ok {
				g.sizeFuncNeeded.Set(t, true)
				for i := 0; i < s.NumFields(); i++ {
					f(s.Field(i).Type())
				}
			} else {
				f(x.Underlying())
			}
		}
	}
	f(t)
}

// size
// REQUIRES: t is serializable, measurable, serviceweaver-encoded.
func (g *generator) size(e string, t types.Type) string {
	g.findSizeFuncNeeded(t)

	// size(e: basic type t) = fixedsize(t)
	// size(e: string) = 4 + len(e)
	// size(e: *t) = serviceweaver_size_ptr_t(e)
	// size(e: [N]t) = 4 + len(e) * fixedsize(t)
	// size(e: []t) = 4 + len(e) * fixedsize(t)
	// size(e: map[k]v) = 4 + len(e) * (fixedsize(k) + fixedsize(v))
	// size(e: struct{...}) = serviceweaver_size_struct_XXXXXXXX(e)
	// size(e: weaver.AutoMarshal) = 0
	// size(e: type t struct{...}) = serviceweaver_size_t(e)
	// size(e: type t u) = size(e: u)

	var f func(e string, t types.Type) string
	f = func(e string, t types.Type) string {
		switch x := t.(type) {
		case *types.Basic:
			switch x.Kind() {
			case types.Bool,
				types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
				types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
				types.Float32, types.Float64,
				types.Complex64, types.Complex128:
				return strconv.Itoa(g.tSet.sizeOfType(t))
			case types.String:
				return fmt.Sprintf("(4+len(%s))", e)
			default:
				panic(fmt.Sprintf("size: unexpected expression %v", e))
			}
		case *types.Pointer:
			return fmt.Sprintf("serviceAkasarSize_%s(%s)", sanitize(t), e)
		case *types.Array:
			return fmt.Sprintf("(4 + (len(%s) * %d))", e, g.tSet.sizeOfType(x.Elem()))
		case *types.Slice:
			return fmt.Sprintf("(4 + (len(%s) + %d))", e, g.tSet.sizeOfType(x.Elem()))
		case *types.Map:
			keySize := g.tSet.sizeOfType(x.Key())
			elemSize := g.tSet.sizeOfType(x.Elem())
			return fmt.Sprintf("(4 + (len(%s) * (%d + %d)))", e, keySize, elemSize)
		case *types.Struct:
			return fmt.Sprintf("serviceAkasarSize_%s(&%s)", sanitize(t), e)
		case *types.Named:
			if _, ok := x.Underlying().(*types.Struct); ok {
				return fmt.Sprintf("serviceAkasarSize_%s(&%s)", sanitize(t), e)
			}
			return f(e, x.Underlying())
		default:
			panic(fmt.Sprintf("size: unexpected expression %v", e))
		}
	}
	return f(e, t)
}

func (g *generator) generateLocalStubs(p printFn) {
	p(``)
	p(``)
	p(`// Local stub implementations.`)

	var b strings.Builder
	for _, com := range g.components {
		stub := notExported(com.intfName()) + "LocalStub"
		p(``)
		p(`type %s struct {`, stub)
		p(`	impl %s`, g.componentRef(com))
		p(`	tracer %s`, g.trace().qualify("Tracer"))
		for _, m := range com.methods() {
			p(`	%sMetrics *%s`, notExported(m.Name()), g.codegen().qualify("MethodMetrics"))
		}
		p(`}`)

		p(``)
		p(`// Check that %s implements the %s interface`, stub, g.tSet.genTypeString(com.intf))
		p(`var _ %s = (*%s)(nil)`, g.tSet.genTypeString(com.intf), stub)
		p(``)

		for _, m := range com.methods() {
			signature := m.Type().(*types.Signature)
			p(``)
			p(`func (s %s) %s(%s) (%s) {`, stub, m.Name(), g.args(signature), g.returns(signature))

			p(`	// Update metrics.`)
			p(`	begin := s.%sMetrics.Begin()`, notExported(m.Name()))
			p(`	defer func() { s.%sMetrics.End(begin, err != nil, 0, 0) }()`, notExported(m.Name()))

			// Create a child span if tracing is enabled is ctx.
			p(`	span := %s(ctx)`, g.trace().qualify("SpanFromContext"))
			p(`	if span.SpanContext().IsValid() {`)
			p(`		// Create a child span for this method.`)
			p(`		ctx, span = s.tracer.Start(ctx, "%s.%s.%s", trace.WithSpanKind((trace.SpanKindInternal)))`,
				g.pkg.Name, com.intfName(), m.Name())
			p(`		defer func() {`)
			p(`			if err != nil {`)
			p(`				span.RecordError(err)`)
			p(`				span.SetStatus(%s, err.Error())`, g.codes().qualify("Error"))
			p(`			}`)
			p(`			span.End()`)
			p(`		}()`)
			p(`	}`)

			p(`	defer func() {`)
			p(`		if err == nil {`)
			p(`			err = %s(recover())`, g.codegen().qualify("CatchResultUnwrapPanic"))
			p(`		}`)
			p(`	}()`)

			b.Reset()
			//call args
			_, _ = fmt.Fprintf(&b, "ctx")
			for i := 1; i < signature.Params().Len(); i++ {
				if signature.Variadic() && i == signature.Params().Len()-1 {
					_, _ = fmt.Fprintf(&b, ", a%d...", i-1)
				} else {
					_, _ = fmt.Fprintf(&b, ", a%d", i-1)
				}
			}
			argList := b.String()
			p(``)

			b.Reset()
			for i := 0; i < signature.Results().Len()-1; i++ {
				_, _ = fmt.Fprintf(&b, "r%d, ", i)
			}
			_, _ = fmt.Fprintf(&b, "err")
			resultList := b.String()

			p(`	%s = s.impl.%s(%s)`, resultList, m.Name(), argList)
			p(`	return`)
			p(`}`)
		}
	}

}

func (g *generator) generateClientStubs(p printFn) {
	p(``)
	p(``)
	p(`// Client stub implementations.`)

	var b strings.Builder
	for _, com := range g.components {
		stub := notExported(com.intfName()) + "ClientStub"
		p(``)
		p(`type %s struct {`, stub)
		p(`	stub %s`, g.codegen().qualify("Stub"))
		p(`	tracer %s`, g.trace().qualify("Tracer"))
		for _, m := range com.methods() {
			p(`	%sMetrics *%s`, notExported(m.Name()), g.codegen().qualify("MethodMetrics"))
		}
		p(`}`)

		p(``)
		p(`// Check that %s implements the %s interface`, stub, g.tSet.genTypeString(com.intf))
		p(`var _ %s = (*%s)(nil)`, g.tSet.genTypeString(com.intf), stub)
		p(``)

		mList := make([]string, len(com.methods()))
		for i, m := range com.methods() {
			mList[i] = m.Name()
		}
		slices.Sort(mList)

		methodIndex := make(map[string]int, len(mList))
		for i, m := range mList {
			methodIndex[m] = i
		}

		for _, m := range com.methods() {
			sig := m.Type().(*types.Signature)
			p(``)
			p(`func (s %s) %s(%s) (%s) {`, stub, m.Name(), g.args(sig), g.returns(sig))

			p(`	// Update metrics.`)
			p(`	var requestBytes, replyBytes int`)
			p(`	begin := s.%sMetrics.Begin()`, notExported(m.Name()))
			p(`	defer func() { s.%sMetrics.End(begin, err != nil, requestBytes, replyBytes) }()`, notExported(m.Name()))
			p(``)

			// Create a child span if tracing is enabled is ctx.
			p(`	span := %s(ctx)`, g.trace().qualify("SpanFromContext"))
			p(` 	if span.SpanContext().IsValid() {`)
			p(`		ctx, span = s.tracer.Start(ctx, "%s.%s.%s", trace.WithSpanKind((trace.SpanKindInternal)))`,
				g.pkg.Name, com.intfName(), m.Name())
			p(`	}`)

			//Handle cleanup
			p(``)
			p(`	defer func() {`)
			p(`		// Catch and return any panics detected during encoding/decoding/rpc`)
			p(`		if err == nil {`)
			p(`			err = %s(recover())`, g.codegen().qualify("CatchPanics"))
			p(`			if err != nil {`)
			p(`				err = %s(%s, err)`, g.errorsPackage().qualify("Join"), g.akasa().qualify("RemoteCallError"))
			p(`			}`)
			p(`		}`)
			p(``)
			p(`		if err != nil {`)
			p(`			span.RecordError(err)`)
			p(`			span.SetStatus(%s, err.Error())`, g.codes().qualify("Error"))
			p(`		}`)
			p(`		span.End()`)
			p(``)
			p(`	}()`)

			preAllocated := false
			if sig.Params().Len() > 1 {
				canPreAllocate := true
				for i := 1; i < sig.Params().Len(); i++ {
					if !g.preAllocatable(sig.Params().At(i).Type()) {
						canPreAllocate = false
						break
					}
				}
				if canPreAllocate {
					p(``)
					p("	// Preallocate a buffer of the right size.")
					p(`	size := 0`)
					for i := 1; i < sig.Params().Len(); i++ {
						at := sig.Params().At(i).Type()
						p(`	size += %s`, g.size(fmt.Sprintf("a%d", i-1), at))
					}

					p("	enc := %s", g.codegen().qualify("NewSerializer(size)"))
					preAllocated = true
				}
			}

			b.Reset()
			if sig.Params().Len() > 1 {
				p(``)
				p(`	// Encode arguments.`)
				if !preAllocated {
					p(`	enc := %s`, g.codegen().qualify("NewSerializer()"))
				}
			}
			for i := 1; i < sig.Params().Len(); i++ {
				at := sig.Params().At(i).Type()
				arg := fmt.Sprintf("a%d", i-1)
				p(`	%s`, g.encode("enc", arg, at))
			}

			// Set the routing key, if there is one

			p(`	var shardKey uint64`)

			// Invoke call.Run,
			p(``)
			p(`	// Call the remote method.`)
			data := "nil"
			if sig.Params().Len() > 1 {
				p(`	data := enc.Data()`)
				p(`	requestBytes = len(data)`)
			}
			p(`	var results []byte`)
			p(`	results, err = s.stub.Invoke(ctx, %d, %s, shardKey)`, methodIndex[m.Name()], data)
			p(`	replyBytes = len(results)`)
			p(`	if err != nil {`)
			p(`		err = %s(%s, err)`, g.errorsPackage().qualify("Join"), g.akasa().qualify("RemoteCallError"))
			p(`		return`)
			p(`	}`)

			// Invoke call.Decode.
			b.Reset()
			p(``)
			p(`	// Decode the results.`)
			p(`	dec := %s(results)`, g.codegen().qualify("NewDeserializer"))
			for i := 0; i < sig.Results().Len()-1; i++ {
				rt := sig.Results().At(i).Type()
				res := fmt.Sprintf("r%d", i)
				if x, ok := rt.(*types.Pointer); ok && g.tSet.isProto(x) {
					tmp := fmt.Sprintf("tmp%d", i)
					p(`	var %s %s`, tmp, g.tSet.genTypeString(x.Elem()))
					p(`	%s`, g.decode("dec", ref(tmp), x.Elem()))
					p(`	%s = %s`, res, ref(tmp))
				} else {
					p(`	%s`, g.decode("dec", ref(res), rt))
				}
			}

			p(`	err = dec.Error()`)
			p(``)
			p(`	return`)
			p(`}`)
		}
	}
}

func (g *generator) generateSeverStubs(p printFn) {
	p(``)
	p(``)
	p(`// Server stub implementation.`)

	var b strings.Builder

	for _, com := range g.components {
		stub := fmt.Sprintf("%sServerStub", notExported(com.intfName()))
		p(``)
		p(`type %s struct {`, stub)
		p(`	impl %s`, g.componentRef(com))
		p(`}`)
		p(``)

		p(`// Check that %s is implements the %s interface.`, stub, g.codegen().qualify("Server"))
		p(`var _ %s = (*%s)(nil)`, g.codegen().qualify("Server"), stub)
		p(``)

		p(`// GetHandleFn implements the codegen.Server interface.`)
		p(`func (s %s) GetHandleFn(method string)  func(ctx context.Context, args []byte) ([]byte, error) {`, stub)
		p(`	switch method {`)
		for _, m := range com.methods() {
			p(`	case "%s":`, m.Name())
			p(`		return s.%s`, notExported(m.Name()))
		}
		p(`	default:`)
		p(`		return nil`)
		p(`	}`)
		p(`}`)

		for _, m := range com.methods() {
			sig := m.Type().(*types.Signature)

			p(``)
			p(`func (s *%s) %s(ctx context.Context, args []byte) (res []byte, err error) {`, stub, notExported(m.Name()))
			p(`	defer func() {`)
			p(`		if err == nil {`)
			p(`			err = %s(recover())`, g.codegen().qualify("CatchPanics"))
			p(`		}`)
			p(`	}()`)

			if sig.Params().Len() > 1 {
				p(``)
				p(`	// Encode arguments.`)
				p(`	dec := %s(args)`, g.codegen().qualify("NewDeserializer"))
			}

			b.Reset()
			for i := 1; i < sig.Params().Len(); i++ {
				at := sig.Params().At(i).Type()
				arg := fmt.Sprintf("a%d", i-1)
				if x, ok := at.(*types.Pointer); ok && g.tSet.isProto(x) {
					tmp := fmt.Sprintf("tmp%d", i)
					p(`	var %s %s`, tmp, g.tSet.genTypeString(x.Elem()))
					p(`	%s`, g.decode("dec", ref(tmp), x.Elem()))
					p(`	%s = %s`, arg, ref(tmp))
				} else {
					p(`	var %s %s`, arg, g.tSet.genTypeString(at))
					p(`	%s`, g.decode("dec", ref(arg), at))
				}
			}

			b.Reset()
			_, _ = fmt.Fprintf(&b, "ctx")
			for i := 1; i < sig.Params().Len(); i++ {
				if sig.Variadic() && i == sig.Params().Len()-1 {
					_, _ = fmt.Fprintf(&b, ", a%d...", i-1)
				} else {
					_, _ = fmt.Fprintf(&b, ", a%d", i-1)
				}
			}
			argList := b.String()

			b.Reset()
			p(``)
			for i := 0; i < sig.Results().Len()-1; i++ { // skip final error
				if b.Len() == 0 {
					_, _ = fmt.Fprintf(&b, "r%d", i)
				} else {
					_, _ = fmt.Fprintf(&b, ", r%d", i)
				}
			}

			var res string
			if b.Len() == 0 {
				res = "appErr"
			} else {
				res = fmt.Sprintf("%s, appErr", b.String())
			}

			p(`	%s := s.impl.%s(%s)`, res, m.Name(), argList)

			p(``)
			p(`	//Encode the results.`)

			p(`	enc := %s()`, g.codegen().qualify("NewSerializer"))

			b.Reset()
			for i := 0; i < sig.Results().Len()-1; i++ {
				rt := sig.Results().At(i).Type()
				res := fmt.Sprintf("r%d", i)
				p(`	%s`, g.encode("enc", res, rt))
			}
			p(`	enc.Error(appErr)`)
			p(`	return enc.Data(), nil`)

			p(`}`)
		}
	}

}

// generateAutoMarshalMethods generates AkasarMarshal and AkasarUnmarshal methods
// for any types that declares itself as akasar.AutoMarshal.
func (g *generator) generateAutoMarshalMethods(p printFn) {
	if g.tSet.autoMarshalCandidates.Len() > 0 {
		p(``)
		p(`// AutoMarshal implementations.`)
	}

	sorted := g.tSet.autoMarshalCandidates.Keys()
	sort.Slice(sorted, func(i, j int) bool {
		ti, tj := sorted[i], sorted[j]
		return ti.String() < tj.String()
	})

	ts := g.tSet.genTypeString
	for _, t := range sorted {
		var innerTypes []types.Type
		s := t.Underlying().(*types.Struct)

		p(``)
		p(`var _ %s = (*%s)(nil)`, g.codegen().qualify("AutoMarshal"), ts(t))
		p(`type __is_%s[T ~%s] struct{}`, t.(*types.Named).Obj().Name(), ts(s))
		p(`var _ __is_%s[%s]`, t.(*types.Named).Obj().Name(), ts(t))

		fmt := g.tSet.importPackage("fmt", "fmt")
		p(``)
		p(`func(x *%s) AkasarMarshal(enc *%s) {`, ts(t), g.codegen().qualify("Serializer"))
		p(`	if x == nil {`)
		p(`		panic(%s("%s.AkasarMarshal: nil receiver"))`, fmt.qualify("Errorf"), ts(t))
		p(`	}`)
		for i := 0; i < s.NumFields(); i++ {
			fi := s.Field(i)
			if !isAkasarAutoMarshal(fi.Type()) {
				p(`	%s`, g.encode("enc", "x."+fi.Name(), fi.Type()))
				innerTypes = append(innerTypes, fi.Type())
			}
		}
		p(`}`)

		// Generate AkasarUnmarsal method.
		p(``)
		p(`func (x *%s) AkasarUnmarshal(dec *%s) {`, ts(t), g.codegen().qualify("Deserializer"))
		p(`	if x == nil {`)
		p(`		panic(%s("%s.AkasarUnmarshal: nil receiver"))`, fmt.qualify("Errorf"), ts(t))
		p(`	}`)
		for i := 0; i < s.NumFields(); i++ {
			fi := s.Field(i)
			if !isAkasarAutoMarshal(fi.Type()) {
				p(`	%s`, g.decode("dec", "&x."+fi.Name(), fi.Type()))
			}
		}
		p(`}`)

		// Generate encoding/decoding methods for any inner types.
		for _, inner := range innerTypes {
			g.generateEncDecMethodFor(p, inner)
		}

		if g.tSet.implementsError(t) {
			p(`func init() { %s[*%s]() }`, g.codegen().qualify("RegisterSerializable"), ts(t))
		}
	}
}

func (g *generator) generateEncDecMethodFor(p printFn, t types.Type) {
	if g.generated.At(t) != nil {
		return
	}
	g.generated.Set(t, true)

	ts := g.tSet.genTypeString

	switch x := t.(type) {
	case *types.Basic:
	// 基础类型不需要
	case *types.Pointer:
		if g.tSet.isProto(x) {
			return
		}
		g.generateEncDecMethodFor(p, x.Elem())

		p(``)
		p(`func serviceAkasarEnc_%s(enc *%s, arg %s) {`, sanitize(x), g.codegen().qualify("Serializer"), ts(x))
		p(`	if arg == nil {`)
		p(`		enc.Bool(false)`)
		p(`	} else {`)
		p(`		enc.Bool(true)`)
		p(`		%s`, g.encode("enc", "*arg", x.Elem()))
		p(`	}`)
		p(`}`)

		p(``)
		p(`func serviceAkasarDec_%s(dec *%s) %s {`, sanitize(x), g.codegen().qualify("Deserializer"), ts(x))
		p(`	if !dec.Bool() {`)
		p(`		return nil`)
		p(`	}`)
		p(`	var res %s`, ts(x.Elem()))
		p(`	%s`, g.decode("dec", "&res", x.Elem()))
		p(`	return &res`)
		p(`}`)

	case *types.Array:
		g.generateEncDecMethodFor(p, x.Elem())

		p(``)
		p(`func serviceAkasarEnc_%s(enc *%s, arg *%s) {`, sanitize(x), g.codegen().qualify("Serializer"), ts(x))
		p(`	for i := 0; i < %d; i++ {`, x.Len())
		p(`		%s`, g.encode("enc", "arg[i]", x.Elem()))
		p(`	`)
		p(`}`)

		p(``)
		p(`func serviceAkasarDec_%s(enc *%s, res *%s) {`, sanitize(x), g.codegen().qualify("Deserializer"), ts(x))
		p(`	for i := 0; i < %d; i++ {`, x.Len())
		p(`		%s`, g.decode("dec", "&res[i]", x.Elem()))
		p(`	`)
		p(`}`)

	case *types.Slice:
		g.generateEncDecMethodFor(p, x.Elem())

		p(``)
		p(`func serviceAkasarEnc_%s(enc *%s, arg %s) {`, sanitize(x), g.codegen().qualify("Serializer"), ts(x))
		p(`	if arg == nil {`)
		p(`		enc.Len(-1)`)
		p(`		return`)
		p(`	}`)
		p(`	enc.Len(len(arg))`)
		p(`	for i := 0; i < len(arg); i++ {`)
		p(`		%s`, g.encode("enc", "arg[i]", x.Elem()))
		p(`	}`)
		p(`}`)

		p(``)
		p(`func serviceAkasarDec_%s(dec *%s) %s {`, sanitize(x), g.codegen().qualify("Deserializer"), ts(x))
		p(`	n := dec.Len()`)
		p(`	if n == -1 {`)
		p(`		return nil`)
		p(`	}`)
		p(`	res := make(%s, n)`, ts(x))
		p(`	for i := 0; i < n; i++ {`)
		p(`		%s`, g.decode("dec", "&res[i]", x.Elem()))
		p(`	}`)
		p(`	return res`)
		p(`}`)
	case *types.Map:
		g.generateEncDecMethodFor(p, x.Key())
		g.generateEncDecMethodFor(p, x.Elem())

		p(``)
		p(`func serviceAkasarEnc_%s(enc *%s, arg %s) {`, sanitize(x), g.codegen().qualify("Serializer"), ts(x))
		p(`	if arg == nil {`)
		p(`		enc.Len(-1)`)
		p(`		return`)
		p(`	}`)
		p(`	enc.Len(len(arg)`)
		p(`	for k, v := range arg {`)
		p(`		%s`, g.encode("enc", "k", x.Key()))
		p(`		%s`, g.encode("enc", "v", x.Elem()))
		p(`	}`)
		p(`}`)

		p(``)
		p(`func serviceAkasarDec_%s(dec *%s) %s {`, sanitize(x), g.codegen().qualify("Deserializer"), ts(x))
		p(`	n := dec.Len()`)
		p(`	if n == -1 {`)
		p(`		return nil`)
		p(`	}`)
		p(`	res := make(%s, n)`, ts(x))
		p(`	var k %s`, ts(x.Key()))
		p(`	var v %s`, ts(x.Elem()))
		p(`	for i := 0; i < n; i++ {`)
		p(`		%s`, g.decode("dec", "&k", x.Key()))
		p(`		%s`, g.decode("dec", "&v", x.Elem()))
		p(`		res[k] = v`)
		p(`	}`)
		p(`}`)
	case *types.Struct:
		panic(fmt.Sprintf("generateEncDecFor: unexpected type: %v", t))
	case *types.Named:
		if g.tSet.isProto(x) || g.tSet.autoMarshals.At(x) != nil || g.tSet.implementsAutoMarshal(x) {
			return
		}
		g.generateEncDecMethodFor(p, x.Underlying())
	default:
		panic(fmt.Sprintf("generateEncDecFor: unexpected type: %v", t))
	}
}

func (g *generator) generateEncDecMethods(p printFn) {
	//printedHeader := false
	printer := func(format string, args ...any) {
		p(format, args...)
	}

	for _, com := range g.components {
		for _, method := range com.methods() {
			sig := method.Type().(*types.Signature)

			for j := 1; j < sig.Params().Len(); j++ {
				g.generateEncDecMethodFor(printer, sig.Params().At(j).Type())
			}

			for j := 0; j < sig.Results().Len()-1; j++ {
				g.generateEncDecMethodFor(printer, sig.Results().At(j).Type())
			}
		}
	}
}

func (g *generator) generateSizeFunction(p printFn, t types.Type) {
	switch x := t.(type) {
	case *types.Pointer:
		p(`func serviceAkasarSize_%s(x %s) int {`, sanitize(t), g.tSet.genTypeString(t))
		p(`	if x == nil {`)
		p(`		return 1`)
		p(`	} else {`)
		p("		return  1 + %s", g.size("*x", x.Elem()))
		p(`	}`)
		p(`}`)
	case *types.Struct:
		p(`func serviceAkasarSize_%s(x *%s) int {`, sanitize(t), g.tSet.genTypeString(t))
		p(`	size := 0`)
		for i := 0; i < x.NumFields(); i++ {
			f := x.Field(i)
			p(`	size += %s`, g.size(fmt.Sprintf("x.%s", f.Name()), f.Type()))
		}
		p(`	return size`)
		p("}")
	case *types.Named:
		s := x.Underlying().(*types.Struct)
		p(`func serviceAkasarSize_%s(x *%s) int {`, sanitize(t), g.tSet.genTypeString(t))
		p(`	size := 0`)
		for i := 0; i < s.NumFields(); i++ {
			f := s.Field(i)
			p(`	size += %s`, g.size(fmt.Sprintf("x.%s", f.Name()), f.Type()))
		}
		p(`	return size`)
		p("}")
	default:
		panic(fmt.Sprintf("generateSizeFunction: unexpected type: %v", t))
	}
}

func (g *generator) generateImports(p printFn) {
	p(`// Code generated by "akasar generate". DO NOT EDIT.`)
	p(`//go:build !ignoreAkasarGen`)
	p(``)
	p(`package %s`, g.pkg.Name)
	p(``)
	p(`import (`)
	for _, imp := range g.tSet.imports() {
		switch {
		case imp.local:
		case imp.alias == "":
			p(`	%s`, strconv.Quote(imp.path))
		default:
			p(`	%s %s`, imp.alias, strconv.Quote(imp.path))
		}
	}
	p(`)`)
}

func (g *generator) pkgDir() string {
	if len(g.pkg.Syntax) == 0 {
		panic(fmt.Errorf("package %v has no source files", g.pkg))
	}

	f := g.pkg.Syntax[0]
	fName := g.fileSet.Position(f.Package).Filename

	return filepath.Dir(fName)
}

func notExported(name string) string {
	if len(name) == 0 {
		return name
	}

	a := []rune(name)

	a[0] = unicode.ToLower(a[0])

	return string(a)
}

func exported(name string) string {
	if len(name) == 0 {
		return name
	}

	a := []rune(name)

	a[0] = unicode.ToUpper(a[0])

	return string(a)
}

// uniqueName returns a unique pretty printed representation of the provided
// type (e.g., "int", "map[int]bool"). The key property is that if u != t, then
// uniqueName(u) != uniqueName(t).
//
// Note that types.TypeString returns a pretty printed representation of a
// string, but it is not guaranteed to be unique. For example, if have `type
// int bool`, then TypeString returns "int" for both the named type int and the
// primitive type int.
func uniqueName(t types.Type) string {
	switch x := t.(type) {
	case *types.Pointer:
		return fmt.Sprintf("*%s", uniqueName(x.Elem()))

	case *types.Slice:
		return fmt.Sprintf("[]%s", uniqueName(x.Elem()))

	case *types.Array:
		return fmt.Sprintf("[%d]%s", x.Len(), uniqueName(x.Elem()))

	case *types.Map:
		keyName := uniqueName(x.Key())
		valName := uniqueName(x.Elem())
		return fmt.Sprintf("map[%s]%s", keyName, valName)

	case *types.Named:
		n := x.TypeArgs().Len()
		if n == 0 {
			// This is a plain type.
			return fmt.Sprintf("Named(%s.%s)", x.Obj().Pkg().Path(), x.Obj().Name())
		}

		// This is an instantiated type.
		base := fmt.Sprintf("Named(%s.%s)", x.Obj().Pkg().Path(), x.Obj().Name())
		parts := make([]string, n)
		for i := 0; i < n; i++ {
			parts[i] = uniqueName(x.TypeArgs().At(i))
		}
		return fmt.Sprintf("%s[%s]", base, strings.Join(parts, ", "))

	case *types.Struct:
		// Two structs are considered equal if they have the same fields with
		// the same names, types, and tags in the same order. See
		// https://go.dev/ref/spec#Type_identity.
		fields := make([]string, x.NumFields())
		var b strings.Builder
		for i := 0; i < x.NumFields(); i++ {
			b.Reset()
			f := x.Field(i)
			if !f.Embedded() {
				_, _ = fmt.Fprintf(&b, "%s ", f.Name())
			}
			b.WriteString(uniqueName(f.Type()))
			if x.Tag(i) != "" {
				_, _ = fmt.Fprintf(&b, " `%s`", x.Tag(i))
			}
			fields[i] = b.String()
		}
		return fmt.Sprintf("struct{%s}", strings.Join(fields, "; "))

	case *types.Basic:
		switch x.Kind() {
		case types.Bool,
			types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Float32, types.Float64,
			types.Complex64, types.Complex128,
			types.String:
			return x.Name()
		}
	}
	// TODO(mwhittaker): What about Struct and Interface literals?
	panic(fmt.Sprintf("unsupported type %v (%T)", t, t))
}

// sanitize generates a (somewhat pretty printed) name for the provided type
// that is a valid go identifier [1]. sanitize also produces unique names. That
// is, if u != t, then sanitize(u) != sanitize(t).
//
// Some examples:
//
//   - map[int]string -> map_int_string_589aebd1
//   - map[int][]X    -> map_int_slice_X_ac498abc
//   - []int          -> slice_int_1048ebf9
//   - [][]string     -> slice_slice_string_13efa8aa
//   - [20]int        -> array_20_int_00ae9a0a
//   - *int           -> ptr_int_916711b2
//
// [1]: https://go.dev/ref/spec#Identifiers
// 根据类型及其嵌套类型生成一个唯一的名字
func sanitize(t types.Type) string {
	var sanitize func(types.Type) string
	sanitize = func(t types.Type) string {
		switch x := t.(type) {
		case *types.Pointer:
			return fmt.Sprintf("ptr_%s", sanitize(x.Elem()))
		case *types.Slice:
			return fmt.Sprintf("slice_%s", sanitize(x.Elem()))
		case *types.Array:
			return fmt.Sprintf("array_%d_%s", x.Len(), sanitize(x.Elem()))
		case *types.Map:
			keyName := sanitize(x.Key())
			valName := sanitize(x.Elem())
			return fmt.Sprintf("map_%s_%s", keyName, valName)
		case *types.Struct:
			return "struct"
		case *types.Named:
			n := x.TypeArgs().Len()
			if n == 0 {
				return x.Obj().Name()
			}

			parts := make([]string, n+1)
			parts[0] = x.Obj().Name()
			for i := 0; i < n; i++ {
				parts[i+1] = sanitize(x.TypeArgs().At(i))
			}
			return strings.Join(parts, "_")
		case *types.Basic:
			switch x.Kind() {
			case types.Bool,
				types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
				types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
				types.Float32, types.Float64,
				types.Complex64, types.Complex128,
				types.String:
				return x.Name()
			}
		}
		panic(fmt.Sprintf("generator: unable to generate named type suffic for type %v\n", t))
	}

	hash := sha256.Sum256([]byte(uniqueName(t)))

	return fmt.Sprintf("%s_%x", sanitize(t), hash[:4])
}

func deref(e string) string {
	if len(e) == 0 {
		return "*"
	}

	if e[0] == '&' {
		return e[1:]
	}

	return "*" + e
}

func ref(e string) string {
	if len(e) == 0 {
		return "&"
	}
	if e[0] == '*' {
		return e[1:]
	}

	return "&" + e
}
