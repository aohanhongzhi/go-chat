package im

type ICallback interface {
	Open(client IClient)
	Message(client IClient, msg []byte)
	Close(client IClient, code int, text string)
	Destroy(client IClient)
}

type (
	OpenCallback         func(client IClient)
	MessageCallback      func(client IClient, msg []byte)
	CloseCallback        func(client IClient, code int, text string)
	DestroyCallback      func(client IClient)
	ClientCallbackOption func(callBack *ClientCallback)
)

type ClientCallback struct {
	open    OpenCallback
	message MessageCallback
	close   CloseCallback
	destroy DestroyCallback
}

func NewClientCallback(opts ...ClientCallbackOption) ICallback {

	o := &ClientCallback{}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (c *ClientCallback) Open(client IClient) {
	if c.open != nil {
		c.open(client)
	}
}

func (c *ClientCallback) Message(client IClient, message []byte) {
	if c.message != nil {
		c.message(client, message)
	}
}

func (c *ClientCallback) Close(client IClient, code int, text string) {
	if c.close != nil {
		c.close(client, code, text)
	}
}

func (c *ClientCallback) Destroy(client IClient) {
	if c.destroy != nil {
		c.destroy(client)
	}
}

// WithOpenCallback 绑定连接成功回调事件
func WithOpenCallback(call OpenCallback) ClientCallbackOption {
	return func(callBack *ClientCallback) {
		callBack.open = call
	}
}

// WithMessageCallback 绑定消息回调事件
func WithMessageCallback(call MessageCallback) ClientCallbackOption {
	return func(callBack *ClientCallback) {
		callBack.message = call
	}
}

// WithCloseCallback 绑定连接关闭回调事件
func WithCloseCallback(call CloseCallback) ClientCallbackOption {
	return func(callBack *ClientCallback) {
		callBack.close = call
	}
}

// WithDestroyCallback 连接销毁回调事件
// TODO 待实现
func WithDestroyCallback(call DestroyCallback) ClientCallbackOption {
	return func(callBack *ClientCallback) {
		callBack.destroy = call
	}
}
