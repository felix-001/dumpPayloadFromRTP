package main

import (
	"bytes"
	"dumpPayloadFromRTP/bitreader"
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var (
	ErrCheckInputFile  = errors.New("check input file error")
	ErrCheckOutputFile = errors.New("check output file error")
	ErrCheckRTP        = errors.New("check rtp error")
)

type consoleParam struct {
	outputFile string
	inputFile  string
	verbose    bool
}

func parseConsoleParam() (*consoleParam, error) {
	param := &consoleParam{}
	flag.StringVar(&param.inputFile, "file", "", "input file")
	flag.StringVar(&param.outputFile, "output-file", "./output.mpg", "output mpg file")
	flag.BoolVar(&param.verbose, "verbose", false, "log verbose")
	flag.Parse()
	if param.inputFile == "" {
		log.Println("must input file")
		return nil, ErrCheckInputFile
	}
	return param, nil
}

type RTPDecoder struct {
	param      *consoleParam
	fileBuf    *[]byte
	fileSize   int
	br         bitreader.BitReader
	inputFile  *os.File
	outputFile *os.File
	streamSSRC uint32
	streamPT   uint32
	lastSeqNum uint32
}

func NewRTPDecoder(br bitreader.BitReader, fileBuf *[]byte, fileSize int, param *consoleParam) *RTPDecoder {
	decoder := &RTPDecoder{
		fileBuf:  fileBuf,
		fileSize: fileSize,
		param:    param,
		br:       br,
	}
	return decoder
}

func (decoder *RTPDecoder) openFiles() error {
	var err error
	decoder.inputFile, err = os.OpenFile(decoder.param.inputFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	decoder.outputFile, err = os.OpenFile(decoder.param.outputFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (decoder *RTPDecoder) getPos() int64 {
	pos := decoder.br.Size() - int64(decoder.br.Len())
	return pos
}

type RTP struct {
	// version
	V uint32
	// padding,填充标志, 如果P=1，则在该报文的尾部填充一个或多个额外的八位组，它们不是有效载荷的一部分
	P uint32
	// 如果X=1，则在RTP报头后跟有一个扩展报头。
	X uint32
	// CSRC计数器，占4位，指示CSRC 标识符的个数。
	CC uint32
	// 标记，占1位，不同的有效载荷有不同的含义，对于视频，标记一帧的结束；对于音频，标记会话的开始
	M uint32
	// 有效载荷类型，占7位，用于说明RTP报文中有效载荷的类型，如GSM音频、JPEM图像等,在流媒体中大部分是用来区分音频流和视频流的，这样便于客户端进行解析
	PT uint32
	// 序列号,占16位，用于标识发送者所发送的RTP报文的序列号，每发送一个报文，序列号增1
	seqNum uint32
	// 时间戳(Timestamp)：占32位，时戳反映了该RTP报文的第一个八位组的采样时刻。接收者使用时戳来计算延迟和延迟抖动，并进行同步控制
	timestamp uint32
	// 同步信源(SSRC)标识符：占32位，用于标识同步信源。该标识符是随机选择的，参加同一视频会议的两个同步信源不能有相同的SSRC
	SSRC uint32
	// 特约信源(CSRC)标识符：每个CSRC标识符占32位，可以有0～15个。每个CSRC标识了包含在该RTP报文有效载荷中的所有特约信源。
	CSRC   []uint32
	hdrLen uint32
}

func (decoder *RTPDecoder) decodePkt() *RTP {
	start := decoder.getPos()
	br := decoder.br
	V, _ := br.Read32(2)
	P, _ := br.Read32(1)
	X, _ := br.Read32(1)
	CC, _ := br.Read32(4)
	M, _ := br.Read32(1)
	PT, _ := br.Read32(7)
	seqNum, _ := br.Read32(16)
	timestamp, _ := br.Read32(32)
	SSRC, _ := br.Read32(32)
	for i := 0; i < int(CC); i++ {
		br.Skip(32)
	}
	end := decoder.getPos()
	rtp := &RTP{
		V:         V,
		P:         P,
		X:         X,
		CC:        CC,
		M:         M,
		PT:        PT,
		SSRC:      SSRC,
		seqNum:    seqNum,
		timestamp: timestamp,
		hdrLen:    uint32(end - start),
	}
	return rtp
}

func (decoder *RTPDecoder) skipInvalidBytes(skipLen uint32) error {
	br := decoder.br
	skipBuf := make([]byte, skipLen)
	if _, err := io.ReadAtLeast(br, skipBuf, int(skipLen)); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (decoder *RTPDecoder) isRTPValid(rtp *RTP) bool {
	if rtp.P == 1 {
		log.Println("currently don't support decode P")
		return false
	}
	if rtp.X == 1 {
		log.Println("currently don't support decode X")
		return false
	}
	if decoder.streamSSRC == 0 {
		decoder.streamSSRC = rtp.SSRC
	} else if rtp.SSRC != decoder.streamSSRC {
		log.Println("check SSRC error, old:", decoder.streamSSRC, "current:", rtp.SSRC)
		return false
	}
	if decoder.streamPT == 0 {
		decoder.streamPT = rtp.PT
	} else if rtp.PT != decoder.streamPT {
		log.Println("check PT error, old:", decoder.streamPT, "current:", rtp.PT)
		return false
	}
	if decoder.param.verbose {
		log.Println("ssrc:", rtp.seqNum)
	}
	if decoder.lastSeqNum == 0 {
		decoder.lastSeqNum = rtp.seqNum
	} else if decoder.lastSeqNum+1 != rtp.seqNum {
		log.Println("check seqNum error, last:", decoder.lastSeqNum, "current:", rtp.seqNum)
	} else {
		decoder.lastSeqNum = rtp.seqNum
	}
	return true
}

func (decoder *RTPDecoder) saveRTPPayload(payloadLen uint32) error {
	if decoder.outputFile == nil {
		log.Println("check outputfile err")
		return ErrCheckOutputFile
	}
	br := decoder.br
	payloadData := make([]byte, payloadLen)
	if _, err := io.ReadAtLeast(br, payloadData, int(payloadLen)); err != nil {
		log.Println(err)
		return err
	}
	if _, err := decoder.outputFile.Write(payloadData); err != nil {
		log.Println(err)
		return err
	}
	decoder.outputFile.Sync()
	return nil
}

func (decoder *RTPDecoder) decodePkts() error {
	br := decoder.br
	for decoder.getPos() < int64(decoder.fileSize) {
		rtpLen, err := br.Read32(16)
		if err != nil {
			log.Println(err)
			return err
		}
		rtp := decoder.decodePkt()
		if !decoder.isRTPValid(rtp) {
			decoder.skipInvalidBytes(rtpLen - rtp.hdrLen)
			continue
		}
		if err := decoder.saveRTPPayload(rtpLen - rtp.hdrLen); err != nil {
			return err
		}

	}
	return nil
}

func main() {
	log.SetFlags(log.Lshortfile)
	param, err := parseConsoleParam()
	if err != nil {
		return
	}
	fileBuf, err := ioutil.ReadFile(param.inputFile)
	if err != nil {
		log.Printf("open file: %s error", param.inputFile)
		return
	}
	log.Println(param.inputFile, "file size:", len(fileBuf))
	br := bitreader.NewReader(bytes.NewReader(fileBuf))
	decoder := NewRTPDecoder(br, &fileBuf, len(fileBuf), param)
	if err := decoder.openFiles(); err != nil {
		return
	}
	if err := decoder.decodePkts(); err != nil {
		log.Println(err)
		return
	}
}