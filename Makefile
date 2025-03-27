proto-merch:
	protoc -I api/proto \
	       -I api/googleapis \
	       -I$(shell go list -m -f '{{ .Dir }}' github.com/grpc-ecosystem/grpc-gateway/v2) \
	       --go_out=. \
	       --go-grpc_out=. \
	       --grpc-gateway_out=. \
	       --openapiv2_out=./docs \
	       api/proto/merch_service.proto

DB_DSN=postgres://champ001:123champ123@localhost:5432/merch-store-grpc?sslmode=disable

.PHONY: goose-up goose-down goose-create goose-status goose-reset

# Применить миграции
goose-up:
	goose -dir ./migrations postgres "$(DB_DSN)" up

# Откатить последнюю миграцию
goose-down:
	goose -dir ./migrations postgres "$(DB_DSN)" down

# Посмотреть статус миграций
goose-status:
	goose -dir ./migrations postgres "$(DB_DSN)" status

# Создать новую миграцию
goose-create:
	goose -dir ./migrations create $(name) sql

# Откатить все миграции
goose-reset:
	goose -dir ./migrations postgres "$(DB_DSN)" reset

k6-run:
	./k6 run scripts/k6/load_tests.js