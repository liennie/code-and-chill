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

type SpanExtender struct{}

func (e *SpanExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(&spanParser{}, 1000)),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(util.Prioritized(&spanRenderer{}, 1000)),
	)
}

var kindSpan = ast.NewNodeKind("Span")

type span struct {
	ast.BaseInline
}

func (s *span) Dump(source []byte, level int) {
	ast.DumpHelper(s, source, level, nil, nil)
}

func (s *span) Kind() ast.NodeKind {
	return kindSpan
}

type spanParser struct{}

func (p *spanParser) Trigger() []byte {
	return []byte{'(', ')'}
}

func (p *spanParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 2, defaultSpanDelimiterProcessor)
	if node == nil || node.OriginalLength > 2 {
		return nil
	}

	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

type spanDelimiterProcessor struct {
}

var defaultSpanDelimiterProcessor = &spanDelimiterProcessor{}

func (p *spanDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '(' || b == ')'
}

func (p *spanDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == '(' && closer.Char == ')'
}

func (p *spanDelimiterProcessor) OnMatch(consumes int) ast.Node {
	return &span{}
}

type spanRenderer struct{}

func (r *spanRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindSpan, r.render)
}

func (r *spanRenderer) render(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = writer.WriteString("<span")
		html.RenderAttributes(writer, n, nil)
		_, _ = writer.WriteString(">")
	} else {
		_, _ = writer.WriteString("</span>")
	}
	return ast.WalkContinue, nil
}
