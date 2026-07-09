package panel

import (
	"bytes"
	"io"
	"net"
	"sync"
	"time"
)

// splitTLS splits one TCP listener into a TLS listener and a plain-HTTP
// listener by peeking at the first byte of each connection: a TLS ClientHello
// always starts with the handshake record type 0x16. This lets the panel serve
// HTTPS and the plain-HTTP subscription endpoint on a single port.
func splitTLS(parent net.Listener) (tlsLn, plainLn net.Listener) {
	sp := &splitter{
		parent: parent,
		tlsC:   make(chan net.Conn),
		plainC: make(chan net.Conn),
		done:   make(chan struct{}),
	}
	go sp.run()
	return &splitListener{sp: sp, conns: sp.tlsC}, &splitListener{sp: sp, conns: sp.plainC}
}

type splitter struct {
	parent       net.Listener
	tlsC, plainC chan net.Conn
	done         chan struct{}
	closeOnce    sync.Once
}

func (sp *splitter) close() {
	sp.closeOnce.Do(func() {
		close(sp.done)
		_ = sp.parent.Close()
	})
}

func (sp *splitter) run() {
	for {
		c, err := sp.parent.Accept()
		if err != nil {
			sp.close()
			return
		}
		// Peek in a goroutine so one silent client cannot stall the accept loop.
		go sp.dispatch(c)
	}
}

func (sp *splitter) dispatch(c net.Conn) {
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	var first [1]byte
	if _, err := io.ReadFull(c, first[:]); err != nil {
		_ = c.Close()
		return
	}
	_ = c.SetReadDeadline(time.Time{})

	pc := &peekedConn{Conn: c, r: io.MultiReader(bytes.NewReader(first[:]), c)}
	target := sp.plainC
	if first[0] == 0x16 {
		target = sp.tlsC
	}
	select {
	case target <- pc:
	case <-sp.done:
		_ = pc.Close()
	}
}

// peekedConn re-attaches the peeked byte in front of the connection stream.
type peekedConn struct {
	net.Conn
	r io.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// splitListener is one protocol side of a splitter.
type splitListener struct {
	sp    *splitter
	conns chan net.Conn
}

func (l *splitListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.sp.done:
		return nil, net.ErrClosed
	}
}

func (l *splitListener) Close() error {
	l.sp.close()
	return nil
}

func (l *splitListener) Addr() net.Addr { return l.sp.parent.Addr() }
