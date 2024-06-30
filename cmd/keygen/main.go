package main

import (
	"fmt"

	"internal/utils"
)

var (
	privateKeyName = "private.pem"
	publicKeyName  = "public.pem"
	bits           = 2048
)

func main() {
	privateKey, publicKey, err := utils.GenerateRSAKeyPair(bits)
	if err != nil {
		fmt.Println("GenerateRSAKeyPair", err)
		return
	}

	err = utils.PrivateKeytoPem(privateKeyName, privateKey)
	if err != nil {
		fmt.Println("PrivateKeytoPem", err)
		return
	}

	err = utils.PublicKeytoPem(publicKeyName, publicKey)
	if err != nil {
		fmt.Println("PublicKeytoPem", err)
		return
	}
}
