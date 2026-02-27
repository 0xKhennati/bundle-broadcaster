BINARY := bundle-broadcaster
INSTALL_DIR := /opt/bundle-broadcaster
GO := go
GOFLAGS := -v

.PHONY: build install clean uninstall

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) .

install: build
	@echo "Installing to $(INSTALL_DIR) (requires sudo)"
	sudo mkdir -p $(INSTALL_DIR)
	sudo cp -f $(BINARY) $(INSTALL_DIR)/
	sudo cp config.json $(INSTALL_DIR)/config.json
	@if [ ! -f $(INSTALL_DIR)/config.json ]; then \
		sudo cp config.json $(INSTALL_DIR)/config.json; \
		echo "Created config.json from example. Edit $(INSTALL_DIR)/config.json with your settings."; \
	else \
		printf '\033[33m%s\033[0m\n' 'config.json exists, leaving unchanged.'; \
	fi
	sudo cp bundle-broadcaster.service $(INSTALL_DIR)/
	sudo ln -sf $(INSTALL_DIR)/bundle-broadcaster.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "Install complete. Edit $(INSTALL_DIR)/config.json then run: sudo systemctl enable --now bundle-broadcaster"

clean:
	rm -f $(BINARY)

uninstall:
	sudo systemctl stop bundle-broadcaster 2>/dev/null || true
	sudo systemctl disable bundle-broadcaster 2>/dev/null || true
	sudo rm -f /etc/systemd/system/bundle-broadcaster.service
	sudo systemctl daemon-reload
	sudo rm -rf $(INSTALL_DIR)
	@echo "Uninstall complete."
