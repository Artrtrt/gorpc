package service

import (
	"fmt"

	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/typedef"
)

const (
	TagSendServerInfoUdp = 2049
	TagDeviceConnected   = 2050
	TagExecuteJsonRPC    = 2052
	TagGetUUID           = 2053
)

type ReceiveDeviceInfo struct {
	Storage *typedef.Storage
}

func (data ReceiveDeviceInfo) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	defer n.Response(TagSendInfoToServer, []byte("OK"))
	var genericInfo typedef.GenericInfo
	err = xbyte.ByteToStruct(val, &genericInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	// uuid, err := n.Execute(TagGetUUID, deviceInfo)
	// if err != nil {
	// 	err = fmt.Errorf("ByteToStruct: %s", err)
	// 	return
	// }

	// fmt.Println(uuid)
	// data.WantToConnectStorage[uuid] = deviceInfo.SystemBoard.Serial

	(*data.Storage)[genericInfo.SystemBoard.Serial] = &typedef.Info{
		Type:        "router",
		GenericInfo: &genericInfo,
		DevicePayload: &typedef.DevicePayload{
			ToConnTCP: true,
		},
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

	err = n.Response(TagGetServerInfo, serverInfo)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	return
}
