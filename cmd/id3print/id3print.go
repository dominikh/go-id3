package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"honnef.co/go/id3"
)

func printFile(name string) {
	fmt.Println(name)
	f, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r := bufio.NewReader(f)

	ok, err := id3.Check(r)
	if err != nil {
		fmt.Println(err)
		return
	}

	if !ok {
		log.Println("no ID3 tag")
		return
	}

	tag, err := id3.NewDecoder(r).Parse()
	if err != nil {
		fmt.Println(err)
		return
	}

	for typ, frames := range tag.Frames {
		if typ == "TXXX" {
			for _, frame := range frames {
				frame := frame.(id3.UserTextInformationFrame)
				fmt.Printf("%s: %s\n", frame.Description, frame.Text)
			}
			continue
		}
		var vals []string
		for _, frame := range frames {
			vals = append(vals, frame.Value())
		}
		fmt.Printf("%s: %s\n", typ.String(), strings.Join(vals, ", "))
	}
}

func main() {
	for _, name := range os.Args[1:] {
		printFile(name)
		fmt.Println()
	}
}
