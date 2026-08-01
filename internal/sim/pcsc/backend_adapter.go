package pcsc

import "github.com/ximineko/nyantenna/internal/backend"

// backend.PCSCCard 要求 OpenUSIMFiles 返回接口类型，而 Card.OpenUSIM 返回具体的
// *USIMSession；Go 没有协变返回，所以这里用一层薄封装桥接。
//
// 方向也只能是这样：internal/sim 依赖 internal/backend，pcsc 又依赖 internal/sim，
// 因此 backend 不能反过来引用 pcsc，接口只能定义在 backend 侧。
type backendCard struct {
	*Card
}

var (
	_ backend.PCSCCard      = backendCard{}
	_ backend.PCSCUSIMFiles = (*USIMSession)(nil)
)

// AsBackendCard 把 Card 适配为 backend.PCSCCard，供设备层构造 PC/SC 后端。
func AsBackendCard(c *Card) backend.PCSCCard {
	if c == nil {
		return nil
	}
	return backendCard{Card: c}
}

// OpenUSIMFiles 开一条 ADF_USIM 会话，返回值收窄为接口。
func (b backendCard) OpenUSIMFiles() (backend.PCSCUSIMFiles, error) {
	return b.Card.OpenUSIM()
}
