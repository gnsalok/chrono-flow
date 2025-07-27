.PHONY: all clean

all: build

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v