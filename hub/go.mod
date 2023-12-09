module hub

go 1.20

replace tcp => ../tcp/

replace gopack/xbyte => ../xbyte/

replace gopack/tlv => ../tlv/

replace gopack/rsautil => ../rsautil/

require gopack/xbyte v0.0.0

require gopack/tlv v0.0.0

require gopack/rsautil v0.0.0

require github.com/mattn/go-sqlite3 v1.14.18

require tcp v0.0.0-00010101000000-000000000000
