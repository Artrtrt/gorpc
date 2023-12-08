package tcp

import (
	"crypto/rsa"
	"net"
)

type Server struct {
	Laddr     string
	lr        *net.TCPListener
	publicKey *rsa.PublicKey
}

func NewServer(laddr string) *Server {
	return &Server{
		Laddr: laddr,
	}
}

func (s *Server) Start() (err error) {
	tpcAddr, err := net.ResolveTCPAddr("tcp", s.Laddr)
	if err != nil {
		return
	}

	lr, err := net.ListenTCP("tcp", tpcAddr)
	if err != nil {
		return
	}

	defer lr.Close()
	s.lr = lr

	// go s.acceptLoop()
	return
}

// func (s *Server) acceptLoop() {
// 	for {
// 		conn, err := s.lr.AcceptTCP()
// 		if err != nil {
// 			fmt.Println("AcceptTCP", err)
// 			continue
// 		}

// 	}
// }

// func (s *Server) Listen() (err error) {
// 	lr, err := net.ListenTCP("tcp", s.Laddr)
// 	if err != nil {
// 		fmt.Println("ListenTCP", err)
// 		return
// 	}

// 	for {
// 		conn, err := lr.AcceptTCP()
// 		if err != nil {
// 			fmt.Println("AcceptTCP", err)
// 			continue
// 		}

// 		go func() {
// 			defer conn.Close()
// 			if conn == nil {
// 				fmt.Println("No connection")
// 				return
// 			}

// 			clientPublicKey, err := rsaSetup(conn, publicKey)
// 			if err != nil {
// 				fmt.Println("rsaSetup", err)
// 				conn.Close()
// 				return
// 			}

// 			go handleTCPConn(conn, clientPublicKey)
// 		}()
// 	}
// 	return
// }
