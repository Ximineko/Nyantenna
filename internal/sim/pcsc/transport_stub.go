//go:build !pcsc

package pcsc

// 默认构建不链接 libpcsclite，保持 CGO_ENABLED=0 可用。
func newContext() (Context, error) { return nil, ErrPCSCUnavailable }
