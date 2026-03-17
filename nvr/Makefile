include .env.client
export

BINARY  := nvr
CTL     := nvrctl
SERVICE := nvr
PI_HOST := $(NVR_PI_HOST)
PI_PATH := $(NVR_PI_PATH)

# Strip debug symbols (DWARF + symbol table) from Pi binaries to reduce size ~30%.
# Stack traces with file:line still work — Go's pclntab is preserved.
# To restore debug symbols, remove $(STRIP) from the build commands below.
STRIP := -ldflags='-s -w'

.PHONY: build deploy restart dev clean run setup-pi setup-mac

build:
	GOOS=linux GOARCH=arm64 go build $(STRIP) -o bin/$(BINARY) ./cmd/nvr
	GOOS=linux GOARCH=arm64 go build $(STRIP) -o bin/.$(CTL) ./cmd/nvrctl
	go build -o bin/$(CTL) ./cmd/nvrctl  # Mac native: keep debug symbols

deploy: build
	rsync -z bin/$(BINARY) bin/.$(CTL) .env.host $(PI_HOST):$(PI_PATH)/
	ssh $(PI_HOST) 'mv $(PI_PATH)/.$(CTL) $(PI_PATH)/$(CTL)'

## Fast deploy for dev loop — builds ARM64 binaries, rsync delta, restart
deploy-fast:
	GOOS=linux GOARCH=arm64 go build $(STRIP) -o bin/$(BINARY) ./cmd/nvr
	GOOS=linux GOARCH=arm64 go build $(STRIP) -o bin/.$(CTL) ./cmd/nvrctl
	rsync -z bin/$(BINARY) bin/.$(CTL) .env.host $(PI_HOST):$(PI_PATH)/
	ssh $(PI_HOST) 'mv $(PI_PATH)/.$(CTL) $(PI_PATH)/$(CTL) && sudo systemctl restart $(SERVICE)'

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
	@which fswatch > /dev/null 2>&1 || { echo "error: fswatch not found — run 'make setup-mac' first"; exit 1; }
	@while true; do \
		fswatch -1 -r -e '.*' -i '\.go$$' cmd/ internal/; \
		echo "\n==> Change detected, rebuilding..."; \
		$(MAKE) -s deploy-fast && echo "==> Deployed" || echo "==> Build failed"; \
	done

clean:
	rm -rf bin/

run:
	go run ./cmd/nvr

## Install dev dependencies on macOS
setup-mac:
	@echo "Installing dev dependencies..."
	@which fswatch > /dev/null 2>&1 || brew install fswatch
	@echo "Done."

setup-pi:
	ssh $(PI_HOST) 'sudo mkdir -p $(PI_PATH) && sudo chown admin:admin $(PI_PATH)'
	scp .env.host $(PI_HOST):$(PI_PATH)/.env.host
	scp deploy/nvr.service $(PI_HOST):/tmp/nvr.service
	ssh $(PI_HOST) 'sudo mv /tmp/nvr.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable $(SERVICE)'

## Install go2rtc on the Pi (run once, or after a go2rtc upgrade)
setup-go2rtc:
	rsync deploy/go2rtc.service deploy/setup-go2rtc.sh $(PI_HOST):$(PI_PATH)/
	ssh $(PI_HOST) 'chmod +x $(PI_PATH)/setup-go2rtc.sh && sudo $(PI_PATH)/setup-go2rtc.sh'
