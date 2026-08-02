//go:build !pcsc

package pcsc

// 默认构建不链接 libpcsclite，保持 CGO_ENABLED=0 可用。
func newContext() (Context, error) { return nil, ErrPCSCUnavailable }

// 无 scard 依赖时没有可识别的句柄失效错误码。
func isScardStaleHandle(error) bool { return false }
