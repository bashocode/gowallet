module github.com/bashocode/gowallet/microservices/notification-service

go 1.26.4

replace github.com/bashocode/gowallet/microservices/shared => ../shared

require (
	github.com/bashocode/gowallet/microservices/shared v0.0.0-20260713161533-63ca5e8f9fa2
	github.com/bashocode/gowallet/microservices/user-service v0.0.0-20260713161533-63ca5e8f9fa2
	github.com/rabbitmq/amqp091-go v1.12.0
	google.golang.org/grpc v1.82.0
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-sql-driver/mysql v1.10.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/redis/go-redis/v9 v9.21.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.mongodb.org/mongo-driver v1.17.9 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
