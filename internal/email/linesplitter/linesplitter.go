package linesplitter

import "io"

type splitter struct {
	MaxLineLength int
	Writer        io.Writer
	pos           int
}

func New(w io.Writer, maxLineLength int) io.Writer {
	return &splitter{
		MaxLineLength: maxLineLength,
		Writer:        w,
	}
}

func (s *splitter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if s.pos == s.MaxLineLength {
			_, err = s.Writer.Write([]byte("\r\n"))
			if err != nil {
				return
			}
			s.pos = 0
		}
		_, err = s.Writer.Write([]byte{b})
		if err != nil {
			return
		}
		s.pos++
	}
	n = len(p)
	return
}
