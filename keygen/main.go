package main

import (
	"fmt"

	"gopack/rsautil"
)

var (
	privateKeyName = "private.pem"
	publicKeyName  = "public.pem"
	bits           = 2048
)

func main() {
	privateKey, publicKey, err := rsautil.GenerateRSAKeyPair(bits)
	if err != nil {
		fmt.Println("GenerateRSAKeyPair", err)
		return
	}

	err = rsautil.PrivateKeytoPem(privateKeyName, privateKey)
	if err != nil {
		fmt.Println("PrivateKeytoPem", err)
		return
	}

	err = rsautil.PublicKeytoPem(publicKeyName, publicKey)
	if err != nil {
		fmt.Println("PublicKeytoPem", err)
		return
	}
}
