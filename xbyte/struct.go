package xbyte

import (
	"bytes"
	"encoding/binary"
)

func StructToByte(src interface{}) (dst []byte, err error) {
	buf := &bytes.Buffer{}

	err = binary.Write(buf, binary.BigEndian, src)
	if err != nil {
		return
	}

	dst = buf.Bytes()

	return
}

func ByteToStruct(src []byte, dst interface{}) (err error) {
	err = binary.Read(bytes.NewBuffer(src), binary.BigEndian, dst)

	return
}
