package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/exp/maps"

	akasa "github.com/kanengo/akasar"

	"golang.org/x/tools/go/packages"
)

const (
	generatedCodeFile = "akasar_gen.go"
)

type generator struct {
	akasa.Components[int]
	tSet       *typeSet
	fileSet    *token.FileSet
	pkg        *packages.Package
	components []*component
}

type component struct {
	intf      *types.Named
	impl      *types.Named
	router    *types.Named
	isRoot    bool
	refs      []*types.Named
	listeners []string
	noRetry   map[string]string
}

func (c *component) fullIntfName() string {
	return fullName(c.intf)
}

func (c *component) intfName() string {
	return c.intf.Obj().Name()
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

func newGenerator(pkg *packages.Package, fSet *token.FileSet) (*generator, error) {
	//Abort if there were any errors loading the package.
	var errs []error
	for _, err := range pkg.Errors {
		errs = append(errs, err)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	tSet := newTypeSet(pkg)
	//遍历语法树 寻找和处理components
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

	return &generator{
		pkg:        pkg,
		tSet:       tSet,
		fileSet:    fSet,
		components: maps.Values(components),
	}, nil
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

// extraComponent 尝试从提供的TypeSpec提取出component
func extraComponent(pkg *packages.Package, file *ast.File, tSet *typeSet, spec *ast.TypeSpec) (*component, error) {
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
			isRoot := isAkasarRoot(arg)
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
			comp, err := extraComponent(pkg, f, tSet, ts)
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
	}

	return nil
}

// codegen imports and returns the codegen package.
func (g *generator) codegen() importPkg {
	p := fmt.Sprintf("%s/runtime/codegen", akasarPackagePath)

	return g.tSet.importPackage(p, "codegen")
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

func (g *generator) generateRegisteredComponents(p printFn) {
	if len(g.components) == 0 {
		return
	}

	g.tSet.importPackage("context", "context")
	p(``)
	p(`fnc init() {`)

	for _, comp := range g.components {
		name := comp.intfName()
		var b strings.Builder

		// 单个方法的方法metric
		emitMetricInitializer := func(m *types.Func, remote bool) {
			_, _ = fmt.Fprintf(&b, ", %sMetrics: %s(%s{Caller: caller, Component: %q, Method: %q, Remote: %v})",
				m.Name(),
				g.codegen().qualify("MethodMetricsFor"),
				g.codegen().qualify("MethodLabels"),
				comp.fullIntfName(),
				m.Name(),
				remote,
			)
		}

		// local stub
		b.Reset()
		for _, m := range comp.methods() {
			emitMetricInitializer(m, false)
		}
		localStubFn := fmt.Sprintf(`
func(impl any, caller string, tracer %v) any {
 return %s_local_stub{impl: impl.(%s), tracer: tracer%s}
}`,
			g.trace().qualify("Tracer"),
			notExported(name),
			g.componentRef(comp),
			b.String(),
		)

		//client stub
		b.Reset()
		for _, m := range comp.methods() {
			emitMetricInitializer(m, true)
		}
		clientFn := fmt.Sprintf(`
func(stub %s, caller string, tracer %v) any {
 return %s_client_stub{stub: stub%s}
}`,
			g.codegen().qualify("Stub"),
			g.trace().qualify("Tracer"),
			notExported(name),
			b.String(),
		)

		// server stub
		serverStubFn := fmt.Sprintf(`
func(impl any) {
  return %s_server_stub{impl: impl.(%s)}"
}`,
			notExported(name),
			g.componentRef(comp),
		)

		fmt.Println(localStubFn)
		fmt.Println(clientFn)
		fmt.Println(serverStubFn)
	}

	p(`}`)

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

	for _, pkg := range pkgList {
		g, err := newGenerator(pkg, fSet)
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
