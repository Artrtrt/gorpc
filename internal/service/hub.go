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
)

type SendClientHttpAddr struct {
	ServerPublicKey *rsa.PublicKey
	HttpAddr        string
	DeviceStorage   *typedef.DeviceStorage
}

func (data SendClientHttpAddr) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo := typedef.GenericInfo{}
	err = xbyte.ByteToStruct(val, &deviceInfo)
	if err != nil {
		err = fmt.Errorf("ByteToStruct: %s", err)
		return
	}

	SN := utils.ByteArrToString(deviceInfo.SystemBoard.Serial[:])
	serverAddr := n.Storage["httpAddr"].(string)
	addr := "http://" + data.HttpAddr + "/api/ubus/" + "?SN=" + SN + "&endpoint=http://" + serverAddr
	(*data.DeviceStorage)[deviceInfo.SystemBoard.Serial].HttpAddrChan <- addr
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
	DeviceStorage *typedef.DeviceStorage
}

func (data ReceiveSN) Handler(req interface{}) (resp interface{}, err error) {
	if req == "" {
		err = errors.New("SN не может быть пустым")
		return
	}

	SNBytes := [64]byte{}
	copy(SNBytes[:], []byte(req.(string)))
	_, ok := (*data.DeviceStorage)[SNBytes]
	if !ok {
		err = errors.New("Запрашиваемое устройство не найдено")
		return
	}

	device := (*data.DeviceStorage)[SNBytes]
	if time.Now().Unix()-device.Time > 120 {
		err = errors.New("Устройство не доступно")
		return
	}

	payload := (*data.DeviceStorage)[SNBytes]
	if payload.ToConnTCP || payload.GenericInfo.Busy {
		err = errors.New("Устройство занято")
		return
	}

	payload.ToConnTCP = true
	(*data.DeviceStorage)[SNBytes] = payload

	select {
	case <-time.After(time.Second * 20):
		payload.ToConnTCP = false
		(*data.DeviceStorage)[SNBytes] = payload
		err = errors.New("Ошибка при подключении к устройству")
		return
	case resp = <-(*data.DeviceStorage)[SNBytes].HttpAddrChan:
	}

	return
}
