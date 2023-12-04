package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"io"
	"math/rand"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
	"time"
)

type Attachment struct {
	Name        string
	CID         string
	ContentType string
	Content     io.Reader
	Inline      bool
}

type Message struct {
	From         string
	To           []string
	Subject      string
	PlainMessage string
	HtmlMessages string
	Attachments  []Attachment
	Rand         *rand.Rand
}

func (e *Message) Write(w io.Writer) (err error) {

	_, err = fmt.Fprintf(w,
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n",
		e.From, strings.Join(e.To, ", "), e.Subject)
	if err != nil {
		return err
	}

	mixed, err := NewWriter(w, "multipart/mixed", e.Rand)
	defer util.CLose(&err, mixed.Close)

	alternatives, err := mixed.AddWriter("multipart/alternative")
	if err != nil {
		return err
	}
	defer util.CLose(&err, alternatives.Close)

	if e.PlainMessage != "" {
		err = alternatives.AddQuotedPrintablePart("text/plain", []byte(e.PlainMessage))
		if err != nil {
			return err
		}
	}

	related, err := alternatives.AddWriter("multipart/related")
	if err != nil {
		return err
	}
	defer util.CLose(&err, related.Close)

	if e.HtmlMessages != "" {
		err = related.AddQuotedPrintablePart("text/html", []byte(e.HtmlMessages))
		if err != nil {
			return err
		}

		// write the inline attachments
		for _, attachment := range e.Attachments {
			if !attachment.Inline {
				continue
			}
			err = attachment.write(related)
			if err != nil {
				return err
			}
		}
	}

	// write the normal attachments
	for _, attachment := range e.Attachments {
		if attachment.Inline {
			continue
		}
		err = attachment.write(mixed)
		if err != nil {
			return err
		}
	}
	return nil
}

func (attachment Attachment) write(writer Writer) error {
	if attachment.CID == "" {
		attachment.CID = attachment.Name
	}
	disposition := "attachment"
	if attachment.Inline {
		disposition = "inline"
	}
	headers := textproto.MIMEHeader{
		"Content-Disposition":       {fmt.Sprintf(`%s; filename="%s""`, disposition, attachment.Name)},
		"Content-Id":                {fmt.Sprintf("<%s>", attachment.CID)},
		"Content-Transfer-Encoding": {"BASE64"},
	}
	if attachment.ContentType != "" {
		headers["Content-Type"] = []string{fmt.Sprintf(`%s; name="%s"`, attachment.ContentType, attachment.Name)}
	}

	buf := bytes.NewBuffer(nil)
	base64Writer := base64.NewEncoder(base64.StdEncoding, buf)
	_, err := io.Copy(base64Writer, attachment.Content)
	if err != nil {
		return err
	}
	err = writer.AddPart(headers, buf)
	if err != nil {
		return err
	}
	return nil
}

type Writer struct {
	multi *multipart.Writer
	rand  *rand.Rand
}

var globalRand *rand.Rand

func init() { // #nosec G404
	globalRand = rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
}

func NewWriter(w io.Writer, contentType string, random *rand.Rand) (Writer, error) {
	if random == nil {
		random = globalRand
	}
	boundary := randomBoundary(random)
	_, err := fmt.Fprintf(w, "Content-Type: %s; boundary=%s\r\n\r\n", contentType, boundary)
	if err != nil {
		return Writer{}, err
	}
	writer := multipart.NewWriter(w)
	err = writer.SetBoundary(boundary)
	if err != nil {
		return Writer{}, err
	}

	return Writer{
		multi: writer,
		rand:  random,
	}, nil
}

func randomBoundary(rand *rand.Rand) string {
	var buf [30]byte
	_, err := io.ReadFull(rand, buf[:])
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", buf[:])
}
func (w Writer) AddWriter(contentType string) (Writer, error) {
	boundary := randomBoundary(w.rand)
	part, err := w.multi.CreatePart(
		textproto.MIMEHeader{"Content-Type": {
			fmt.Sprintf(`%s; boundary="%s"`, contentType, boundary),
		}},
	)
	if err != nil {
		return Writer{}, err
	}

	child := multipart.NewWriter(part)
	err = child.SetBoundary(boundary)
	if err != nil {
		return Writer{}, err
	}
	return Writer{multi: child, rand: w.rand}, nil
}

func (w Writer) AddQuotedPrintablePart(contentType string, content []byte) error {
	headers := textproto.MIMEHeader{
		"Content-Transfer-Encoding": {"quoted-printable"},
		"Content-Type":              {contentType},
	}
	buf := bytes.NewBuffer(nil)
	qp := quotedprintable.NewWriter(buf)
	_, err := qp.Write(content)
	if err != nil {
		return err
	}
	err = qp.Close()
	if err != nil {
		return err
	}
	return w.AddPart(headers, buf)
}

func (w Writer) AddPart(headers textproto.MIMEHeader, reader io.Reader) error {
	part, err := w.multi.CreatePart(headers)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, reader)
	if err != nil {
		return err
	}
	return nil
}

func (w Writer) Close() error {
	return w.multi.Close()
}

func (w Writer) Boundary() string {
	return w.multi.Boundary()
}
