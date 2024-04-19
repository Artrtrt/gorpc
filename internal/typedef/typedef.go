package typedef

import "errors"

type GenericInfo struct {
	// Manufacturer [64]byte
	// Product      [64]byte
	// Hostname     [64]byte
	// Serial       [64]byte
	// Release      struct {
	// 	Revision [64]byte
	// 	Version  [64]byte
	// }
	SN     [16]byte
	Uptime int64
	Busy   bool
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
