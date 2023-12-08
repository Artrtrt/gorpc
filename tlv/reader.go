package tlv

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

type Reader interface {
	Read() (tag uint16, value []byte, err error)
}

type reader struct {
	Reader

	mutex  chan interface{}
	reader io.Reader
}

func (r *reader) Read() (tag uint16, value []byte, err error) {
	var (
		readCount  int
		readBuffer []byte
		readHash   uint32

		length uint16
		hash   uint32
	)

	r.mutex <- true

	defer func() {
		<-r.mutex
	}()

	readBuffer = make([]byte, 4)

	readCount, err = io.ReadFull(r.reader, readBuffer[:])
	if readCount != 4 {
		err = errors.New("read error length")
		return
	}

	if err != nil {
		return
	}

	tag = binary.BigEndian.Uint16(readBuffer[0:2])
	length = binary.BigEndian.Uint16(readBuffer[2:4])

	readBuffer = make([]byte, 4+length)

	readCount, err = io.ReadFull(r.reader, readBuffer[0:4+length])
	if readCount != int(4+length) {
		err = errors.New("read error length")
		return
	}

	if err != nil {
		return
	}

	readHash = crc32.ChecksumIEEE(readBuffer[0:length])
	hash = binary.BigEndian.Uint32(readBuffer[length : 4+length])

	if readHash != hash {
		err = errors.New("uncorrected crc32")
		return
	}

	value = readBuffer[0:length]

	return
}

func NewReader(r io.Reader) Reader {
	return &reader{
		mutex:  make(chan interface{}, 1),
		reader: r,
	}
}
