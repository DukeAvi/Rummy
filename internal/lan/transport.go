package lan

import (
	"encoding/json"
	"fmt"
	"github.com/DukeAvi/rummy-tui/internal/game"
	"io"
	"net"
	"strings"
	"time"
)

const Port = "45873"

type Server struct {
	Session  *Session
	listener net.Listener
}

func Listen(cfg game.Config) (*Server, error) {
	l, err := net.Listen("tcp", ":"+Port)
	if err != nil {
		return nil, fmt.Errorf("cannot host on port %s: %w", Port, err)
	}
	s, err := New(cfg)
	if err != nil {
		l.Close()
		return nil, err
	}
	server := &Server{Session: s, listener: l}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go ServeConn(c, s)
		}
	}()
	return server, nil
}
func (s *Server) Close() { s.Session.Close(); _ = s.listener.Close() }
func ServeConn(c net.Conn, s *Session) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	var r Request
	if err := json.NewDecoder(io.LimitReader(c, 65536)).Decode(&r); err != nil {
		return
	}
	_ = json.NewEncoder(c).Encode(s.Handle(r))
}
func Call(address string, r Request) (Response, error) {
	c, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf("cannot reach host: %w", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if err = json.NewEncoder(c).Encode(r); err != nil {
		return Response{}, err
	}
	var out Response
	err = json.NewDecoder(io.LimitReader(c, 65536)).Decode(&out)
	return out, err
}
func ParseCode(code string) (string, string, error) {
	address, token, ok := strings.Cut(strings.TrimSpace(code), "/")
	if !ok || len(token) != 32 {
		return "", "", fmt.Errorf("paste the entire address:port/code from the HOST LOBBY; the host must first choose the mode and wild rank to generate it")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return "", "", fmt.Errorf("code must contain an address and port")
	}
	return address, token, nil
}
func LocalAddress() string {
	addresses, _ := net.InterfaceAddrs()
	for _, a := range addresses {
		ip, _, err := net.ParseCIDR(a.String())
		if err == nil && ip.To4() != nil && !ip.IsLoopback() {
			return ip.String()
		}
	}
	return "127.0.0.1"
}
