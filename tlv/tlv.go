package tlv

import "io"

type ReadWriter interface {
	Reader
	Writer
}

type readWriter struct {
	ReadWriter

	reader Reader
	writer Writer
}

func (rw *readWriter) Read() (tag uint16, value []byte, err error) {
	return rw.reader.Read()
}

func (rw *readWriter) Write(tag uint16, value []byte) (err error) {
	return rw.writer.Write(tag, value)
}

func NewReadWriter(rw io.ReadWriter) ReadWriter {
	return &readWriter{
		reader: NewReader(rw),
		writer: NewWriter(rw),
	}
}
