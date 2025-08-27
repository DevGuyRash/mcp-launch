package httpx

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "time"
)

var (
    DefaultTimeout = 20 * time.Second
)

func GetJSON(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, err
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return nil, fmt.Errorf("GET %s: %s (%d)", url, string(b), resp.StatusCode)
    }
    all, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    return all, nil
}

func WaitHTTPUp(url string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for {
        if time.Now().After(deadline) {
            return fmt.Errorf("timeout waiting for %s", url)
        }
        resp, err := http.Get(url) // #nosec G107
        if err == nil && resp.StatusCode < 500 {
            resp.Body.Close()
            return nil
        }
        if resp != nil && resp.Body != nil {
            resp.Body.Close()
        }
        time.Sleep(300 * time.Millisecond)
    }
}
