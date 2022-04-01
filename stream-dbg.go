package main

import (
	"bytes"
	"dumpPayloadFromRTP/bitreader"
	"dumpPayloadFromRTP/rtptool"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"time"
)

var (
	ErrCheckInputFile  = errors.New("check input file error")
	ErrCheckOutputFile = errors.New("check output file error")
)

func parseConsoleParam() (*rtptool.ConsoleParam, error) {
	param := &rtptool.ConsoleParam{}
	flag.StringVar(&param.InputFile, "file", "", "input file")
	flag.StringVar(&param.OutputFile, "output-file", "", "output mpg file")
	flag.StringVar(&param.CsvFile, "csv-file", "", "output csv file")
	flag.StringVar(&param.SearchBytes, "search-bytes", "", "search bytes get rtp info")
	flag.StringVar(&param.RemoteAddr, "remote-addr", "", "remote ip:port")
	flag.BoolVar(&param.ShowProgress, "show-progress", false, "show progress bar")
	flag.BoolVar(&param.Verbose, "Verbose", false, "log Verbose")
	flag.IntVar(&param.SendRtpCount, "send-rtp-count", 100, "发送多少个rtp就不发了")
	flag.BoolVar(&param.DumpOneFrame, "dump-one-frame", false, "从h264文件摘出第一帧")
	flag.Parse()
	if param.InputFile == "" {
		log.Println("must input file")
		return nil, ErrCheckInputFile
	}
	return param, nil
}

func main() {
	log.SetFlags(log.Lshortfile)
	param, err := parseConsoleParam()
	if err != nil {
		flag.PrintDefaults()
		return
	}
	fileBuf, err := ioutil.ReadFile(param.InputFile)
	if err != nil {
		log.Printf("open file: %s error", param.InputFile)
		return
	}
	log.Println(param.InputFile, "file size:", len(fileBuf))
	br := bitreader.NewReader(bytes.NewReader(fileBuf))
	decoder := rtptool.NewRTPDecoder(br, &fileBuf, len(fileBuf), param)
	if decoder == nil {
		return
	}
	if err := decoder.OpenFiles(); err != nil {
		return
	}
	if param.DumpOneFrame {
		decoder.DumpOneFrame()
		decoder.Save()
		return
	}
	if err := decoder.DecodePkts(); err != nil {
		log.Println(err)
		if err == rtptool.ErrSendDone {
			time.Sleep(10 * time.Second)
		}
		return
	}
	decoder.Save()
	decoder.DumpStream()
}
