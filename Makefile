NAME=live-stream-user-service
VERSION=$(shell git rev-parse HEAD)
DEFAULT_PORT=3001

build:
	docker build --build-arg PORT_ARG=$(DEFAULT_PORT) -t $(NAME)/$(VERSION) ./

run:
	docker run --rm -it \
		--env-file .env \
		--network host \
		-p $(DEFAULT_PORT):$(DEFAULT_PORT) \
		$(NAME)/$(VERSION)

deploy:
	# TODO: CI to handle build/deploy steps
	heroku container:push web
	heroku container:release web

clean:
	docker stop $(NAME):$(VERSION)