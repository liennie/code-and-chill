package mdext

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type CheckboxExtender struct{}

func (e *CheckboxExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(&checkboxParser{}, 1)),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(util.Prioritized(&checkboxRenderer{}, 1000)),
	)
}

var kindCheckbox = ast.NewNodeKind("Checkbox")

type checkbox struct {
	ast.BaseInline
	checked bool
}

func (s *checkbox) Dump(source []byte, level int) {
	ast.DumpHelper(s, source, level, nil, nil)
}

func (s *checkbox) Kind() ast.NodeKind {
	return kindCheckbox
}

type checkboxParser struct{}

func (p *checkboxParser) Trigger() []byte {
	return []byte{'['}
}

func (p *checkboxParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	savedLine, savedPosition := block.Position()
	revert := func() ast.Node {
		block.SetPosition(savedLine, savedPosition)
		return nil
	}

	checked := false

	block.Advance(1) // [

	switch block.Peek() {
	case ' ':
		block.Advance(1)

	case 'x', 'X':
		checked = true
		block.Advance(1)

	default:
		return revert()
	}

	switch block.Peek() {
	case ']':
		block.Advance(1)

	default:
		return revert()
	}

	return &checkbox{
		checked: checked,
	}
}

type checkboxRenderer struct{}

func (r *checkboxRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindCheckbox, r.render)
}

func (r *checkboxRenderer) render(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = writer.WriteString("<input type=\"checkbox\"")
		if c, ok := n.(*checkbox); ok && c.checked {
			_, _ = writer.WriteString(" checked")
		}
		html.RenderAttributes(writer, n, nil)
		_, _ = writer.WriteString(">")
	}
	return ast.WalkContinue, nil
}
