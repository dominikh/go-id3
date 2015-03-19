package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

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

	for _, frame := range tag.Frames {
		if frame.ID() == "TXXX" {
			frame := frame.(id3.UserTextInformationFrame)
			fmt.Printf("%s: %s\n", frame.Description, frame.Text)
			continue
		}

		fmt.Printf("%s: %s\n", frame.ID(), frame.Value())
	}
}

func main() {
	for _, name := range os.Args[1:] {
		printFile(name)
		fmt.Println()
	}
}
