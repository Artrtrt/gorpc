module tcp

go 1.20

replace gopack/tlv => ../../gopack/tlv/

replace gopack/xbyte => ../../gopack/xbyte/

replace gopack/tagrpc => ../../gopack/tagrpc/

replace rsautil => ../rsautil/

require (
	gopack/tlv v0.0.0-00010101000000-000000000000
	gopack/xbyte v0.0.0-00010101000000-000000000000
	rsautil v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	gopack/tagrpc v0.0.0-00010101000000-000000000000 // indirect
)
