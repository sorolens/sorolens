// Command configdoc generates docs/configuration.md, the reference of every
// configuration option of the API, indexer, CLI, web dashboard and Discord
// bot (issue #846).
//
// Go components are read from source: every call whose function name
// contains "env" (os.Getenv, os.LookupEnv, getEnvDefault, envInt, ...) with a
// string-literal UPPER_SNAKE_CASE name is an option. The default is the
// call's second argument when there is one; the description is the comment on
// the line above the call or at the end of its line, so descriptions live next
// to the code that reads the option. Write "Required." in that comment for an
// option the component cannot start without.
//
// The web dashboard and Discord bot are read from their .env.example files:
// the comment block above each NAME=value line is its description, with
// optional "Required/Optional:" and "Default:" lines.
//
// Usage (from apps/api): go run ./cmd/configdoc -o ../../docs/configuration.md
// The freshness test in this package fails when the committed file is stale.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Option is one configuration setting of one component.
type Option struct {
	Name        string
	Default     string
	Description string
	Required    bool
	Source      string // file that reads or declares it, relative to the repo root
}

// Component is a section of the reference.
type Component struct {
	Title   string
	Kind    string // "go" or "dotenv"
	Path    string // directory (go) or .env.example file (dotenv), repo-relative
	Options []Option
}

func components() []Component {
	return []Component{
		{Title: "API (`apps/api`)", Kind: "go", Path: "apps/api"},
		{Title: "Indexer (`services/indexer`)", Kind: "go", Path: "services/indexer"},
		{Title: "CLI (`cli`)", Kind: "go", Path: "cli"},
		{Title: "Docker Compose and shared settings (`.env.example`)", Kind: "dotenv", Path: ".env.example"},
		{Title: "Web dashboard (`apps/web`)", Kind: "dotenv", Path: "apps/web/.env.example"},
		{Title: "Discord bot (`services/discord-bot`)", Kind: "dotenv", Path: "services/discord-bot/.env.example"},
	}
}

// notConfig lists variables read only to pass through to child processes;
// they are not configuration of the component itself.
var notConfig = map[string]bool{"PATH": true, "HOME": true, "RUSTUP_HOME": true, "CARGO_HOME": true}

