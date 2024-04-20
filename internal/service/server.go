package service

import (
	"fmt"
	"gopack/tagrpc"
	"gopack/xbyte"

	"internal/typedef"
)

const (
	TagReceiveDeviceInfo = 1027
	TagSendServerInfo    = 1029
)

type ReceiveDeviceInfo struct {
	WantToConnectStorage map[[16]byte]typedef.GenericInfo
}

func (data ReceiveDeviceInfo) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	defer n.Response(TagReceiveDeviceInfo, []byte("OK"))
	var deviceInfo typedef.GenericInfo
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	_, ok := data.WantToConnectStorage[deviceInfo.SN]
	if !ok {
		data.WantToConnectStorage[deviceInfo.SN] = deviceInfo
	}
	return
}

type SendServerInfo struct {
	*typedef.ServerInfoControl
}

func (data SendServerInfo) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	serverInfo, err := xbyte.StructToByte(data.ServerInfoControl.ServerInfo)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = n.Response(1029, serverInfo)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	return
}
