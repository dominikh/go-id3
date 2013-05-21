package main

import (
	"fmt"
	"honnef.co/go/id3"
	"os"
)

func main() {
	id3.Logging = true
	tags, err := id3.Open(os.Args[1])
	if err != nil {
		panic(err)
	}

	err = tags.Parse()
	if err != nil {
		panic(err)
	}

	tags.Clear() // Remove all existing tags
	tags.SetArtists([]string{"dominikh", "some other güy"})
	fmt.Println(tags.Save())
	// tags.SetTitle("A test file.")
	// tags.SetAlbum("Proud test productions")
	// tags.SetArtist("dominikh")
	// // tags.SetTrack(1)
	// tags.SetRecordingTime(time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC))
	// tags.SetPublisher("Gophers united")
	// tags.SetBPM(200) // utz utz utz
	// fmt.Println(tags.Save())
}
