package typedef

import (
	"errors"
	"fmt"
	"gopack/tagrpc"
	"internal/utils"
)

type SystemBoard struct {
	Manufacturer [64]byte
	Product      [64]byte
	Hostname     [64]byte
	Serial       [64]byte
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

type DevicePayload struct {
	UUID         string
	Time         uint64
	ToConnTCP    bool
	HttpAddrChan chan string
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

type Info struct {
	Type     string
	Conn     *tagrpc.TCPConn
	SentToDB bool

	GenericInfo   *GenericInfo
	DevicePayload *DevicePayload
	ServerInfo    *ServerInfo
}

func (i Info) MatchGenericInfo(genericInfo GenericInfo) bool {
	if i.GenericInfo.SystemBoard.Hostname != genericInfo.SystemBoard.Hostname {
		return false
	}

	if i.GenericInfo.SystemBoard.Manufacturer != genericInfo.SystemBoard.Manufacturer {
		return false
	}

	if i.GenericInfo.SystemBoard.Product != genericInfo.SystemBoard.Product {
		return false
	}

	if i.GenericInfo.SystemBoard.Serial != genericInfo.SystemBoard.Serial {
		return false
	}

	return true
}

func (i Info) WhitelistContainsServer(whitelist []string) bool {
	if i.Type != "server" {
		return false
	}

	serial := utils.ByteArrToString(i.GenericInfo.SystemBoard.Serial[:])
	for _, val := range whitelist {
		if val == serial {
			return true
		}
	}

	return false
}

type Storage map[[64]byte]*Info

func (s Storage) RouterExist(serial [64]byte) bool {
	_, ok := s[serial]
	return ok && s[serial].Type == "router"
}

func (s Storage) LessBusyServer() (conn *tagrpc.TCPConn, addr [32]byte, err error) {
	maxFreeConn := 0
	for _, info := range s {
		if info.Type != "server" || info.ServerInfo == nil {
			continue
		}

		freeConn := info.ServerInfo.ConnectionLimit - info.ServerInfo.ConnectionCount
		if freeConn > uint32(maxFreeConn) {
			addr = info.ServerInfo.TcpAddr
			conn = info.Conn
			maxFreeConn = int(freeConn)
		}
	}

	if conn == nil {
		err = fmt.Errorf("%s", "No servers available")
		return
	}

	return
}

type ToSql struct {
	Id           int64  `sql:"NAME=Id, TYPE=INTEGER, PRIMARY_KEY, AUTO_INCREMENT"`
	Manufacturer string `sql:"NAME=Manufacturer, TYPE=TEXT(64)"`
	Product      string `sql:"NAME=Product, TYPE=TEXT(64)"`
	Hostname     string `sql:"NAME=Hostname, TYPE=TEXT(64)"`
	Serial       string `sql:"NAME=Serial, TYPE=TEXT(64)"`
	Type         string `sql:"NAME=Type, TYPE=TEXT(64)"`
}
