module github.com/xraph/farp/discovery/redis

go 1.23.0

require (
	github.com/redis/go-redis/v9 v9.7.0
	github.com/xraph/farp v1.1.0
	github.com/xraph/farp/discovery v1.3.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace (
	github.com/xraph/farp => ../../
	github.com/xraph/farp/discovery => ../
)
