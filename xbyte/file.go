package xbyte

import (
	"io"
	"os"
)

func FileToByte(filename string) (dst []byte, err error) {
	filedesc, err := os.OpenFile(filename, os.O_RDONLY, 0666)
	if err != nil {
		return
	}

	defer filedesc.Close()

	dst, err = io.ReadAll(filedesc)

	return
}

func ByteToFile(src []byte, filename string) (err error) {
	filedesc, err := os.Create(filename)
	if err != nil {
		return
	}

	defer filedesc.Close()

	_, err = filedesc.Write(src)

	return
}
