package app

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"gopack/tagrpc"
	"gopack/xbyte"
	"gorpc/internal/components"
	"gorpc/internal/components/server/config"
	"gorpc/internal/components/server/entity"
	"gorpc/internal/typedef"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Server struct {
	Context    context.Context
	Config     *config.Config
	PrivateKey *rsa.PrivateKey

	Storage typedef.Storage

	HubConn     *tagrpc.TCPConn
	HttpServer  *http.Server
	HttpHandler *http.ServeMux
	Tcp         *tagrpc.TCPListener

	Rpc struct {
		hubHandlers    map[int]components.HandlerInterface
		deviceHandlers map[int]components.HandlerInterface
	}
}

func NewServer(config *config.Config, privateKey *rsa.PrivateKey) *Server {
	return &Server{
		Config:     config,
		PrivateKey: privateKey,
		// Context:    context,
		HttpServer: &http.Server{
			Addr: fmt.Sprintf("%s:%s", config.Ip, config.HttpPort),
		},
	}
}

func (s *Server) Boot() (err error) {
	tcpAddr := fmt.Sprintf("%s:%s", s.Config.Ip, s.Config.TcpPort)
	addr, err := net.ResolveTCPAddr("tcp", tcpAddr)
	if err != nil {
		return
	}

	tcpLr, err := tagrpc.ListenTCP(addr)
	if err != nil {
		return
	}

	s.Tcp = tcpLr
	return
}

func (s *Server) Start(ctx context.Context) (err error) {
	var wg sync.WaitGroup
	var errPool []error

	wg.Add(1)
	go func() {
		defer func() {
			log.Println("tcp stoped")
			wg.Done()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := s.Tcp.AcceptTCP()
				if err != nil {
					continue
				}
				go func(*tagrpc.TCPConn) {
					for {
						err = conn.Update(time.Second * 60)
						if err != nil {
							conn.Close()
							break
						}
					}
				}(conn)

				go s.HandleTCPConnection(conn)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer func() {
			log.Println("http stoped")
			wg.Done()
		}()
		if err := s.HttpServer.ListenAndServe(); err != nil {
			errPool = append(errPool, err)
		}
	}()

	log.Println("server is running")

	wg.Wait()
	if len(errPool) > 0 {
		return errors.Join(errPool...)
	}

	return
}

func (s *Server) Stop() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.HttpServer.Shutdown(ctx)
	if err != nil {
		log.Println("http server shutdown failed")
		return
	}

	err = s.Tcp.Close()
	if err != nil {
		log.Println("tcp shutdown failed")
		return
	}

	return
}

func (s *Server) GetGenericInfo(conn *tagrpc.TCPConn) (genericInfo *typedef.GenericInfo, err error) {
	response, err := conn.Execute(entity.TagSendGenericInfo, []byte{})
	if err != nil {
		fmt.Println("Execute", err)
		return
	}

	err = xbyte.ByteToStruct(response, genericInfo)
	if err != nil {
		fmt.Println("ByteToStruct:", err)
		return
	}

	return
}

func (s *Server) SendGenericInfoToHub(genericInfo *typedef.GenericInfo) (err error) {
	devicePayload, err := xbyte.StructToByte(genericInfo)
	if err != nil {
		fmt.Println("StructToByte:", err)
		return
	}

	err = s.HubConn.Request(entity.TagDeviceConnected, devicePayload)
	if err != nil {
		fmt.Println("Request", err)
		return
	}

	return
}

func (s *Server) HandleTCPConnection(conn *tagrpc.TCPConn) {
	// TODO: Убрать это, должно быть включено в tagrpc. Что то по типу conn.Setup()
	dst, err := xbyte.RsaPublicToByte(&s.PrivateKey.PublicKey)
	if err != nil {
		fmt.Println("RsaPublicToByte", err)
		return
	}

	response, err := conn.Execute(entity.TagRsaSetup, dst)
	if err != nil {
		fmt.Println("Execute", err)
		return
	}

	clientPublicKey, err := xbyte.ByteToRsaPublic(response)
	if err != nil {
		fmt.Println("ByteToRsaPublic", err)
		return
	}

	conn.Codec = tagrpc.NewRsaCodec(s.PrivateKey, clientPublicKey)

	// до сюда
	genericInfo, err := s.GetGenericInfo(conn)
	info, ok := s.Storage[genericInfo.SystemBoard.Serial]
	if !ok {
		conn.Write(entity.TagRemoteErr, []byte("Unknown device"))
		conn.Close()
		return
	}

	info.DevicePayload.ToConnTCP = false
	info.Conn = conn
	info.GenericInfo.UUID = uuid.New()
	conn.Storage["info"] = info

	err = s.SendGenericInfoToHub(info.GenericInfo)
	if err != nil {
		return
	}
}
