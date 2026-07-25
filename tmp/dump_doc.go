//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"implcache-mcp/store"
)

func main() {
	st, err := store.Open("./tmp/esp-idf-test.db")
	if err != nil {
		panic(err)
	}
	defer st.Close()
	uri := "project://esp-idf/projects/esp-idf/en/stable/esp32/get-started/index.html"
	if len(os.Args) > 1 {
		uri = os.Args[1]
	}
	doc, chunks, err := st.GetDocumentByURI(context.Background(), uri)
	if err != nil {
		panic(err)
	}
	fmt.Println("title:", doc.Title, "chunks:", len(chunks))
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString("### " + c.Heading + "\n")
		b.WriteString(c.Body)
		b.WriteString("\n\n")
	}
	s := b.String()
	if len(s) > 14000 {
		s = s[:14000] + "\n…[truncated]…"
	}
	fmt.Println(s)
}
