package breaker

import "github.com/shunvel/mcp-breaker/pkg/proxy"

// Chain composes multiple interceptors; first block wins on client path.
type Chain struct {
	interceptors []proxy.Interceptor
}

// NewChain creates an interceptor chain from the given interceptors.
func NewChain(interceptors ...proxy.Interceptor) *Chain {
	return &Chain{interceptors: interceptors}
}

// OnClientFrame implements proxy.Interceptor.
func (c *Chain) OnClientFrame(frame proxy.Frame) proxy.Decision {
	for _, i := range c.interceptors {
		if i == nil {
			continue
		}
		decision := i.OnClientFrame(frame)
		if !decision.Forward {
			return decision
		}
	}
	return proxy.Decision{Forward: true}
}

// OnServerFrame implements proxy.Interceptor.
func (c *Chain) OnServerFrame(frame proxy.Frame) proxy.Frame {
	out := frame
	for _, i := range c.interceptors {
		if i == nil {
			continue
		}
		out = i.OnServerFrame(out)
	}
	return out
}
