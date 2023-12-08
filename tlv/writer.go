package tlv

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
)

type Writer interface {
	Write(tag uint16, value []byte) (err error)
}

type writer struct {
	Writer

	mutex  chan interface{}
	writer io.Writer
}

func (w *writer) Write(tag uint16, value []byte) (err error) {
	var (
		writeCount  int
		writeBuffer []byte

		length uint16
		hash   uint32
	)

	w.mutex <- true

	defer func() {
		<-w.mutex
	}()

	length = uint16(len(value))
	hash = crc32.ChecksumIEEE(value)

	writeBuffer = make([]byte, 8)

	binary.BigEndian.PutUint16(writeBuffer[0:2], tag)
	binary.BigEndian.PutUint16(writeBuffer[2:4], length)
	binary.BigEndian.PutUint32(writeBuffer[4:8], hash)

	writeCount, err = w.writer.Write(writeBuffer[0:4])
	if writeCount != 4 {
		err = errors.New("no write header")
		return
	}

	writeCount, err = w.writer.Write(value[0:length])
	if writeCount != int(length) {
		err = errors.New("no write payload")
		return
	}

	writeCount, err = w.writer.Write(writeBuffer[4:8])
	if writeCount != 4 {
		err = errors.New("no write crc32")
		return
	}

	return
}

func NewWriter(w io.Writer) Writer {
	return &writer{
		mutex:  make(chan interface{}, 1),
		writer: w,
	}
}
