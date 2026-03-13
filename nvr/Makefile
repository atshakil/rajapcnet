include .env.client
export

BINARY  := nvr
CTL     := nvrctl
SERVICE := nvr
PI_HOST := $(NVR_PI_HOST)
PI_PATH := $(NVR_PI_PATH)

.PHONY: build deploy restart dev clean run setup-pi

build:
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY) ./cmd/nvr
	GOOS=linux GOARCH=arm64 go build -o bin/.$(CTL) ./cmd/nvrctl
	go build -o bin/$(CTL) ./cmd/nvrctl

deploy: build
	scp -q bin/$(BINARY) $(PI_HOST):$(PI_PATH)/$(BINARY).new
	scp -q bin/.$(CTL) $(PI_HOST):$(PI_PATH)/$(CTL).new
	ssh $(PI_HOST) 'mv $(PI_PATH)/$(BINARY).new $(PI_PATH)/$(BINARY) && mv $(PI_PATH)/$(CTL).new $(PI_PATH)/$(CTL)'

restart:
	ssh $(PI_HOST) 'sudo systemctl restart $(SERVICE)'

stop:
	ssh $(PI_HOST) 'sudo systemctl stop $(SERVICE)'

enable:
	ssh $(PI_HOST) 'sudo systemctl enable --now $(SERVICE)'

disable:
	ssh $(PI_HOST) 'sudo systemctl disable --now $(SERVICE)'

## Single dev command: build, deploy, restart, then watch for changes
dev: deploy enable
	@echo "\n==> Deployed and running. Watching for changes... (Ctrl+C to stop)"
	@fswatch -1 -r -e '.*' -i '\.go$$' cmd/ internal/ | while read -r _; do \
		echo "\n==> Change detected, rebuilding..."; \
		$(MAKE) -s deploy restart && echo "==> Deployed" || echo "==> Build failed"; \
		fswatch -1 -r -e '.*' -i '\.go$$' cmd/ internal/ > /dev/null; \
	done

clean:
	rm -rf bin/

run:
	go run ./cmd/nvr

setup-pi:
	ssh $(PI_HOST) 'sudo mkdir -p $(PI_PATH) && sudo chown admin:admin $(PI_PATH)'
	scp .env.host $(PI_HOST):$(PI_PATH)/.env.host
	scp deploy/nvr.service $(PI_HOST):/tmp/nvr.service
	ssh $(PI_HOST) 'sudo mv /tmp/nvr.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable $(SERVICE)'
