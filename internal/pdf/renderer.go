package pdf

import "context"

type Renderer interface {
	Render(context.Context, Request) ([]byte, error)
	Ready(context.Context) error
}
