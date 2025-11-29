package mdext

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type RawStringExtender struct{}

func (e *RawStringExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(&rawStringParser{}, 1)),
	)
}

var kindRawString = ast.NewNodeKind("RawString")

type rawString struct {
	ast.BaseInline
}

func (n *rawString) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func (n *rawString) Kind() ast.NodeKind {
	return kindRawString
}

const rawStringDelimiter = '='

type rawStringParser struct{}

func (p *rawStringParser) Trigger() []byte {
	return []byte{rawStringDelimiter}
}

func (p *rawStringParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	savedLine, savedPosition := block.Position()
	revert := func() ast.Node {
		block.SetPosition(savedLine, savedPosition)
		return nil
	}

	for range 3 {
		if block.Peek() != rawStringDelimiter {
			return revert()
		}
		block.Advance(1)
	}

	_, start := block.Position()
text:
	for {
		switch block.Peek() {
		case text.EOF:
			return revert()

		case '\\':
			block.Advance(2)

		case rawStringDelimiter:
			endLine, end := block.Position()
			for range 3 {
				if block.Peek() != rawStringDelimiter {
					block.SetPosition(endLine, end)
					block.Advance(1)
					continue text
				}
				block.Advance(1)
			}

			node := &rawString{}
			node.AppendChild(node, ast.NewTextSegment(start.WithStop(end.Start)))
			return node

		default:
			block.Advance(1)
		}
	}
}

func RawStringEscape(s string) string {
	return "===" + strings.ReplaceAll(s, "=", "\\=") + "==="
}
