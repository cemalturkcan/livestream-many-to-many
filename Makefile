# Simple Makefile for a Go project
build:
	@echo "Building..."
	@set CGO_ENABLED=0 && go build -ldflags "-s -w" -o main.exe cmd/main.go

# Run the application
run:
	@go run cmd/main.go

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main.exe

# Live Reload
watch:
	@powershell -ExecutionPolicy Bypass -Command "if (Get-Command air -ErrorAction SilentlyContinue) { \
		air; \
		Write-Output 'Watching...'; \
	} else { \
		Write-Output 'Installing air...'; \
		go install github.com/air-verse/air@latest; \
		air; \
		Write-Output 'Watching...'; \
	}"

