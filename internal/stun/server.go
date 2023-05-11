package stun

import (
	"errors"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/pion/stun"
	"go.uber.org/zap"
	"net"
	"strconv"
	"sync"
)

func ListenAndStart(address string, log *zap.Logger) (*ClosableServer, error) {
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}

	_, port, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return nil, err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	if log == nil {
		log = zap.NewNop()
	}

	s := &ClosableServer{
		conn: conn,
		Port: p,
		Server: Server{
			Log: log,
		},
	}

	s.Log.Info("Stun server listening", zap.Int("port", p))

	util.GoWithWaitGroup(&s.wg, func() {
		err = s.Serve(conn)
		if err != nil {
			s.Log.Info("Failed Serve", zap.Error(err))
		}
	})

	return s, nil
}

type ClosableServer struct {
	conn net.PacketConn
	wg   sync.WaitGroup
	Server
	Port int
}

func (s *ClosableServer) Close() error {
	return s.conn.Close()
}
func (s *ClosableServer) Shutdown() error {
	err := s.Close()
	if err != nil {
		return err
	}
	s.wg.Wait()
	return nil
}

func ListenAndServe(address string, log *zap.Logger) error {
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return err
	}

	if log == nil {
		log = zap.NewNop()
	}

	s := &Server{
		Log: log,
	}

	s.Log.Info("Stun server listening", zap.String("port", conn.LocalAddr().String()))
	return s.Serve(conn)
}

type Server struct {
	Log *zap.Logger
}

func (s *Server) Serve(conn net.PacketConn) error {
	buf := make([]byte, 1024)
	response := stun.Message{}
	request := stun.Message{}
	fromAddress := stun.XORMappedAddress{}
	var software = stun.NewSoftware("nexodus")
	for {

		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		requestBytes := buf[:n]

		if !stun.IsMessage(requestBytes) {
			s.Log.Debug("Not a STUN request")
			continue
		}

		switch addr := addr.(type) {
		case *net.UDPAddr:
			fromAddress.IP = addr.IP
			fromAddress.Port = addr.Port
		default:
			panic("expected *net.UDPAddr")
		}

		request.Reset()
		if _, err = request.Write(requestBytes); err != nil {
			s.Log.Debug("Failed request.Write", zap.Error(err))
			continue
		}

		response.Reset()
		err = response.Build(&request,
			stun.BindingSuccess,
			software,
			&fromAddress,
			stun.Fingerprint,
		)

		if err != nil {
			s.Log.Info("Failed response.Build", zap.Error(err))
			continue
		}

		_, err = conn.WriteTo(response.Raw, addr)
		if err != nil {
			s.Log.Info("Failed conn.WriteTo", zap.Error(err))
		}

		s.Log.Debug("Stun server processed request: ", zap.String("endpoint", fromAddress.String()))

	}
}
