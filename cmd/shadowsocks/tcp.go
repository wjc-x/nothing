package shadowsocks

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/wjc-x/nothing/socks"
	"github.com/wjc-x/nothing/stat"
)

var (
	l net.Listener
)

type RecordConn struct {
	net.Conn
	meter stat.TrafficMeter
}

func (c *RecordConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.meter.Count(0, uint64(n))
	return n, err
}

func (c *RecordConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.meter.Count(uint64(n), 0)
	return n, err
}

// Create a SOCKS server listening on addr and proxy to server.
func socksLocal(addr, server string, meter stat.TrafficMeter, shadow func(net.Conn) net.Conn, ctx context.Context) {
	logf("SOCKS proxy %s <-> %s", addr, server)
	tcpLocal(addr, server, ctx, meter, shadow, func(c net.Conn) (socks.Addr, error) { return socks.Handshake(c) })
}

// Listen on addr and proxy to server to reach target from getAddr.
func tcpLocal(addr, server string, ctx context.Context, meter stat.TrafficMeter, shadow func(net.Conn) net.Conn, getAddr func(net.Conn) (socks.Addr, error)) {
	var err error
	l, err = net.Listen("tcp", addr)
	if err != nil {
		logf("failed to listen on %s: %v", addr, err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			logf("failed to accept: %s", err)
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		go func() {
			defer c.Close()
			c.(*net.TCPConn).SetKeepAlive(true)
			tgt, err := getAddr(c)
			if err != nil {

				// UDP: keep the connection until disconnect then free the UDP socket
				if err == socks.InfoUDPAssociate {
					buf := make([]byte, 1)
					// block here
					for {
						_, err := c.Read(buf)
						if err, ok := err.(net.Error); ok && err.Timeout() {
							continue
						}
						logf("UDP Associate End.")
						return
					}
				}

				logf("failed to get target address: %v", err)
				return
			}

			rc, err := net.Dial("tcp", server)
			if err != nil {
				logf("failed to connect to server %v: %v", server, err)
				return
			}
			defer rc.Close()
			rc.(*net.TCPConn).SetKeepAlive(true)
			rc = shadow(rc)
			
			recordConn := &RecordConn{
				Conn:  rc,
				meter: meter,
			}

			if _, err = rc.Write(tgt); err != nil {
				logf("failed to send target address: %v", err)
				return
			}

			logf("proxy %s <-> %s <-> %s", c.RemoteAddr(), server, tgt)
			_, _, err = relay(recordConn, c)
			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					return // ignore i/o timeout
				}
				logf("relay error: %v", err)
			}
		}()
	}
}

// Close Tcp Local Port
func closeTcpLocal() {
	l.Close()
}

// relay copies between left and right bidirectionally. Returns number of
// bytes copied from right to left, from left to right, and any error occurred.
func relay(left, right net.Conn) (int64, int64, error) {
	type res struct {
		N   int64
		Err error
	}
	ch := make(chan res)

	go func() {
		n, err := io.Copy(right, left)
		right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
		left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
		ch <- res{n, err}
	}()

	n, err := io.Copy(left, right)
	right.SetDeadline(time.Now()) // wake up the other goroutine blocking on right
	left.SetDeadline(time.Now())  // wake up the other goroutine blocking on left
	rs := <-ch

	if err == nil {
		err = rs.Err
	}
	return n, rs.N, err
}
