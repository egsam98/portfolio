package rest

import (
	"github.com/egsam98/portfolio/domain"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

// binder wraps echo.DefaultBinder adding possibility to validate types implementing validatable after (*echo.Binder).Bind()
type binder struct {
	echo.DefaultBinder
}

type validatable interface {
	Validate() error
}

func (b *binder) Bind(i interface{}, ctx echo.Context) error {
	if err := b.DefaultBinder.Bind(i, ctx); err != nil {
		return err
	}
	if v, ok := i.(validatable); ok {
		if err := v.Validate(); err != nil {
			return echo.NewHTTPError(400, err.Error())
		}
	}
	return nil
}

func httpErrorHandler(e *echo.Echo) echo.HTTPErrorHandler {
	return func(err error, ctx echo.Context) {
		if errors.As(err, new(domain.Error)) {
			err = echo.NewHTTPError(400, err.Error())
		}
		e.DefaultHTTPErrorHandler(err, ctx)
	}
}
