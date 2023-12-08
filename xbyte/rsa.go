package xbyte

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

func RsaKeyToByte(src interface{}) (dst []byte, err error) {
	var pemType string
	var rsaByte []byte

	switch key := src.(type) {
	case *rsa.PrivateKey:
		pemType = "RSA PRIVATE KEY"
		rsaByte = x509.MarshalPKCS1PrivateKey(key)
	case *rsa.PublicKey:
		pemType = "RSA PUBLIC KEY"
		rsaByte, err = x509.MarshalPKIXPublicKey(key)

		if err != nil {
			return
		}
	}

	dst = pem.EncodeToMemory(
		&pem.Block{
			Type:  pemType,
			Bytes: rsaByte,
		},
	)

	return
}

func RsaPrivateToByte(src *rsa.PrivateKey) (dst []byte, err error) {
	return RsaKeyToByte(src)
}

func RsaPublicToByte(src *rsa.PublicKey) (dst []byte, err error) {
	return RsaKeyToByte(src)
}

func ByteToRsaPrivate(src []byte) (dst *rsa.PrivateKey, err error) {
	block, _ := pem.Decode(src)
	if block == nil {
		err = errors.New("failed to parse PEM block containing the key")
		return
	}

	dst, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return
	}

	return
}

func ByteToRsaPublic(src []byte) (dst *rsa.PublicKey, err error) {
	block, _ := pem.Decode(src)
	if block == nil {
		err = errors.New("failed to parse PEM block containing the key")
		return
	}

	var rsapublic any

	rsapublic, err = x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return
	}

	var ok bool

	dst, ok = rsapublic.(*rsa.PublicKey)
	if !ok {
		err = errors.New("key type is not RSA")
		return
	}

	return
}
