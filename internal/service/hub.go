package service

import (
	"crypto/rsa"
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
	ServerPublicKey *rsa.PublicKey
	HttpAddr        string
	Storage         *typedef.Storage
}

func (data SendClientHttpAddr) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	SN := utils.ByteArrToString(deviceInfo.SystemBoard.Serial[:])
	info := n.Storage["info"].(*typedef.Info)
	addr := "http://" + data.HttpAddr + "/api/ubus/" + "?SN=" + SN + "&endpoint=http://" + utils.ByteArrToString(info.ServerInfo.HttpAddr[:])
	(*data.Storage)[deviceInfo.SystemBoard.Serial].DevicePayload.HttpAddrChan <- addr
	return
}

// type GetUUID struct {
// 	DeviceStorage *typedef.DeviceStorage
// }

// func (data GetUUID) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
// 	var deviceInfo typedef.GenericInfo
// 	err = xbyte.ByteToStruct(val, &deviceInfo)
// 	if err != nil {
// 		err = fmt.Errorf("ByteToStruct: %s", err)
// 		return
// 	}

// 	uuid := utils.GenerateUUID(utils.ByteArrToString(deviceInfo.Serial[:]))
// 	uuidByte, err := uuid.MarshalBinary()
// 	if err != nil {
// 		err = fmt.Errorf("MarshalBinary: %s", err)
// 		return
// 	}

// 	data.DeviceStorage[deviceInfo.Serial].UUID = uuid.String()
// 	n.Response(TagGetUUID, uuidByte)
// 	return
// }

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

	fmt.Println(device.DevicePayload.ToConnTCP, device.GenericInfo.Busy)
	if device.DevicePayload.ToConnTCP || device.GenericInfo.Busy {
		err = errors.New("Устройство занято")
		return
	}

	device.DevicePayload.ToConnTCP = true

	select {
	case <-time.After(time.Second * 20):
		device.DevicePayload.ToConnTCP = false
		err = errors.New("Ошибка при подключении к устройству")
		return
	case resp = <-device.DevicePayload.HttpAddrChan:
	}

	return
}
