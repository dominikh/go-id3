package main

import (
	"fmt"
	"honnef.co/go/id3"
	"os"
)

func main() {
	id3.Logging = true
	file, err := id3.Open(os.Args[1])
	if err != nil {
		panic(err)
	}

	file.Clear() // Remove all existing tags
	file.SetArtists([]string{"me", "you"})
	file.SetAlbum("Yippey")
	fmt.Println(file.Save())
	// tags.SetTitle("A test file.")
	// tags.SetAlbum("Proud test productions")
	// tags.SetArtist("dominikh")
	// // tags.SetTrack(1)
	// tags.SetRecordingTime(time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC))
	// tags.SetPublisher("Gophers united")
	// tags.SetBPM(200) // utz utz utz
	// fmt.Println(tags.Save())
}
