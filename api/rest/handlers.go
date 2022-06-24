package rest

import (
	"net/http"

	"github.com/egsam98/portfolio/amqp"
	"github.com/egsam98/portfolio/domain/gateways"
	"github.com/egsam98/portfolio/domain/portfolio"
	"github.com/egsam98/portfolio/pg"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"gitlab.com/moderntoken/gateways/core"
)

const (
	requestBodyCtxKey  = "request_body"
	responseBodyCtxKey = "response_body"
)

// InitHandler setups REST API routes
func InitHandler(secret []byte, pm *portfolio.Manager) http.Handler {
	r := echo.New()
	r.Binder = &binder{}
	r.HTTPErrorHandler = httpErrorHandler(r)
	r.Use(
		recovery(),
		middleware.RequestID(),
		requestLogger(),
		bodyDump(),
	)

	priv := r.Group("", middleware.JWT(secret))
	ctrl := newPortfoliosController(pm)
	priv.GET("/portfolios/:name/data", ctrl.getData)
	priv.POST("/portfolios/:name/triggers", ctrl.addTriggers)

	// API docs
	r.GET("/swagger/*", echoSwagger.WrapHandler)
	r.GET("/docs", func(ctx echo.Context) error {
		return ctx.Redirect(301, "/swagger/index.html")
	})

	return r
}

// InitHealthHandler setups health check /ready and /live http routes on returned http.Handler
func InitHealthHandler(
	version string,
	db *pg.DB,
	rabbit *amqp.Connection,
	rabbitPool *amqp.ChannelPool,
	gwsMngr gateways.Manager,
) http.Handler {
	r := echo.New()
	r.Use(recovery())

	r.GET("/ready", ready(version, db, rabbit, rabbitPool, gwsMngr))
	r.GET("/live", live)
	return r
}

type RabbitMQStatus struct {
	Status      string `json:"status"`
	ChannelPool struct {
		AcquiredResources int32 `json:"acquired_resources"`
		TotalResources    int32 `json:"total_resources"`
		MaxResources      int32 `json:"max_resources"`
	} `json:"channel_pool"`
}

func ready(
	version string,
	db *pg.DB,
	rabbit *amqp.Connection,
	rabbitPool *amqp.ChannelPool,
	gwsMngr gateways.Manager,
) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		var err error
		res := struct {
			Version       string                        `json:"version"`
			DB            string                        `json:"db"`
			RabbitMQ      RabbitMQStatus                `json:"rabbit_mq"`
			GatewayStatus map[string]core.GatewayStatus `json:"gateway_status"`
		}{
			Version: version,
			DB:      "ok",
			RabbitMQ: RabbitMQStatus{
				Status: "ok",
			},
			GatewayStatus: make(map[string]core.GatewayStatus),
		}

		// Check DB
		if err = db.Ping(ctx.Request().Context()); err != nil {
			res.DB = err.Error()
		}

		// Check RabbitMQ
		if channel, err := rabbit.Channel(); err != nil {
			res.RabbitMQ.Status = err.Error()
		} else if err := channel.Publish("", "healthcheck"); err != nil {
			res.RabbitMQ.Status = err.Error()
		}
		stat := rabbitPool.Stat()
		res.RabbitMQ.ChannelPool.AcquiredResources = stat.AcquiredResources()
		res.RabbitMQ.ChannelPool.TotalResources = stat.TotalResources()
		res.RabbitMQ.ChannelPool.MaxResources = stat.MaxResources()

		// Check gateways
		res.GatewayStatus = gwsMngr.Status()

		if err != nil {
			return ctx.NoContent(503)
		}
		return ctx.JSON(200, res)
	}
}

func live(echo.Context) error { return nil }