var (
	envName      = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,}$`)
	requiredWord = regexp.MustCompile(`\bRequired\b`)
	dotenvLine   = regexp.MustCompile(`^([A-Z][A-Z0-9_]*)=(.*)$`)
	flagFunc     = regexp.MustCompile(`^(?:String|Int|Int64|Uint|Uint64|Bool|Duration|Float64)(?:Var)?(?:P)?$`)
)

// isEnvReader reports whether a function name reads an environment variable.
func isEnvReader(name string) bool {
	lower := strings.ToLower(name)
	if !strings.Contains(lower, "env") {
		return false
	}
	return !strings.Contains(lower, "setenv") && !strings.Contains(lower, "unsetenv")
}

func callName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// commentText joins a comment group into one sentence-like line.
func commentText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	return strings.Join(strings.Fields(g.Text()), " ")
}

type goFile struct {
	fset *token.FileSet
	file *ast.File
	rel  string
}

// fieldDocs maps struct field names to their doc comments, and environment
// variable names to the doc comment of a field that mentions them.
func fieldDocs(files []goFile) (byField, byEnv map[string]string) {
	byField = map[string]string{}
	byEnv = map[string]string{}
	envMention := regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)
	for _, f := range files {
		ast.Inspect(f.file, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				doc := commentText(field.Doc)
				if doc == "" {
					doc = commentText(field.Comment)
				}
				if doc == "" {
					continue
				}
				for _, name := range field.Names {
					if _, dup := byField[name.Name]; !dup {
						byField[name.Name] = doc
					}
				}
				for _, env := range envMention.FindAllString(doc, -1) {
					if _, dup := byEnv[env]; !dup {
						byEnv[env] = doc
					}
				}
			}
			return true
		})
	}
	return byField, byEnv
}

// assignedField returns the struct field an env read is stored into: the key
// of `Field: read`, the selector of `x.Field = read`, or the first
// `x.Field = ...` inside an `if v := read; ... { }` body.
func assignedField(stack []ast.Node) string {
	for i := len(stack) - 2; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.KeyValueExpr:
			if id, ok := n.Key.(*ast.Ident); ok {
				return id.Name
			}
		case *ast.AssignStmt:
			if sel, ok := n.Lhs[0].(*ast.SelectorExpr); ok {
				return sel.Sel.Name
			}
			if i > 0 {
				if ifs, ok := stack[i-1].(*ast.IfStmt); ok && ifs.Init == n {
					return firstFieldAssignment(ifs.Body)
				}
			}
			return ""
		case *ast.FuncDecl, *ast.FuncLit:
			return ""
		}
	}
	return ""
}

// flagUsage returns the usage string of a flag whose default is the env read,
// e.g. flag.String("shard-store", envString("INDEXER_SHARD_STORE", ""), "usage").
func flagUsage(stack []ast.Node, read *ast.CallExpr) string {
	if len(stack) < 2 {
		return ""
	}
	parent, ok := stack[len(stack)-2].(*ast.CallExpr)
	if !ok {
		return ""
	}
	// flag.String(name, default, usage) and the pflag/cobra
	// StringVar(&p, name, default, usage) forms: usage follows the default.
	idx := -1
	for i, a := range parent.Args {
		if a == ast.Expr(read) {
			idx = i
		}
	}
	if idx < 1 || idx+1 >= len(parent.Args) || !flagFunc.MatchString(callName(parent.Fun)) {
		return ""
	}
	lit, ok := parent.Args[idx+1].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	usage, err := strconv.Unquote(lit.Value)
	if err != nil {
		return ""
	}
	return usage
}

// funcDocMentioning returns the doc comment of the enclosing function when it
// names the variable.
func funcDocMentioning(stack []ast.Node, name string) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if fn, ok := stack[i].(*ast.FuncDecl); ok {
			doc := commentText(fn.Doc)
			if strings.Contains(doc, name) {
				return doc
			}
			return ""
		}
	}
	return ""
}

func firstFieldAssignment(body *ast.BlockStmt) string {
	name := ""
	ast.Inspect(body, func(n ast.Node) bool {
		if name != "" {
			return false
		}
		if as, ok := n.(*ast.AssignStmt); ok {
			if sel, ok := as.Lhs[0].(*ast.SelectorExpr); ok {
				name = sel.Sel.Name
			}
		}
		return true
	})
	return name
}

// scanGo collects every environment option read by non-test Go files under dir.
func scanGo(root, dir string) ([]Option, error) {
	var files []goFile
	err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "testdata", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, goFile{fset: fset, file: file, rel: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, err
	}

	docByField, docByEnv := fieldDocs(files)
	required := requiredByCode(files)
	byName := map[string]*Option{}
	var order []string

	for _, f := range files {
		fset := f.fset
		// Comment groups by the line they end on and the line they start on.
		above := map[int]*ast.CommentGroup{}
		trailing := map[int]*ast.CommentGroup{}
		for _, g := range f.file.Comments {
			above[fset.Position(g.End()).Line] = g
			trailing[fset.Position(g.Pos()).Line] = g
		}

		var stack []ast.Node
		ast.Inspect(f.file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !isEnvReader(callName(call.Fun)) {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			name, err := strconv.Unquote(lit.Value)
			if err != nil || !envName.MatchString(name) || notConfig[name] {
				return true
			}

			line := fset.Position(call.Pos()).Line
			desc := commentText(above[line-1])
			if g := trailing[line]; g != nil {
				desc = commentText(g)
			}
			if desc == "" {
				desc = flagUsage(stack, call)
			}
			if desc == "" {
				desc = docByField[assignedField(stack)]
			}
			if desc == "" {
				desc = funcDocMentioning(stack, name)
			}
			if desc == "" {
				desc = docByEnv[name]
			}
			def := ""
			if len(call.Args) > 1 {
				var buf bytes.Buffer
				if err := printer.Fprint(&buf, fset, call.Args[1]); err == nil {
					def = buf.String()
					if s, err := strconv.Unquote(def); err == nil {
						def = s
					}
				}
			}

			opt, seen := byName[name]
			if !seen {
				opt = &Option{Name: name, Source: f.rel}
				byName[name] = opt
				order = append(order, name)
			}
			if opt.Description == "" && desc != "" {
				opt.Description = desc
				opt.Source = f.rel
			}
			if opt.Default == "" && def != "" {
				opt.Default = def
			}
			return true
		})
	}

	opts := make([]Option, 0, len(order))
	for _, name := range order {
		o := *byName[name]
		o.Required = required[o.Name] || requiredWord.MatchString(o.Description)
		opts = append(opts, o)
	}
	return opts, nil
}

// requiredByCode finds variables a component refuses to start without: the
// string literals appended to a slice named like "missing" (the pattern
// config.Load uses to report every absent required variable at once).
func requiredByCode(files []goFile) map[string]bool {
	required := map[string]bool{}
	for _, f := range files {
		ast.Inspect(f.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || callName(call.Fun) != "append" || len(call.Args) < 2 {
				return true
			}
			slice, ok := call.Args[0].(*ast.Ident)
			if !ok || !strings.Contains(strings.ToLower(slice.Name), "missing") {
				return true
			}
			for _, a := range call.Args[1:] {
				if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if name, err := strconv.Unquote(lit.Value); err == nil && envName.MatchString(name) {
						required[name] = true
					}
				}
			}
			return true
		})
	}
	return required
}

// scanDotenv reads options from an .env.example file.
func scanDotenv(root, path string) ([]Option, error) {
	f, err := os.Open(filepath.Join(root, path))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var opts []Option
	var comment []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "":
			comment = nil
			continue
		case strings.HasPrefix(line, "#"):
			text := strings.TrimSpace(strings.TrimLeft(line, "#"))
			// Section rules ("# ==== ..." / "# ---- Title ----") are not descriptions.
			if strings.HasPrefix(text, "===") || strings.HasPrefix(text, "---") {
				comment = nil
				continue
			}
			// "# NAME=value" documents an option that is off by default.
			if dotenvLine.MatchString(text) {
				line = text
				break
			}
			comment = append(comment, text)
			continue
		}
		{
			m := dotenvLine.FindStringSubmatch(line)
			if m == nil {
				comment = nil
				continue
			}
			opt := Option{Name: m[1], Default: strings.Trim(m[2], `"'`), Source: path}
			var desc []string
			for _, c := range comment {
				switch {
				case strings.HasPrefix(c, "Required/Optional:"):
					opt.Required = strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(c, "Required/Optional:")), "Required")
				case strings.HasPrefix(c, "Default:"):
					opt.Default = strings.TrimSpace(strings.TrimPrefix(c, "Default:"))
				default:
					desc = append(desc, c)
				}
			}
			opt.Description = strings.Join(desc, " ")
			if !opt.Required {
				opt.Required = requiredWord.MatchString(opt.Description)
			}
			opts = append(opts, opt)
			// A comment block describes every NAME= line directly below it,
			// until a blank line ends the group.
		}
	}
	return opts, scanner.Err()
}

