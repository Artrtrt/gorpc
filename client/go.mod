module client

go 1.20

replace gopack/tlv => ../tlv/

require gopack/tlv v0.0.0

replace gopack/xbyte => ../xbyte/

require gopack/xbyte v0.0.0

require (
	gopack/rsautil v0.0.0-00010101000000-000000000000 // indirect
	tcp v0.0.0-00010101000000-000000000000 // indirect
)

replace gopack/rsautil => ../rsautil/

replace tcp => ../tcp/
