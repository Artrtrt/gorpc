package typedef

import "errors"

type GenericInfo struct {
	SN     [32]byte
	Uptime int64
	Busy   bool
}

type ServerInfo struct {
	Addr            [32]byte
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

func NewServerInfoControl(addr [32]byte, connectionLimit uint32) *ServerInfoControl {
	return &ServerInfoControl{
		connection: make(chan interface{}, 1),
		ServerInfo: ServerInfo{
			Addr:            addr,
			ConnectionCount: 0,
			ConnectionLimit: connectionLimit,
		},
	}
}
