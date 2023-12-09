module server

go 1.20

replace tcp => ../tcp/

replace gopack/xbyte => ../xbyte/

replace gopack/tlv => ../tlv/

replace gopack/rsautil => ../rsautil/

require tcp v0.0.0-00010101000000-000000000000

require (
	gopack/rsautil v0.0.0-00010101000000-000000000000 // indirect
	gopack/tlv v0.0.0-00010101000000-000000000000 // indirect
	gopack/xbyte v0.0.0-00010101000000-000000000000 // indirect
)
