#!/bin/bash
set -e

BINARY_NAME="aws-tui"
VERSION="0.1.0"
BUILD_DIR="./build"
MAIN_PATH="./cmd/aws-tui"

case "${1:-build}" in
    deps)
        echo "Installing dependencies..."
        go mod tidy
        go mod download
        echo "Done!"
        ;;
    build)
        echo "Building $BINARY_NAME..."
        mkdir -p "$BUILD_DIR"
        go build -ldflags "-X main.version=$VERSION" -o "$BUILD_DIR/$BINARY_NAME" "$MAIN_PATH"
        echo "Built $BUILD_DIR/$BINARY_NAME"
        ;;
    run)
        $0 build
        "$BUILD_DIR/$BINARY_NAME" "${@:2}"
        ;;
    clean)
        echo "Cleaning..."
        rm -rf "$BUILD_DIR"
        go clean
        echo "Done!"
        ;;
    install)
        echo "Installing to GOPATH/bin..."
        go install -ldflags "-X main.version=$VERSION" "$MAIN_PATH"
        echo "Done!"
        ;;
    setup-config)
        mkdir -p ~/.aws-tui
        if [ ! -f ~/.aws-tui/config.yaml ]; then
            cp config.example.yaml ~/.aws-tui/config.yaml
            echo "Created ~/.aws-tui/config.yaml"
        else
            echo "Config already exists at ~/.aws-tui/config.yaml"
        fi
        ;;
    help|*)
        echo "AWS TUI - Terminal User Interface for AWS"
        echo ""
        echo "Usage: ./build.sh [command]"
        echo ""
        echo "Commands:"
        echo "  deps          Install dependencies"
        echo "  build         Build the application (default)"
        echo "  run [args]    Build and run with optional arguments"
        echo "  clean         Clean build artifacts"
        echo "  install       Install to GOPATH/bin"
        echo "  setup-config  Create config directory"
        echo "  help          Show this help"
        echo ""
        echo "Examples:"
        echo "  ./build.sh build"
        echo "  ./build.sh run --profile production"
        ;;
esac
