package service

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"gopack/tagrpc"
	"gopack/xbyte"
	"internal/typedef"
)

const (
	TagSendClientInfoUdp = 3073
)

type GetDeviceInfo struct {
	*typedef.DeviceInfo
}

func (data GetDeviceInfo) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	deviceInfo, err := xbyte.StructToByte(*data.DeviceInfo)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = n.Response(TagGetDeviceInfo, deviceInfo)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	return
}

type ConnectToServer struct {
	TrpcDefaultHandler
	ExecuteJsonRPC
	*typedef.DeviceInfo
}

func (data ConnectToServer) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	defer n.Response(TagConnectToServer, []byte("OK"))

	if data.DeviceInfo.Busy {
		return
	}

	val = bytes.TrimRightFunc(val, func(r rune) bool {
		return r == 0
	})

	tcpAddr, err := net.ResolveTCPAddr("tcp", string(val))
	if err != nil {
		err = fmt.Errorf("ResolveTCPAddr: %s", err)
		return
	}

	conn, err := tagrpc.DialTCP(nil, tcpAddr)
	if err != nil {
		err = fmt.Errorf("DialTCP: %s", err)
		return
	}

	conn.HandleFunc(TagRemoteErr, data.TrpcDefaultHandler.RemoteErr.Handler)
	conn.HandleFunc(TagRsaSetup, data.TrpcDefaultHandler.RsaSetup.Handler)
	conn.HandleFunc(TagSendGenericInfo, data.TrpcDefaultHandler.SendGenericInfo.Handler)
	conn.HandleFunc(TagExecuteJsonRPC, data.ExecuteJsonRPC.Handler)
	fmt.Printf("Подключился к серверу %s\n", conn.Tcp.RemoteAddr())

	go func(*tagrpc.TCPConn) {
		data.DeviceInfo.Busy = true
		for {
			err = conn.Update(time.Second * 60)
			if err != nil {
				data.DeviceInfo.Busy = false
				fmt.Println(err)
				return
			}
		}
	}(conn)

	return
}

type ExecuteJsonRPC struct {
}

func (data ExecuteJsonRPC) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	val = bytes.TrimRightFunc(val, func(r rune) bool {
		return r == 0
	})

	body := bytes.NewReader(val)
	resp, err := http.Post("http://localhost/ubus", "application/json", body)
	if err != nil {
		err = fmt.Errorf("%s %s", "POST", err.Error())
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("%s %s", "ReadAll", err.Error())
		return
	}

	n.Response(TagExecuteJsonRPC, bodyBytes)
	return
}
