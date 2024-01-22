module tcp

go 1.20

replace gopack/tlv => ../../gopack/tlv/

replace gopack/xbyte => ../../gopack/xbyte/

replace rsautil => ../rsautil/

require (
	gopack/tlv v0.0.0-00010101000000-000000000000
	gopack/xbyte v0.0.0-00010101000000-000000000000
	rsautil v0.0.0-00010101000000-000000000000
)
