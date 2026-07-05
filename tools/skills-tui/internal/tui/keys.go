package tui

import (
	"io"
	"time"
)

// escTimeout bounds the total wait for the trailing bytes of an escape
// sequence after a lone ESC byte, matching bash `read -rsn2 -t 1`: one whole
// second for the remainder of the sequence, so an arrow press whose bytes
// arrive in separate packets (high-latency links) still decodes as an arrow
// instead of a lone ESC (which quits).
const escTimeout = 1 * time.Second

// KeyReader decodes keypresses from a byte stream, mirroring bash read_key.
// A background goroutine pumps single bytes into a channel so the ESC
// disambiguation can wait with a timeout on any io.Reader (tty or test
// fixture) alike.
type KeyReader struct {
	ch chan byte
}

// NewKeyReader starts decoding keys from r.
func NewKeyReader(r io.Reader) *KeyReader {
	kr := &KeyReader{ch: make(chan byte)}
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				kr.ch <- buf[0]
			}
			if err != nil {
				close(kr.ch)
				return
			}
		}
	}()
	return kr
}

// ReadKey reads one keypress, expanding arrow escape sequences:
//   - "\x1b[A" / "\x1b[B" for arrows (ESC plus up to two trailing bytes read
//     within escTimeout, like bash `read -rsn2 -t 1`)
//   - "" for Enter (\r or \n), matching bash read -rsn1 returning "" on the
//     delimiter
//   - the byte itself for anything else
//
// It returns an error only when the stream is exhausted (bash read failure).
func (k *KeyReader) ReadKey() (string, error) {
	b, ok := <-k.ch
	if !ok {
		return "", io.EOF
	}
	switch b {
	case 0x1b:
		seq := []byte{b}
		// One deadline for the whole trailing read, like bash's single
		// `read -rsn2 -t 1` call.
		deadline := time.After(escTimeout)
		for len(seq) < 3 {
			select {
			case b2, ok := <-k.ch:
				if !ok {
					return string(seq), nil
				}
				seq = append(seq, b2)
			case <-deadline:
				return string(seq), nil
			}
		}
		return string(seq), nil
	case '\r', '\n':
		return "", nil
	}
	return string(b), nil
}
