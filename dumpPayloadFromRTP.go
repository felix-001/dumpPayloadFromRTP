package main

import (
	"errors"
	"flag"
	"log"
)

var (
	ErrCheckInputFile = errors.New("check input file error")
)

type consoleParam struct {
	outputFile string
	inputFile  string
}

func parseConsoleParam() (*consoleParam, error) {
	param := &consoleParam{}
	flag.StringVar(&param.inputFile, "file", "", "input file")
	flag.StringVar(&param.outputFile, "output-file", "./output.mpg", "output mpg file")
	flag.Parse()
	if param.inputFile == "" {
		log.Println("must input file")
		return nil, ErrCheckInputFile
	}
	return param, nil
}

func main() {
	log.SetFlags(log.Lshortfile)
	param, err := parseConsoleParam()
	if err != nil {
		return
	}
	log.Println(param.inputFile)
}
