package typedef

import (
	"errors"
	"fmt"
	"gopack/tagrpc"
)

type SystemBoard struct {
	Manufacturer [64]byte
	Product      [64]byte
	Hostname     [64]byte
	Serial       [64]byte
	Release      struct {
		Revision [64]byte
		Version  [64]byte
	}
}

type GenericInfo struct {
	SystemBoard SystemBoard
	Uptime      uint64
	Busy        bool
}

type ServerInfo struct {
	TcpAddr         [32]byte
	HttpAddr        [32]byte
	ConnectionCount uint32
	ConnectionLimit uint32
}

type ServerInfoControl struct {
	connection chan interface{}
	ServerInfo
}

func (sic *ServerInfoControl) ConnectionInc() error {
	sic.connection <- true

	if sic.ConnectionCount >= sic.ConnectionLimit {
		<-sic.connection
		return errors.New("error")
	}

	sic.ConnectionCount++
	<-sic.connection

	return nil
}

func (sic *ServerInfoControl) ConnectionDec() error {
	sic.connection <- true

	if sic.ConnectionCount == 0 {
		<-sic.connection
		return errors.New("error")
	}

	sic.ConnectionCount--
	<-sic.connection

	return nil
}

func NewServerInfoControl(tcpAddr [32]byte, httpAddr [32]byte, connectionLimit uint32) *ServerInfoControl {
	return &ServerInfoControl{
		connection: make(chan interface{}, 1),
		ServerInfo: ServerInfo{
			TcpAddr:         tcpAddr,
			HttpAddr:        httpAddr,
			ConnectionCount: 0,
			ConnectionLimit: connectionLimit,
		},
	}
}

type DevicePayload struct {
	GenericInfo  GenericInfo
	UUID         string
	Time         int64
	SentToDB     bool
	ToConnTCP    bool
	HttpAddrChan chan string
}

type DeviceStorage map[[64]byte]DevicePayload

// FIXIT
func (s DeviceStorage) Contains(SN [64]byte) bool {
	for _, payload := range s {
		if payload.GenericInfo.SystemBoard.Serial == SN {
			return true
		}
	}

	return false
}

type ServerPayload struct {
	GenericInfo *GenericInfo
	ServerInfo  *ServerInfo
}

type ServerStorage map[*tagrpc.TCPConn]ServerPayload

// FIXIT
func (s ServerStorage) Contains(SN [64]byte) bool {
	for _, payload := range s {
		if payload.GenericInfo.SystemBoard.Serial == SN {
			return true
		}
	}

	return false
}

func (s ServerStorage) LessBusyServer() (conn *tagrpc.TCPConn, addr [32]byte, err error) {
	if len(s) == 0 {
		err = fmt.Errorf("%s", "No servers")
		return
	}

	maxFreeConn := 0
	for key, server := range s {
		freeConn := server.ServerInfo.ConnectionLimit - server.ServerInfo.ConnectionCount
		if freeConn > uint32(maxFreeConn) {
			addr = server.ServerInfo.TcpAddr
			conn = key
			maxFreeConn = int(freeConn)
		}
	}
	return
}
