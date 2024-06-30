package service

import (
	"errors"
	"fmt"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/typedef"
	"internal/utils"
)

// TagRpc
const (
	TagConnectToHub     = 1025
	TagConnectToServer  = 1026
	TagSendInfoToServer = 1027
	TagGetServerInfo    = 1028
	TagGetDeviceInfo    = 1029
)

type SendClientHttpAddr struct {
	HttpAddr string
	Storage  *typedef.Storage
}

func (data SendClientHttpAddr) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		fmt.Println("ByteToStruct:", err)
		return
	}

	info := n.Storage["info"].(*typedef.Info)
	addr := "http://" + data.HttpAddr + "/api/ubus/" + "?UUID=" + deviceInfo.UUID.String() + "&endpoint=http://" + utils.ByteArrToString(info.ServerInfo.HttpAddr[:])
	(*data.Storage)[deviceInfo.SystemBoard.Serial].DevicePayload.HttpAddrChan <- addr
	return
}

// JsonRpc
const (
	MethodSendSN = "sendSN"
)

type ReceiveSN struct {
	Storage *typedef.Storage
}

func (data ReceiveSN) Handler(req interface{}) (resp interface{}, err error) {
	if req == "" {
		err = errors.New("SN не может быть пустым")
		return
	}

	SNBytes := [64]byte{}
	copy(SNBytes[:], []byte(req.(string)))
	_, ok := (*data.Storage)[SNBytes]
	if !ok {
		err = errors.New("Запрашиваемое устройство не найдено")
		return
	}

	device := (*data.Storage)[SNBytes]
	if device.Type != "router" {
		err = errors.New("Устройство не является роутером")
		return
	}

	if time.Now().Unix()-int64(device.DevicePayload.Time) > 120 {
		err = errors.New("Устройство недоступно")
		return
	}

	if device.DevicePayload.ToConnTCP || device.GenericInfo.Busy {
		err = errors.New("Устройство занято")
		return
	}

	device.DevicePayload.ToConnTCP = true

	select {
	case <-time.After(time.Second * 20):
		device.DevicePayload.ToConnTCP = false
		err = errors.New("Превышено время подключения к устройтву")
	case resp = <-device.DevicePayload.HttpAddrChan:
	case err = <-device.DevicePayload.ErrChan:
		fmt.Println("aa")
	}

	return
}
