package main

import (
	"GoSeek/gui"
	"net/http"
	_ "net/http/pprof"
)

// TODO:
// Optimizations:
// Readings on My TestData 15GB:
// DocFetcher Readings:
// ----> Max Ram Used : Almost 1000 MB
// ----> Speed        : 7m 48sec
// -> Desired Readings with GoSeek :
// ----> Max Ram Used : Almost 2000 MB
// ----> Speed        : < 4m
// -> 1st Readings
// ----> Max Ram Used : Almost 7500+ MB (freezing) (With TermVectors OFF)
// ----> Speed 		  : < 2m
// -> 2nd Readings
// ----> Max Ram Used : Almost 4500 MB (With TermVectors OFF)
// ----> Speed 		  : 4m 56sec
// -> Curr Reading :
// ----> Max Ram Used : Almost 2200 MB (With TermVectors OFF)
// ----> Speed 		  : 5m 30sec

// JUST SOME DUMMY/BUGGY APP ENTRY POINT FOR NOW

func main() {

	// For profiling and debug
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	app := gui.NewApp()
	app.Run()
}
