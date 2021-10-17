package interp

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// traceExpr prints expressions like a shell would do if its
// options '-o' is set to either 'xtrace' or its shorthand, '-x'.
//
// For some expressions, we explicitly use syntax.Quote instead
// of syntax.Printer type because the latter defaults to
// print expressions in double quotes.
func (r *Runner) traceExpr(cm syntax.Node) {
	if !r.opts[optXTrace] {
		return
	}

	var (
		printer = syntax.NewPrinter()
		b       strings.Builder
	)
	// trace always starts with '+', followed by the expression,
	// some of which are evaluated, some are not
	b.WriteString("+ ")

	switch x := cm.(type) {
	case *syntax.CallExpr:
		if len(x.Args) > 0 {
			funcName := x.Args[0].Lit()
			fields := r.fields(x.Args[1:]...)
			s := strings.Join(fields, " ")

			if strings.TrimSpace(s) == "" {
				// fields may be empty for function () {} declarations
				b.WriteString(funcName)
			} else if funcName == "set" {
				// do not quote the arguments to the builtin 'set'
				b.WriteString(funcName)
				b.WriteByte(' ')
				b.WriteString(s)
			} else {
				qs, err := syntax.Quote(s, syntax.LangBash)
				if err != nil {
					return
				}
				b.WriteString(funcName)
				b.WriteByte(' ')
				b.WriteString(qs)
			}
		}

		for _, v := range x.Assigns {
			if v.Array != nil {
				printer.Print(&b, x)
			} else if v.Value != nil {
				for _, w := range v.Value.Parts {
					switch w.(type) {
					case *syntax.ArithmExp:
						b.WriteString(fmt.Sprintf("%s=%d", v.Name.Value, r.arithm(v.Value)))
					case *syntax.DblQuoted:
						// bash prints double-quoted arguments as single quotes
						qs, err := syntax.Quote(r.literal(v.Value), syntax.LangBash)
						if err != nil {
							return
						}
						b.WriteString(fmt.Sprintf("%s=%s", v.Name.Value, qs))
					case *syntax.SglQuoted, *syntax.Lit:
						printer.Print(&b, x)
					case *syntax.ParamExp, *syntax.ProcSubst, *syntax.ExtGlob, *syntax.BraceExp:
						return // unsupported
					default:
						return
					}
				}
			}
		}
	case *syntax.ForClause:
		switch y := x.Loop.(type) {
		case *syntax.WordIter:
			// when tracing, arguments to the 'for' command is quoted
			// but unquoted for 'select'
			if x.Select {
				b.WriteString("select ")
				for _, item := range y.Items {
					printer.Print(&b, item)
					b.WriteString(" ")
				}
			} else {
				b.WriteString("for ")
				if !y.InPos.IsValid() {
					return
				}

				b.WriteString(y.Name.Value)
				b.WriteString(" in '")
				// wrap items in single quotes
				for _, item := range y.Items {
					printer.Print(&b, item)
				}
				b.WriteString("'")
			}
		case *syntax.CStyleLoop:
			return
		}
	case *syntax.CaseClause:
		b.WriteString("case ")
		printer.Print(&b, x.Word)
		b.WriteString(" in") // don't print the patterns
	case *syntax.LetClause:
		for _, expr := range x.Exprs {
			switch v := expr.(type) {
			case *syntax.Word:
				qs, err := syntax.Quote(r.literal(v), syntax.LangBash)
				if err != nil {
					return
				}
				b.WriteString(fmt.Sprintf("let %v", qs))
			default:
				printer.Print(&b, x)
			}
		}
	case *syntax.IfClause, *syntax.WhileClause, *syntax.ArithmCmd, *syntax.TestClause, *syntax.DeclClause, *syntax.TimeClause:
		return // unsupported
	default:
		return
	}

	// trace always appends a new line after every expression
	b.WriteString("\n")
	r.out(b.String())
}
