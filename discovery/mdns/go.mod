module github.com/xraph/farp/discovery/mdns

go 1.25.0

require (
	github.com/grandcat/zeroconf v1.0.0
	github.com/xraph/farp v1.1.0
	github.com/xraph/farp/discovery v0.0.0
)

require (
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/miekg/dns v1.1.27 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/net v0.54.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
)

replace (
	github.com/xraph/farp => ../../
	github.com/xraph/farp/discovery => ../
)
