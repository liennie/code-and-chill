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

type CodeSpanExtender struct{}

func (e *CodeSpanExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(&codeSpanParser{}, 50)),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(util.Prioritized(&codeSpanRenderer{}, 1000)),
	)
}

var kindCodeSpan = ast.NewNodeKind("ExCodeSpan")

type codeSpan struct {
	ast.BaseInline
}

func (s *codeSpan) Dump(source []byte, level int) {
	ast.DumpHelper(s, source, level, nil, nil)
}

func (s *codeSpan) Kind() ast.NodeKind {
	return kindCodeSpan
}

type codeSpanParser struct{}

func (p *codeSpanParser) Trigger() []byte {
	return []byte{'`'}
}

func (p *codeSpanParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 1, defaultCodeSpanDelimiterProcessor)
	if node == nil || node.OriginalLength != 1 {
		return nil
	}

	node.CanOpen = true
	node.CanClose = true
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

type codespanDelimiterProcessor struct {
}

var defaultCodeSpanDelimiterProcessor = &codespanDelimiterProcessor{}

func (p *codespanDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '`'
}

func (p *codespanDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == '`' && closer.Char == '`'
}

func (p *codespanDelimiterProcessor) OnMatch(consumes int) ast.Node {
	return &codeSpan{}
}

type codeSpanRenderer struct{}

func (r *codeSpanRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindCodeSpan, r.renderCodeSpan)
}

func (r *codeSpanRenderer) renderCodeSpan(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = writer.WriteString("<code")
		html.RenderAttributes(writer, n, nil)
		_, _ = writer.WriteString(">")
	} else {
		_, _ = writer.WriteString("</code>")
	}
	return ast.WalkContinue, nil
}
