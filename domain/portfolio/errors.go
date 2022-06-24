package portfolio

import (
	"errors"

	"github.com/egsam98/portfolio/domain"
)

const (
	ErrAccountNotFound = domain.Error("account isn't found")
	ErrNotFound        = domain.Error("portfolio isn't found")
	ErrExist           = domain.Error("portfolio already exists")
	ErrGateway         = domain.Error("gateway error")
)

var ErrGatewayNotFound = errors.New("gateway isn't found")
