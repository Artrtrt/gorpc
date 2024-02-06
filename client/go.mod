module client

go 1.20

replace gopack/tlv => ../../gopack/tlv/

replace gopack/tagrpc => ../../gopack/tagrpc/

replace rsautil => ../rsautil/

replace tcp => ../tcp/

replace typedef => ../typedef/

replace tag => ../tag/

require gopack/tlv v0.0.0

replace gopack/xbyte => ../../gopack/xbyte/

require (
	gopack/tagrpc v0.0.0-00010101000000-000000000000
	gopack/xbyte v0.0.0
	rsautil v0.0.0-00010101000000-000000000000
	tcp v0.0.0-00010101000000-000000000000
	typedef v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	tag v0.0.0-00010101000000-000000000000 // indirect
)
