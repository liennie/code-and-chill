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

type CodeBlockExtender struct{}

func (e *CodeBlockExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(util.Prioritized(&codeBlockParser{}, 50)),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(util.Prioritized(&codeBlockRenderer{}, 1000)),
	)
}

var kindCodeBlock = ast.NewNodeKind("ExCodeBlock")

type codeBlock struct {
	ast.BaseBlock
}

func (s *codeBlock) Dump(source []byte, level int) {
	ast.DumpHelper(s, source, level, nil, nil)
}

func (s *codeBlock) Kind() ast.NodeKind {
	return kindCodeBlock
}

const codeBlockDelimiter = '`'

type codeBlockParser struct{}

func (b *codeBlockParser) Trigger() []byte {
	return []byte{codeBlockDelimiter}
}

func (b *codeBlockParser) lineHasDelim(line []byte) bool {
	if len(line) < 4 {
		return false
	}
	for i := range 4 {
		if line[i] != codeBlockDelimiter {
			return false
		}
	}
	if len(line) > 4 && line[4] == codeBlockDelimiter {
		return false
	}
	return true
}

func (b *codeBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, _ := reader.PeekLine()
	if !b.lineHasDelim(line) {
		return nil, parser.NoChildren
	}

	node := &codeBlock{}
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

func (b *codeBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, segment := reader.PeekLine()
	if b.lineHasDelim(line) {
		reader.AdvanceToEOL()
		return parser.Close
	}

	node.Lines().Append(segment)
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

func (b *codeBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {}

func (b *codeBlockParser) CanInterruptParagraph() bool {
	return true
}

func (b *codeBlockParser) CanAcceptIndentedLine() bool {
	return false
}

type codeBlockRenderer struct{}

func (r *codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindCodeBlock, r.renderCodeBlock)
}

func (r *codeBlockRenderer) renderCodeBlock(writer util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = writer.WriteString("<pre><code")
		html.RenderAttributes(writer, n, nil)
		_, _ = writer.WriteString(">")
	} else {
		_, _ = writer.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}
