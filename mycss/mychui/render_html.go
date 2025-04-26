package mychui

import (
	"context"
	"fmt"
	"html"
	"io"

	"myceliumweb.org/mycelium"
	"myceliumweb.org/mycelium/internal/stores"
	"myceliumweb.org/mycelium/myccanon"
	"myceliumweb.org/mycelium/myccanon/mychtml"
	"myceliumweb.org/mycelium/mycexpr"
	myc "myceliumweb.org/mycelium/mycmem"
	"myceliumweb.org/mycelium/mycss"
)

func RenderLambdaType() myc.Type {
	return myc.NewLambdaType(myccanon.NS_Type, mychtml.HTMLType())
}

func RenderLazy(env myc.Value) *myc.Lazy {
	laz, err := mycexpr.BuildLazy(mychtml.HTMLType(), func(eb mycexpr.EB) *mycexpr.Expr {
		return eb.LetVal(env, func(eb mycexpr.EB) *mycexpr.Expr {
			return eb.Apply(
				eb.AnyValueTo(
					myccanon.NSGetExpr(eb.P(0), "renderHTML"),
					RenderLambdaType(),
				),
				eb.P(0),
			)
		})
	})
	if err != nil {
		panic(err)
	}
	return laz
}

func SetRenderHTML(ctx context.Context, pod *mycss.Pod, htmlNode mychtml.HTML) error {
	store := stores.NewMem(mycelium.Hash, mycelium.MaxSizeBytes)
	htmlVal, err := mychtml.EncodeHTML(ctx, store, htmlNode)
	if err != nil {
		return err
	}
	lam, err := mycexpr.BuildLambda(myccanon.NS_Type, mychtml.HTMLType(), func(eb mycexpr.EB) *mycexpr.Expr {
		return eb.Lit(htmlVal)
	})
	return pod.Put(ctx, store, "renderHTML", lam)
}

func renderHTMLDoc(w io.Writer, node mychtml.HTML) error {
	before := "<!DOCTYPE html>\n<html><body>"
	after := `</body></html>`
	if _, err := w.Write([]byte(before)); err != nil {
		return err
	}
	if err := renderHTMLNode(w, node); err != nil {
		return err
	}
	if _, err := w.Write([]byte(after)); err != nil {
		return err
	}
	return nil
}

func renderHTMLNode(w io.Writer, node mychtml.HTML) error {
	hasChildren, exists := htmlTypes[node.Type]
	if !exists {
		_, err := fmt.Fprintf(w, "<div> UNSUPPORTED TYPE (%v) </div>", node.Type)
		return err
	}
	if _, err := fmt.Fprintf(w, "<%s id=%v>", node.Type, node.GetID()); err != nil {
		return err
	}
	text := html.EscapeString(node.Text)
	if _, err := w.Write([]byte(text)); err != nil {
		return err
	}
	if hasChildren {
		for _, child := range node.Children {
			if err := renderHTMLNode(w, child); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "</%s>", node.Type); err != nil {
		return err
	}
	return nil
}

var htmlTypes = map[string]bool{
	"div": true,
	"p":   true,
	"a":   true,
	"b":   true,

	// cannot contain children
	"area":   false,
	"base":   false,
	"br":     false,
	"col":    false,
	"embed":  false,
	"hr":     false,
	"img":    false,
	"input":  false,
	"link":   false,
	"meta":   false,
	"param":  false,
	"source": false,
	"track":  false,
	"wbr":    false,
}
