package service

import (
	"crypto/rsa"
	"fmt"
	"gopack/tagrpc"
	"gopack/xbyte"

	typedef "internal/typedef"
)

type RemoteErr struct {
}

func (data RemoteErr) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	return fmt.Errorf("RemoteErr: %s", string(val))
}

type RsaSetup struct {
	PrivateKey *rsa.PrivateKey
}

func (data RsaSetup) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	rPublicKey, err := xbyte.ByteToRsaPublic(val)
	if err != nil {
		err = fmt.Errorf("%s %s", "ByteToRsaPublic:", err)
		return
	}

	dst, err := xbyte.RsaPublicToByte(&data.PrivateKey.PublicKey)
	if err != nil {
		err = fmt.Errorf("%s %s", "RsaPublicToByte:", err)
		return
	}

	err = n.Response(2, dst)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	n.Codec = tagrpc.NewRsaCodec(data.PrivateKey, rPublicKey)
	return
}

type SendGenericInfo struct {
	GenericInfo *typedef.GenericInfo
}

func (data SendGenericInfo) Handler(n *tagrpc.Node, tag uint16, val []byte) (err error) {
	byteInfo, err := xbyte.StructToByte(*data.GenericInfo)
	if err != nil {
		err = fmt.Errorf("StructToByte: %s", err)
		return
	}

	err = n.Response(3, byteInfo)
	if err != nil {
		err = fmt.Errorf("%s %s", "Response:", err)
		return
	}

	return
}
