package rest

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

const DumpResponseBodyMaxSize = 1000

func recovery() echo.MiddlewareFunc {
	return middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			var trimmedStack []string
			for _, line := range strings.Split(string(stack), "\n") {
				if line != "" {
					trimmedStack = append(trimmedStack, strings.Trim(line, "\t\n"))
				}
			}
			log.Error().Stack().Err(err).Strs("stack", trimmedStack).Msg("Panic recovered")
			return nil
		},
	})
}

func requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper:        nil,
		BeforeNextFunc: nil,
		LogValuesFunc: func(ctx echo.Context, v middleware.RequestLoggerValues) (err error) {
			var reqBody, respBody []byte
			reqBody, _ = ctx.Get(requestBodyCtxKey).([]byte)
			respBody, _ = ctx.Get(responseBodyCtxKey).([]byte)

			e := log.Debug().
				Str("method", v.Method).
				Str("uri", v.URI).
				Str("request_id", v.RequestID).
				Str("remote_ip", v.RemoteIP).
				Bytes("request_body", reqBody).
				Bytes("response_body", respBody).
				Time("start_time", v.StartTime.UTC()).
				Dur("latency", v.Latency).
				Str("protocol", v.Protocol).
				Int("status", v.Status)
			if v.Error != nil {
				e = e.Stack().Err(v.Error)
			}
			e.Msg("Request")

			ctx.Set(requestBodyCtxKey, nil)
			ctx.Set(responseBodyCtxKey, nil)
			return nil
		},
		LogLatency:   true,
		LogProtocol:  true,
		LogRemoteIP:  true,
		LogMethod:    true,
		LogURI:       true,
		LogRequestID: true,
		LogStatus:    true,
		LogError:     true,
	})
}

func bodyDump() echo.MiddlewareFunc {
	return middleware.BodyDump(func(ctx echo.Context, reqBody []byte, resBody []byte) {
		if len(resBody) > DumpResponseBodyMaxSize {
			resBody = append(resBody[:DumpResponseBodyMaxSize], '.', '.', '.')
		}
		ctx.Set(requestBodyCtxKey, reqBody)
		ctx.Set(responseBodyCtxKey, resBody)
	})
}
