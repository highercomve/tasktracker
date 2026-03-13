#!/bin/bash
if [ "$1" = "cc" ] || [ "$1" = "c++" ]; then
    MODE="$1"
    shift
    TARGET=""
    ARGS=()
    SKIP_NEXT=0
    for arg in "$@"; do
        if [ "$SKIP_NEXT" = "1" ]; then
            SKIP_NEXT=0
            case "$arg" in
                x86_64-linux-gnu*|aarch64-linux-gnu*|arm-linux-gnu*)
                    TARGET="$arg" ;;
                /usr/include)
                    ;; # skip -isystem /usr/include (Zig-specific)
                *)
                    ARGS+=("$arg") ;;
            esac
            continue
        fi
        case "$arg" in
            -target|-isystem)
                SKIP_NEXT=1 ;;
            *)
                ARGS+=("$arg") ;;
        esac
    done

    # Detect host architecture to decide native vs cross compiler
    HOST_ARCH=$(dpkg --print-architecture)

    if [ "$MODE" = "cc" ]; then
        case "$TARGET" in
            x86_64-linux-gnu*)
                if [ "$HOST_ARCH" = "amd64" ]; then
                    exec gcc "${ARGS[@]}"
                else
                    exec x86_64-linux-gnu-gcc "${ARGS[@]}"
                fi ;;
            aarch64-linux-gnu*)
                if [ "$HOST_ARCH" = "arm64" ]; then
                    exec gcc "${ARGS[@]}"
                else
                    exec aarch64-linux-gnu-gcc "${ARGS[@]}"
                fi ;;
            *) exec gcc "${ARGS[@]}" ;;
        esac
    else
        case "$TARGET" in
            x86_64-linux-gnu*)
                if [ "$HOST_ARCH" = "amd64" ]; then
                    exec g++ "${ARGS[@]}"
                else
                    exec x86_64-linux-gnu-g++ "${ARGS[@]}"
                fi ;;
            aarch64-linux-gnu*)
                if [ "$HOST_ARCH" = "arm64" ]; then
                    exec g++ "${ARGS[@]}"
                else
                    exec aarch64-linux-gnu-g++ "${ARGS[@]}"
                fi ;;
            *) exec g++ "${ARGS[@]}" ;;
        esac
    fi
else
    exec /usr/local/zig/zig.real "$@"
fi
