// +build go1.7

package crimsonred

import (
	"net/http"
)

func setRequestContext(r *http.Request, ctx Context) *http.Request {
	return r.WithContext(ctx)
}
