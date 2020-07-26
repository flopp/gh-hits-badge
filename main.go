package main

import (
	"github.com/jessevdk/go-flags"
	"log"
)

func main() {
	var opts struct {
		DB   string `long:"db" description:"The SQLite DB file to use" value-name:"DB_FILE" default:"./gh-hits-badge.db"`
		Port int    `long:"port" description:"The HTTP port to use" value-name:"NUMBER" default:"8080"`
	}

	parser := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	parser.LongDescription = `Start the gh-hits-badge server`
	_, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	a := App{}
	a.Initialize(opts.DB)
	a.Run(opts.Port)
}
