package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func GenerateRSAKeyPair(bits int) (privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, err error) {
	privateKey, err = rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		err = fmt.Errorf("%s %s", "GenerateKey", err)
		return
	}

	publicKey = &privateKey.PublicKey

	return
}

func PrivateKeytoPem(fileName string, key *rsa.PrivateKey) (err error) {
	file, err := os.Create(fileName)
	if err != nil {
		err = fmt.Errorf("%s %s", "Create", err)
		return
	}

	defer file.Close()

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	err = pem.Encode(file, block)
	if err != nil {
		err = fmt.Errorf("%s %s", "Encode", err)
		return
	}

	return
}

func PublicKeytoPem(fileName string, key *rsa.PublicKey) (err error) {
	bytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		err = fmt.Errorf("%s %s", "MarshalPKIXPublicKey", err)
		return
	}

	block := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: bytes,
	}

	file, err := os.Create(fileName)
	if err != nil {
		err = fmt.Errorf("%s %s", "Create", err)
		return
	}

	defer file.Close()
	err = pem.Encode(file, block)
	if err != nil {
		err = fmt.Errorf("%s %s", "Encode", err)
		return
	}

	return
}

func PemToPublicKey(fileName string) (key *rsa.PublicKey, err error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		err = fmt.Errorf("%s %s", "ReadFile", err)
		return
	}

	block, _ := pem.Decode(data)
	if block == nil {
		err = fmt.Errorf("%s", "Failed to parse PEM block")
		return
	}

	if block.Type != "RSA PUBLIC KEY" {
		err = fmt.Errorf("%s", "Invalid key type")
		return
	}

	parseData, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		err = fmt.Errorf("%s %s", "ParsePKIXPublicKey", err)
		return
	}

	key, ok := parseData.(*rsa.PublicKey)
	if !ok {
		err = fmt.Errorf("%s", "Failed to parse key")
		return
	}

	return
}

func PemToPrivateKey(fileName string) (key *rsa.PrivateKey, err error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		err = fmt.Errorf("%s %s", "ReadFile", err)
		return
	}

	block, _ := pem.Decode(data)
	if block == nil {
		err = fmt.Errorf("%s", "Failed to parse PEM block")
		return
	}

	if block.Type != "RSA PRIVATE KEY" {
		err = fmt.Errorf("%s", "Invalid key type")
		return
	}

	key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		err = fmt.Errorf("%s %s", "ParsePKCS1PrivateKey", err)
		return
	}

	return
}

func DecryptPKCS1(privateKey *rsa.PrivateKey, ciphertext []byte) (decriptData []byte, err error) {
	decriptData, err = rsa.DecryptPKCS1v15(rand.Reader, privateKey, ciphertext)
	if err != nil {
		err = fmt.Errorf("%s %s", "DecryptPKCS1v15", err)
		return
	}

	return
}

func EncryptPKCS1(publicKey *rsa.PublicKey, msg []byte) (encriptData []byte, err error) {
	encriptData, err = rsa.EncryptPKCS1v15(rand.Reader, publicKey, msg)
	if err != nil {
		err = fmt.Errorf("%s %s", "EncryptPKCS1v15", err)
		return
	}

	return
}
