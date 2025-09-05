package ws

import (
	"io"

	gobwas "github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func ReadClientData(rw io.ReadWriter) ([]byte, gobwas.OpCode, error) {
	s := gobwas.StateServerSide
	want := gobwas.OpText | gobwas.OpBinary

	return readData(rw, s, want)
}

func DiscardClientData(rw io.ReadWriter) ([]byte, gobwas.OpCode, error) {
	s := gobwas.StateServerSide

	return readData(rw, s, 0)
}

func readData(rw io.ReadWriter, s gobwas.State, want gobwas.OpCode) ([]byte, gobwas.OpCode, error) {
	controlHandler := wsutil.ControlFrameHandler(rw, s)
	rd := wsutil.Reader{
		Source:          rw,
		State:           s,
		CheckUTF8:       true,
		SkipHeaderCheck: false,
		OnIntermediate:  controlHandler,
	}
	for {
		hdr, err := rd.NextFrame()
		if err != nil {
			return nil, 0, err
		}
		if hdr.OpCode.IsControl() {
			if cerr := controlHandler(hdr, &rd); cerr != nil {
				return nil, 0, cerr
			}

			if hdr.OpCode == gobwas.OpPong || hdr.OpCode == gobwas.OpPing {
				return nil, hdr.OpCode, nil
			}

			continue
		}
		if hdr.OpCode&want == 0 {
			if derr := rd.Discard(); derr != nil {
				return nil, 0, derr
			}

			if want == 0 {
				return nil, hdr.OpCode, nil
			}

			continue
		}

		bts, err := io.ReadAll(&rd)

		return bts, hdr.OpCode, err
	}
}
