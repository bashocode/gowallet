module github.com/bashocode/gowallet/microservices/scheduler-service

go 1.26.4

replace github.com/bashocode/gowallet/microservices/auth-service => ../auth-service

replace github.com/bashocode/gowallet/microservices/shared => ../shared

replace github.com/bashocode/gowallet/microservices/transaction-service => ../transaction-service

replace github.com/bashocode/gowallet/microservices/wallet-service => ../wallet-service

require (
	github.com/bashocode/gowallet/microservices/auth-service v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/shared v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/transaction-service v0.0.0-00010101000000-000000000000
	github.com/bashocode/gowallet/microservices/wallet-service v0.0.0-00010101000000-000000000000
	github.com/robfig/cron/v3 v3.0.1
	google.golang.org/grpc v1.82.0
)

require (
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
