// Package mdext provides a Goldmark extension to parse and apply inline attributes
// to preceding AST nodes using a {key=value} syntax.
package mdext

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type InlineAttrExtender struct{}

func (e *InlineAttrExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(&inlineAttrParser{}, 100)),
		parser.WithASTTransformers(util.Prioritized(&inlineAttrTransformer{}, 100)),
	)
}

var kindAttributes = ast.NewNodeKind("InlineAttributes")

type inlineAttr struct {
	ast.BaseInline
}

func (n *inlineAttr) Dump(source []byte, level int) {
	attrs := n.Attributes()
	list := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		name := util.BytesToReadOnlyString(attr.Name)
		value := util.BytesToReadOnlyString(util.EscapeHTML(attr.Value.([]byte)))
		list[name] = value
	}

	ast.DumpHelper(n, source, level, list, nil)
}

func (n *inlineAttr) Kind() ast.NodeKind {
	return kindAttributes
}

type inlineAttrParser struct{}

func (p *inlineAttrParser) Trigger() []byte {
	return []byte{'{'}
}

func (p *inlineAttrParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	_, start := block.Position()

	attrs, ok := parser.ParseAttributes(block)
	if !ok {
		return nil
	}

	node := &inlineAttr{}
	for _, attr := range attrs {
		node.SetAttribute(attr.Name, attr.Value)
	}

	_, end := block.Position()
	node.AppendChild(node, ast.NewTextSegment(start.WithStop(end.Start)))

	return node
}

type inlineAttrTransformer struct{}

func (t *inlineAttrTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	var attrs []ast.Node
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == kindAttributes {
			attrs = append(attrs, n)
			return ast.WalkSkipChildren, nil
		}

		return ast.WalkContinue, nil
	})

	for _, attr := range attrs {
		prev := attr.PreviousSibling()
		if prev == nil || prev.Type() != ast.TypeInline || prev.Kind() == kindAttributes || prev.Kind() == ast.KindText {
			continue
		}

		for _, attr := range attr.Attributes() {
			prev.SetAttribute(attr.Name, attr.Value)
		}
		attr.Parent().RemoveChild(attr.Parent(), attr)
	}
}
