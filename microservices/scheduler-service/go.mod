module github.com/bashocode/gowallet/microservices/scheduler-service

go 1.26.4

replace github.com/bashocode/gowallet/microservices/auth-service => ../auth-service

replace github.com/bashocode/gowallet/microservices/shared => ../shared

replace github.com/bashocode/gowallet/microservices/payment-service => ../payment-service

replace github.com/bashocode/gowallet/microservices/transaction-service => ../transaction-service

replace github.com/bashocode/gowallet/microservices/user-service => ../user-service

replace github.com/bashocode/gowallet/microservices/wallet-service => ../wallet-service

require (
	github.com/bashocode/gowallet/microservices/auth-service v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/payment-service v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/shared v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/transaction-service v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/user-service v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/wallet-service v0.0.0-00010101000000-000000000000
	github.com/robfig/cron/v3 v3.0.1
	google.golang.org/grpc v1.82.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/minio-go/v7 v7.2.1 // indirect
	github.com/pelletier/go-toml/v2 v2.3.1 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spf13/viper v1.21.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tinylib/msgp v1.6.1 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/ini.v1 v1.67.2 // indirect
)
