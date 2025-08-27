package ports

import (
    "fmt"
    "net"
)

func FindFreePort() (int, error) {
    l, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return 0, fmt.Errorf("listen: %w", err)
    }
    defer l.Close()
    return l.Addr().(*net.TCPAddr).Port, nil
}
