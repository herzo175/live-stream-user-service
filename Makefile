NAME=live-stream-user-service
VERSION=$(shell git rev-parse HEAD)
HOST_PORT=3001

build:
	docker build -t $(NAME)/$(VERSION) ./

run:
	docker run --rm -it \
		--env-file .env \
		-p $(HOST_PORT):80 \
		$(NAME)/$(VERSION)

deploy:
	# TODO: CI to handle build/deploy steps
	heroku container:push web
	heroku container:release web

clean:
	docker stop $(NAME):$(VERSION)