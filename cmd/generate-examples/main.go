package main

import (
	"flag"
	"github.com/bogware/terraform-provider-mergeah/internal/provider"
	"log"
)

func main() {
	root := flag.String("root", ".", "Provider directory")
	flag.Parse()
	if err := provider.WriteExamples(*root); err != nil {
		log.Fatal(err)
	}
}