// Collect reads every component's options from the repository at root.
//
// A Go option without a code comment falls back to its entry in the root
// .env.example, which documents the settings shared by docker-compose.
func Collect(root string) ([]Component, error) {
	shared, err := scanDotenv(root, ".env.example")
	if err != nil {
		return nil, fmt.Errorf(".env.example: %w", err)
	}
	sharedByName := map[string]Option{}
	for _, o := range shared {
		sharedByName[o.Name] = o
	}

	comps := components()
	for i := range comps {
		var err error
		switch comps[i].Kind {
		case "go":
			comps[i].Options, err = scanGo(root, comps[i].Path)
			for j, o := range comps[i].Options {
				if s, ok := sharedByName[o.Name]; ok && o.Description == "" {
					comps[i].Options[j].Description = s.Description
					comps[i].Options[j].Required = comps[i].Options[j].Required || s.Required
				}
			}
		case "dotenv":
			comps[i].Options, err = scanDotenv(root, comps[i].Path)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", comps[i].Path, err)
		}
		sort.Slice(comps[i].Options, func(a, b int) bool {
			return comps[i].Options[a].Name < comps[i].Options[b].Name
		})
	}
	return comps, nil
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	if s == "" {
		return "—"
	}
	return s
}

// Render produces the Markdown reference.
func Render(comps []Component) string {
	var b strings.Builder
	b.WriteString("<!-- Code generated by apps/api/cmd/configdoc. DO NOT EDIT. -->\n")
	b.WriteString("<!-- Regenerate with `make config-docs`; CI fails when this file is stale. -->\n\n")
	b.WriteString("# Configuration reference\n\n")
	b.WriteString("Every environment variable each component reads, with its default and\n")
	b.WriteString("description. Descriptions come from the comment next to the code that reads\n")
	b.WriteString("the option (Go components) or from the component's `.env.example`; edit them\n")
	b.WriteString("there, not here.\n\n")
	for _, c := range comps {
		fmt.Fprintf(&b, "## %s\n\n", c.Title)
		b.WriteString("| Name | Default | Required | Description |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, o := range c.Options {
			def := "—"
			if o.Default != "" {
				def = "`" + strings.ReplaceAll(o.Default, "`", "'") + "`"
			}
			req := "no"
			if o.Required {
				req = "yes"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", o.Name, def, req, cell(o.Description))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func main() {
	root := flag.String("root", "../..", "repository root")
	out := flag.String("o", "", "output file (default: stdout)")
	flag.Parse()

	comps, err := Collect(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configdoc:", err)
		os.Exit(1)
	}
	doc := Render(comps)
	if *out == "" {
		fmt.Print(doc)
		return
	}
	if err := os.WriteFile(*out, []byte(doc), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "configdoc:", err)
		os.Exit(1)
	}
}
